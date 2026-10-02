package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type correctnessReport struct {
	Test             string `json:"test"`
	Concurrency      int    `json:"concurrency"`
	Created          int    `json:"created"`
	Conflicted       int    `json:"conflicted"`
	Other            int    `json:"other"`
	ExpectedCreated  int    `json:"expected_created"`
	DBStateConfirmed int    `json:"db_state_confirmed"`
	Pass             bool   `json:"pass"`
}

const correctnessConcurrent = 128

// runCorrectness proves the invariants under concurrency in an isolated
// scope: a fresh class with capacity C is created, then C*3 concurrent
// bookings are fired at it — exactly C must succeed (201) and every other
// attempt must get a capacity conflict (409). Separately, one fresh member is
// checked in concurrently five times (exactly one 201, rest 409) and a fresh
// invoice is paid concurrently five times (exactly one 2xx, rest 409).
func runCorrectness(cfg config) error {
	c := newClient(cfg.base)
	if err := c.login(cfg.admin, cfg.password); err != nil {
		return err
	}

	// 1. Capacity exactly-N under the FOR UPDATE guard.
	branchID := firstOf(seedBranches, c, "/branches?limit=100").ids[0]
	pkg := firstOf(seedPackages, c, "/packages?limit=100").ids[0]
	classBody := map[string]any{
		"branchId": branchID,
		"name":     fmt.Sprintf("Contention-%d", time.Now().UnixNano()),
		"startsAt": time.Now().UTC().Add(time.Hour).Truncate(time.Minute).Format(time.RFC3339),
		"endsAt":   time.Now().UTC().Add(2 * time.Hour).Truncate(time.Minute).Format(time.RFC3339),
		"capacity": cfg.capacity,
	}
	raw, err := json.Marshal(classBody)
	if err != nil {
		return err
	}
	body, status, err := c.do("POST", "/classes", raw)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("create contention class: HTTP %d: %s", status, truncate(string(body), 200))
	}
	classID, err := idOf(body, "contention class")
	if err != nil {
		return err
	}

	// Create members on the same branch lazily inside the hammer.
	bookers := cfg.capacity * 3
	var (
		created  atomic.Int64
		conflict atomic.Int64
		other    atomic.Int64
		wg       sync.WaitGroup
		sem      = make(chan struct{}, correctnessConcurrent)
	)
	wg.Add(bookers)
	for i := 0; i < bookers; i++ {
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			defer wg.Done()
			mb, err := c.postJSON("/members", map[string]any{
				"branchId": branchID,
				"name":     fmt.Sprintf("Booker-%d-%d", i, time.Now().UnixNano()),
				"email":    fmt.Sprintf("booker-%d-%d@load.local", i, time.Now().UnixNano()),
			})
			if err != nil {
				other.Add(1)
				return
			}
			memberID, err := idOf(mb, "booker member")
			if err != nil {
				other.Add(1)
				return
			}
			bBody := []byte(fmt.Sprintf(`{"classId":%q,"memberId":%q}`, classID, memberID))
			_, st, err := c.do("POST", "/bookings", bBody)
			if err != nil {
				other.Add(1)
				return
			}
			switch {
			case st == 201:
				created.Add(1)
			case st == 409:
				conflict.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()

	seen, err := countClassBookings(c, classID)
	if err != nil {
		return err
	}
	rep := &correctnessReport{
		Test:             "booking-capacity-exactly-N",
		Concurrency:      bookers,
		Created:          int(created.Load()),
		Conflicted:       int(conflict.Load()),
		Other:            int(other.Load()),
		ExpectedCreated:  cfg.capacity,
		DBStateConfirmed: seen,
		Pass:             int(created.Load()) == cfg.capacity && conflict.Load() == int64(bookers-cfg.capacity) && other.Load() == 0 && seen == cfg.capacity,
	}
	fmt.Printf("  correctness booking-capacity: created=%d (want %d) conflict=%d other=%d db_count=%d  => ",
		rep.Created, rep.ExpectedCreated, rep.Conflicted, rep.Other, rep.DBStateConfirmed)
	fmt.Println(passWord(rep.Pass))

	rep2, err := checkInRace(c, branchID, pkg)
	if err != nil {
		return err
	}
	fmt.Printf("  correctness check-in-race:    created=%d conflicts=%d other=%d => %s\n",
		rep2.Created, rep2.Conflicted, rep2.Other, passWord(rep2.Pass))

	// 3. Invoice pay race: exactly one transition, the rest conflict.
	rep3, err := payRace(c)
	if err != nil {
		return err
	}
	fmt.Printf("  correctness invoice-pay-race: created=%d conflicts=%d other=%d => %s\n",
		rep3.Created, rep3.Conflicted, rep3.Other, passWord(rep3.Pass))

	reports := []*correctnessReport{rep, rep2, rep3}
	if err := writeCorrectness(cfg, reports); err != nil {
		return err
	}
	if !rep.Pass || !rep2.Pass || !rep3.Pass {
		return fmt.Errorf("correctness: one or more invariants FAILED")
	}
	return nil
}

func writeCorrectness(cfg config, reps []*correctnessReport) error {
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(reps, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.out, "correctness-"+time.Now().Format("20060102-150405")+".json")
	return os.WriteFile(path, raw, 0o644)
}

func passWord(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// countClassBookings walks the paginated booking list for a class.
func countClassBookings(c *client, classID string) (int, error) {
	total := 0
	cursor := ""
	for {
		path := fmt.Sprintf("/classes/%s/bookings?limit=100", classID)
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		body, status, err := c.do("GET", path, nil)
		if err != nil {
			return 0, err
		}
		if status != 200 {
			return 0, fmt.Errorf("list class bookings: HTTP %d: %s", status, truncate(string(body), 200))
		}
		var page struct {
			Items      []json.RawMessage `json:"items"`
			NextCursor *string           `json:"nextCursor"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return 0, err
		}
		total += len(page.Items)
		if page.NextCursor == nil || *page.NextCursor == "" {
			return total, nil
		}
		cursor = *page.NextCursor
	}
}

// checkInRace hammers one fresh member with five concurrent check-ins and the
// ticket must resolve to exactly one open visit.
func checkInRace(c *client, branchID, pkgID string) (*correctnessReport, error) {
	mb, err := c.postJSON("/members", map[string]any{
		"branchId": branchID,
		"name":     fmt.Sprintf("CheckInRace-%d", time.Now().UnixNano()),
		"email":    fmt.Sprintf("race-%d@load.local", time.Now().UnixNano()),
	})
	if err != nil {
		return nil, err
	}
	memberID, err := idOf(mb, "race member")
	if err != nil {
		return nil, err
	}
	if _, err := c.postJSON("/memberships", map[string]any{
		"memberId": memberID, "packageId": pkgID, "branchId": branchID,
	}); err != nil {
		return nil, err
	}

	const attempts = 5
	var (
		created  atomic.Int64
		conflict atomic.Int64
		other    atomic.Int64
		wg       sync.WaitGroup
	)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := []byte(fmt.Sprintf(`{"memberId":%q,"branchId":%q}`, memberID, branchID))
			_, st, err := c.do("POST", "/attendance/check-in", body)
			if err != nil {
				other.Add(1)
				return
			}
			switch {
			case st == 201:
				created.Add(1)
			case st == 409:
				conflict.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()
	rep := &correctnessReport{Test: "check-in-single-open", Concurrency: attempts,
		Created: int(created.Load()), Conflicted: int(conflict.Load()), Other: int(other.Load()), ExpectedCreated: 1}
	// Confirm via the member's attendance list that exactly one open visit
	// exists (a record with checkedOutAt == null).
	open := 0
	for _, a := range attendanceOf(c, memberID) {
		var rec struct {
			CheckedOutAt *time.Time `json:"checkedOutAt"`
		}
		_ = json.Unmarshal(a, &rec)
		if rec.CheckedOutAt == nil {
			open++
		}
	}
	rep.DBStateConfirmed = open
	rep.Pass = created.Load() == 1 && conflict.Load() == int64(attempts-1) && other.Load() == 0 && open == 1
	return rep, nil
}

func attendanceOf(c *client, memberID string) []json.RawMessage {
	body, status, err := c.do("GET", "/members/"+memberID+"/attendance?limit=100", nil)
	if err != nil || status != 200 {
		return nil
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	if json.Unmarshal(body, &page) != nil {
		return nil
	}
	return page.Items
}

// payRace fires five concurrent paid transitions at one fresh invoice.
func payRace(c *client) (*correctnessReport, error) {
	branchID := firstOf(1, c, "/branches?limit=100").ids[0]
	pkgID := firstOf(1, c, "/packages?limit=100").ids[0]
	mb, err := c.postJSON("/members", map[string]any{
		"branchId": branchID,
		"name":     fmt.Sprintf("PayRace-%d", time.Now().UnixNano()),
		"email":    fmt.Sprintf("payrace-%d@load.local", time.Now().UnixNano()),
	})
	if err != nil {
		return nil, err
	}
	memberID, err := idOf(mb, "pay race member")
	if err != nil {
		return nil, err
	}
	ms, err := c.postJSON("/memberships", map[string]any{
		"memberId": memberID, "packageId": pkgID, "branchId": branchID,
	})
	if err != nil {
		return nil, err
	}
	membershipID, err := idOf(ms, "pay race membership")
	if err != nil {
		return nil, err
	}
	inv, err := c.postJSON("/invoices", map[string]any{
		"memberId": memberID, "membershipId": membershipID,
		"amountCents": 25000, "currency": "BHD",
		"dueAt": time.Now().UTC().AddDate(0, 1, 0).Format(time.RFC3339), "status": "pending",
	})
	if err != nil {
		return nil, err
	}
	invoiceID, err := idOf(inv, "pay race invoice")
	if err != nil {
		return nil, err
	}

	const attempts = 5
	var (
		created  atomic.Int64
		conflict atomic.Int64
		other    atomic.Int64
		wg       sync.WaitGroup
	)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := []byte(`{"status":"paid"}`)
			_, st, err := c.do("PATCH", "/invoices/"+invoiceID, body)
			if err != nil {
				other.Add(1)
				return
			}
			switch {
			case st >= 200 && st < 300:
				created.Add(1)
			case st == 409:
				conflict.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()
	rep := &correctnessReport{Test: "invoice-pay-single-transition", Concurrency: attempts,
		Created: int(created.Load()), Conflicted: int(conflict.Load()), Other: int(other.Load()), ExpectedCreated: 1}
	b, status, err := c.do("GET", "/invoices/"+invoiceID, nil)
	if err == nil && status == 200 {
		var invOut struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(b, &invOut) == nil && invOut.Status == "paid" {
			rep.DBStateConfirmed = 1
		}
	}
	rep.Pass = created.Load() == 1 && conflict.Load() == int64(attempts-1) && other.Load() == 0 && rep.DBStateConfirmed == 1
	return rep, nil
}

type idList struct{ ids []string }

// firstOf pulls up to n ids from a paginated list endpoint (used to pick
// branch/package ids inside correctness without the full fixture).
func firstOf(n int, c *client, path string) idList {
	body, status, err := c.do("GET", path, nil)
	if err != nil || status != 200 {
		return idList{}
	}
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if json.Unmarshal(body, &page) != nil {
		return idList{}
	}
	if len(page.Items) > n {
		page.Items = page.Items[:n]
	}
	out := idList{}
	for _, it := range page.Items {
		out.ids = append(out.ids, it.ID)
	}
	return out
}
