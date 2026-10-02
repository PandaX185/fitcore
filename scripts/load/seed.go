package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	seedBranches   = 100
	seedMembers    = 500
	seedPackages   = 4
	seedClasses    = 80
	seedConcurrent = 80
)

type fixture struct {
	Branches    []string
	Members     []string // flat, [branchIdx*nMembers + offset]
	Memberships []string // aligned 1:1 with Members
	Packages    []string
	Classes     []string
	ClassBranch []int // branch index per class, aligned 1:1 with Classes
	Contention  map[string]string
	capacityMap map[string]int
}

func (f *fixture) member(branch int, offset int) string {
	return f.Members[branch*seedMembers+offset]
}

func (f *fixture) capacity(classID string) int {
	if c, ok := f.capacityMap[classID]; ok {
		return c
	}
	return 50
}

// seed creates the deterministic fixture the load phases draw from. It is
// idempotent per fresh load DB only; run it once after `load-migrate`.
func seed(cfg config) error {
	if cfg.seedVia == "db" {
		if cfg.seedDB == "" {
			return fmt.Errorf("seed via db requires -seed-db (postgres DSN); run through `just load-seed`")
		}
		return seedDB(cfg, cfg.seedDB)
	}

	c := newClient(cfg.base)
	if err := c.login(cfg.admin, cfg.password); err != nil {
		return err
	}
	fx := &fixture{Contention: map[string]string{}, capacityMap: map[string]int{}}

	for i := 0; i < seedBranches; i++ {
		body, err := c.postJSON("/branches", map[string]any{
			"name":      fmt.Sprintf("LoadBr-%d", time.Now().UnixNano()),
			"address":   fmt.Sprintf("Load St %d", i),
			"latitude":  31.5 + float64(i)*0.01,
			"longitude": -8.0 + float64(i)*0.01,
		})
		if err != nil {
			return fmt.Errorf("branch %d: %w", i, err)
		}
		id, err := idOf(body, "branch")
		if err != nil {
			return err
		}
		fx.Branches = append(fx.Branches, id)
	}

	currencies := []string{"BHD", "AED", "SAR"}
	for i := 0; i < seedPackages; i++ {
		body, err := c.postJSON("/packages", map[string]any{
			"name":         fmt.Sprintf("LoadPkg-%d-%d", time.Now().UnixNano(), i),
			"durationDays": 30 * (i + 1),
			"priceCents":   25000 * (i + 1),
			"currency":     currencies[i%len(currencies)],
		})
		if err != nil {
			return fmt.Errorf("package %d: %w", i, err)
		}
		id, err := idOf(body, "package")
		if err != nil {
			return err
		}
		fx.Packages = append(fx.Packages, id)
	}

	var memberMu sync.Mutex
	memberWG := sync.WaitGroup{}
	memberErrCh := make(chan error, seedConcurrent)
	sem := make(chan struct{}, seedConcurrent)
	for bi := 0; bi < seedBranches; bi++ {
		for mi := 0; mi < seedMembers; mi++ {
			memberWG.Add(1)
			sem <- struct{}{}
			go func(bi, mi int) {
				defer func() { <-sem }()
				defer memberWG.Done()
				body, err := c.postJSON("/members", map[string]any{
					"branchId": fx.Branches[bi],
					"name":     fmt.Sprintf("LoadMember-%d-%d", bi, mi),
					"email":    fmt.Sprintf("member-%d-%d@load.local", bi, mi),
				})
				if err != nil {
					memberErrCh <- fmt.Errorf("member %d/%d: %w", bi, mi, err)
					return
				}
				id, err := idOf(body, "member")
				memberMu.Lock()
				fx.Members = append(fx.Members, id)
				memberMu.Unlock()
				if err != nil {
					memberErrCh <- err
				}
			}(bi, mi)
		}
	}
	memberWG.Wait()
	close(memberErrCh)
	for err := range memberErrCh {
		if err != nil {
			return err
		}
	}
	if len(fx.Members) != seedBranches*seedMembers {
		return fmt.Errorf("seeded %d members, want %d", len(fx.Members), seedBranches*seedMembers)
	}

	for bi := 0; bi < seedBranches; bi++ {
		for ci := 0; ci < seedClasses; ci++ {
			cap := 50
			if ci == 0 {
				cap = cfg.capacity // contention class per branch
			}
			start := time.Now().UTC().Add(time.Duration(ci) * time.Hour).Truncate(time.Minute)
			body, err := c.postJSON("/classes", map[string]any{
				"branchId": fx.Branches[bi],
				"name":     fmt.Sprintf("LoadClass-%d-%d", bi, ci),
				"startsAt": start.Format(time.RFC3339),
				"endsAt":   start.Add(time.Hour).Format(time.RFC3339),
				"capacity": cap,
			})
			if err != nil {
				return fmt.Errorf("class %d/%d: %w", bi, ci, err)
			}
			id, err := idOf(body, "class")
			if err != nil {
				return err
			}
			fx.Classes = append(fx.Classes, id)
			fx.ClassBranch = append(fx.ClassBranch, bi)
			fx.capacityMap[id] = cap
			if ci == 0 {
				fx.Contention[fx.Branches[bi]] = id
			}
		}
	}

	membershipWG := sync.WaitGroup{}
	membershipErrCh := make(chan error, seedConcurrent)
	fx.Memberships = make([]string, len(fx.Members))
	for i := range fx.Members {
		membershipWG.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer func() { <-sem }()
			defer membershipWG.Done()
			body, err := c.postJSON("/memberships", map[string]any{
				"memberId":  fx.Members[i],
				"packageId": fx.Packages[i%len(fx.Packages)],
				"branchId":  fx.Branches[i/seedMembers],
			})
			if err != nil {
				membershipErrCh <- fmt.Errorf("membership %d: %w", i, err)
				return
			}
			id, err := idOf(body, "membership")
			if err != nil {
				membershipErrCh <- err
				return
			}
			memberMu.Lock()
			fx.Memberships[i] = id
			memberMu.Unlock()
		}(i)
	}
	membershipWG.Wait()
	close(membershipErrCh)
	for err := range membershipErrCh {
		if err != nil {
			return err
		}
	}

	fmt.Printf("seeded %d branches, %d members, %d packages, %d classes, %d memberships\n",
		len(fx.Branches), len(fx.Members), len(fx.Packages), len(fx.Classes), seedBranches*seedMembers)
	return saveFixture(cfg, fx)
}
