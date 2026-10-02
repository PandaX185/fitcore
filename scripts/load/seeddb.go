package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// seedDB bulk-loads the load fixture straight into the load postgres,
// bypassing the HTTP API. Seeding is bookkeeping (it sets up the state the
// load phases draw from), not something we are trying to measure, so it uses
// multi-row batched INSERTs inside a single transaction instead of ~100k web
// requests. The fixture.json written afterwards is byte-compatible with the
// API path: same shapes, same branch/member/class/membership layouts, so every
// load scenario is agnostic to how the fixture was created.
func seedDB(cfg config, dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("seed-db: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("seed-db: %w", err)
	}

	fx := &fixture{Contention: map[string]string{}, capacityMap: map[string]int{}}
	fx.Members = make([]string, 0, seedBranches*seedMembers)
	fx.Memberships = make([]string, 0, seedBranches*seedMembers)
	fx.Classes = make([]string, 0, seedBranches*seedClasses)
	fx.ClassBranch = make([]int, 0, seedBranches*seedClasses)

	// The load DB is disposable. Clear the fixture tables in FK-safe order so
	// re-seeding is idempotent instead of tripping the email/name unique
	// constraints. Deliberately NOT `TRUNCATE … CASCADE`: cascade would reach
	// `staff` (branches FK) and wipe the seeded load admin + its auth state.
	// DELETE keeps the admin row intact (its branch reference just becomes NULL).
	clearFixture := `
DELETE FROM class_bookings;
DELETE FROM attendance;
DELETE FROM invoices;
DELETE FROM memberships;
DELETE FROM classes;
DELETE FROM members;
DELETE FROM membership_packages;
DELETE FROM branches;`
	if _, err := db.Exec(clearFixture); err != nil {
		return fmt.Errorf("seed-db clear: %w", err)
	}

	now := time.Now().UTC()
	classStart := func(ci int) time.Time {
		return now.Add(time.Duration(ci) * time.Hour).Truncate(time.Minute)
	}

	var branchRows, pkgRows, memberRows, classRows, memRows [][]any

	// branches -----------------------------------------------------------------
	for i := 0; i < seedBranches; i++ {
		id := genUUID4()
		fx.Branches = append(fx.Branches, id)
		branchRows = append(branchRows, []any{
			id,
			fmt.Sprintf("LoadBr-%d", i),
			fmt.Sprintf("Load St %d", i),
			31.5 + float64(i)*0.01,
			-8.0 + float64(i)*0.01,
		})
	}

	// packages ---------------------------------------------------------------
	currencies := []string{"BHD", "AED", "SAR"}
	for i := 0; i < seedPackages; i++ {
		id := genUUID4()
		fx.Packages = append(fx.Packages, id)
		pkgRows = append(pkgRows, []any{
			id,
			fmt.Sprintf("LoadPkg-%d", i),
			"",
			30 * (i + 1),    // duration_days
			25000 * (i + 1), // price_cents
			true,
			currencies[i%len(currencies)],
		})
	}

	// members (flat [branchIdx*nMembers + offset], like the API path) --------
	for bi := 0; bi < seedBranches; bi++ {
		for mi := 0; mi < seedMembers; mi++ {
			id := genUUID4()
			fx.Members = append(fx.Members, id)
			memberRows = append(memberRows, []any{
				id,
				fx.Branches[bi],
				fmt.Sprintf("LoadMember-%d-%d", bi, mi),
				fmt.Sprintf("member-%d-%d@load.local", bi, mi),
			})
		}
	}

	// classes (ci==0 is the contention class, capacity = cfg.capacity) -------
	for bi := 0; bi < seedBranches; bi++ {
		for ci := 0; ci < seedClasses; ci++ {
			cap := 50
			if ci == 0 {
				cap = cfg.capacity
			}
			id := genUUID4()
			fx.Classes = append(fx.Classes, id)
			fx.ClassBranch = append(fx.ClassBranch, bi)
			fx.capacityMap[id] = cap
			if ci == 0 {
				fx.Contention[fx.Branches[bi]] = id
			}
			s := classStart(ci)
			classRows = append(classRows, []any{
				id,
				fx.Branches[bi],
				fmt.Sprintf("LoadClass-%d-%d", bi, ci),
				cap,
				s,
				s.Add(time.Hour),
			})
		}
	}

	// memberships (1:1 with members, status defaults to 'active') -------------
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for i := range fx.Members {
		id := genUUID4()
		fx.Memberships = append(fx.Memberships, id)
		pkg := i % len(fx.Packages)
		memRows = append(memRows, []any{
			id,
			fx.Members[i],
			fx.Packages[pkg],
			fx.Branches[i/seedMembers],
			today,
			today.Add(time.Duration(30*(pkg+1)) * 24 * time.Hour),
		})
	}

	// One transaction, batched multi-row statements ---------------------------
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("seed-db begin: %w", err)
	}
	defer tx.Rollback()

	if err := batchInsert(tx, "branches", []string{"id", "name", "address", "latitude", "longitude"}, branchRows); err != nil {
		return err
	}
	if err := batchInsert(tx, "membership_packages", []string{"id", "name", "description", "duration_days", "price_cents", "active", "currency"}, pkgRows); err != nil {
		return err
	}
	if err := batchInsert(tx, "members", []string{"id", "branch_id", "name", "email"}, memberRows); err != nil {
		return err
	}
	if err := batchInsert(tx, "classes", []string{"id", "branch_id", "name", "capacity", "starts_at", "ends_at"}, classRows); err != nil {
		return err
	}
	if err := batchInsert(tx, "memberships", []string{"id", "member_id", "package_id", "branch_id", "starts_on", "expires_on"}, memRows); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed-db commit: %w", err)
	}

	// Fresh relation statistics keep the planner's guesses sane for the loads.
	if _, err := db.Exec(`VACUUM ANALYZE members, memberships, classes, branches, membership_packages`); err != nil {
		return fmt.Errorf("seed-db vacuum: %w", err)
	}

	if err := saveFixture(cfg, fx); err != nil {
		return err
	}
	fmt.Printf("seeded (db) %d branches, %d members, %d packages, %d classes, %d memberships\n",
		len(fx.Branches), len(fx.Members), len(fx.Packages), len(fx.Classes), len(fx.Memberships))
	return nil
}

// batchInsert inserts rows into table with one multi-row INSERT per ~1000 rows,
// sharing the caller's transaction for a single commit.
func batchInsert(tx *sql.Tx, table string, cols []string, rows [][]any) error {
	const batch = 1000

	colList := strings.Join(cols, ", ")
	for len(rows) > 0 {
		n := batch
		if n > len(rows) {
			n = len(rows)
		}
		chunk := rows[:n]
		rows = rows[n:]

		var sb strings.Builder
		sb.WriteString("INSERT INTO ")
		sb.WriteString(table)
		sb.WriteString(" (")
		sb.WriteString(colList)
		sb.WriteString(") VALUES ")
		args := make([]any, 0, len(chunk)*len(cols))
		ph := 0
		for ri, r := range chunk {
			if ri > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(")
			for ci := range r {
				if ci > 0 {
					sb.WriteString(",")
				}
				ph++
				sb.WriteString("$")
				sb.WriteString(strconv.Itoa(ph))
			}
			sb.WriteString(")")
			args = append(args, r...)
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("seed-db %s multi-row insert: %w", table, err)
		}
	}
	return nil
}

// genUUID4 returns a random RFC 4122 version-4 UUID string without pulling a
// uuid dependency into the harness module.
func genUUID4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("genUUID4: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
