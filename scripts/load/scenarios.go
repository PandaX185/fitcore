package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/tsenart/vegeta/v12/lib"
)

// ---- request builders ----

func getTarget(base, path string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		t.Method = http.MethodGet
		t.URL = base + path
		t.Header = hdr
		t.Body = nil
		return nil
	}
}

func postTarget(base, path string, hdr http.Header, body string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		t.Method = http.MethodPost
		t.URL = base + path
		t.Header = hdr
		t.Body = []byte(body)
		return nil
	}
}

// ---- fixture-backed targeters (random member/class per request) ----

func readMemberTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		mi := rand.Intn(len(fx.Members))
		t.Method = http.MethodGet
		t.URL = base + "/members/" + fx.Members[mi]
		t.Header = hdr
		t.Body = nil
		return nil
	}
}

func listMembersTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		t.Method = http.MethodGet
		t.URL = base + fmt.Sprintf("/members?limit=100&page=%d", rand.Intn(4)+1)
		t.Header = hdr
		t.Body = nil
		return nil
	}
}

func listClassesTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		t.Method = http.MethodGet
		t.URL = base + fmt.Sprintf("/classes?limit=100&page=%d", rand.Intn(3)+1)
		t.Header = hdr
		t.Body = nil
		return nil
	}
}

func memberAttendanceTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		mi := rand.Intn(len(fx.Members))
		t.Method = http.MethodGet
		t.URL = base + "/members/" + fx.Members[mi] + "/attendance?limit=100"
		t.Header = hdr
		t.Body = nil
		return nil
	}
}

func checkInTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		mi := rand.Intn(len(fx.Members))
		t.Method = http.MethodPost
		t.URL = base + "/attendance/check-in"
		t.Header = hdr
		t.Body = []byte(fmt.Sprintf(`{"memberId":%q,"branchId":%q}`, fx.Members[mi], fx.Branches[mi/seedMembers]))
		return nil
	}
}

func checkOutTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		mi := rand.Intn(len(fx.Members))
		t.Method = http.MethodPost
		t.URL = base + "/attendance/check-out"
		t.Header = hdr
		t.Body = []byte(fmt.Sprintf(`{"memberId":%q}`, fx.Members[mi]))
		return nil
	}
}

// createBookingTarget aligns the member and class to the same branch so
// capacity bookkeeping stays semantically clean.
func createBookingTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		bi := rand.Intn(len(fx.Branches))
		mi := bi*seedMembers + rand.Intn(seedMembers)
		ci := bi*seedClasses + rand.Intn(seedClasses)
		t.Method = http.MethodPost
		t.URL = base + "/bookings"
		t.Header = hdr
		t.Body = []byte(fmt.Sprintf(`{"classId":%q,"memberId":%q}`, fx.Classes[ci], fx.Members[mi]))
		return nil
	}
}

func createInvoiceTarget(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		mi := rand.Intn(len(fx.Members))
		t.Method = http.MethodPost
		t.URL = base + "/invoices"
		t.Header = hdr
		t.Body = []byte(fmt.Sprintf(
			`{"memberId":%q,"membershipId":%q,"amountCents":25000,"currency":"BHD","dueAt":%q,"status":"pending"}`,
			fx.Members[mi], fx.Memberships[mi], time.Now().UTC().Add(30*24*time.Hour).Format(time.RFC3339)))
		return nil
	}
}

func loginTarget(base, email, password string) vegeta.Targeter {
	return postTarget(base, "/auth/login",
		http.Header{"Content-Type": []string{"application/json"}},
		fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
}

type weighted struct {
	n  int
	tr vegeta.Targeter
}

func weightedTargeter(ws []weighted) vegeta.Targeter {
	var items []vegeta.Targeter
	// Expand each weighted targeter into n copies; total is small enough that
	// re-picking is simpler and keeps weights exact across a run.
	for _, w := range ws {
		for i := 0; i < w.n; i++ {
			items = append(items, w.tr)
		}
	}
	return func(t *vegeta.Target) error {
		return items[rand.Intn(len(items))](t)
	}
}

// ---- scenarios ----

// runWarmup validates the stack with a single-threaded conversational walk
// before any load is applied. It exits nonzero on the first failure.
func runWarmup(cfg config) error {
	fx, err := loadFixture(cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg.base)
	if err := c.login(cfg.admin, cfg.password); err != nil {
		return err
	}
	mi := rand.Intn(len(fx.Members))
	member := fx.Members[mi]
	cases := []struct {
		name string
		path string
		body any
	}{
		{"list members", "/members?limit=100", nil},
		{"list classes", "/classes?limit=100", nil},
		{"get member", "/members/" + member, nil},
		{"member attendance", "/members/" + member + "/attendance?limit=100", nil},
		{"check-in", "/attendance/check-in", map[string]string{"memberId": member, "branchId": fx.Branches[mi/seedMembers]}},
		{"check-out", "/attendance/check-out", map[string]string{"memberId": member}},
		{"create booking", "/bookings", map[string]string{"classId": fx.Classes[mi/seedMembers*seedClasses], "memberId": member}},
		{"create invoice", "/invoices", map[string]any{
			"memberId": member, "membershipId": fx.Memberships[mi],
			"amountCents": 25000, "currency": "BHD", "dueAt": time.Now().UTC().AddDate(0, 1, 0).Format(time.RFC3339), "status": "pending",
		}},
	}
	for _, tc := range cases {
		var payload []byte
		method := "GET"
		if tc.body != nil {
			raw, err := json.Marshal(tc.body)
			if err != nil {
				return err
			}
			payload = raw
			method = "POST"
		}
		body, status, err := c.do(method, tc.path, payload)
		if err != nil {
			return fmt.Errorf("warmup %s: %w", tc.name, err)
		}
		if status >= 400 {
			return fmt.Errorf("warmup %s: HTTP %d: %s", tc.name, status, truncate(string(body), 200))
		}
		fmt.Printf("  warmup OK  %-22s HTTP %d\n", tc.name, status)
	}
	return nil
}

// mixedTargeter is the representative production mix: 70% reads, 20% check-in
// /check-out churn, 10% write-heavy (bookings + invoices).
func mixedTargeter(fx *fixture, base string, hdr http.Header) vegeta.Targeter {
	return weightedTargeter([]weighted{
		{20, readMemberTarget(fx, base, hdr)},
		{20, listMembersTarget(fx, base, hdr)},
		{15, listClassesTarget(fx, base, hdr)},
		{15, memberAttendanceTarget(fx, base, hdr)},
		{10, checkInTarget(fx, base, hdr)},
		{10, checkOutTarget(fx, base, hdr)},
		{5, createBookingTarget(fx, base, hdr)},
		{5, createInvoiceTarget(fx, base, hdr)},
	})
}

// runSLO runs the mixed realistic workload and checks the SLO thresholds
// (p95 latency and 5xx+transport share). 4xx from stateful races are reported
// but excluded from the failure decision.
func runSLO(cfg config) error {
	fx, err := loadFixture(cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg.base)
	if err := c.login(cfg.admin, cfg.password); err != nil {
		return err
	}
	rep, err := runAttack(mixedTargeter(fx, cfg.base, bearerHeaders(c.token)), cfg.rate, cfg.duration, "SLO-mixed")
	if err != nil {
		return err
	}
	rep.Name = fmt.Sprintf("SLO-mixed-@%drps", cfg.rate)
	rep.print()
	if err := writeReport(cfg, []*report{rep}); err != nil {
		return err
	}
	passed := rep.P95Ms <= cfg.thresholdP95.Milliseconds() && rep.failedShare() <= cfg.maxErrRate
	fmt.Printf("  SLO: p95<=%dms, 5xx+transport<=%.2f%%  =>  ", cfg.thresholdP95.Milliseconds(), cfg.maxErrRate*100)
	if passed {
		fmt.Println("PASS")
		return nil
	}
	fmt.Println("FAIL")
	os.Exit(1)
	return nil
}

// runRamp scales RPS toward the ceiling per endpoint and records the highest
// rate that stayed under the stop gate (p95 < 5s and 5xx+transport < 5%).
func runRamp(cfg config) error {
	fx, err := loadFixture(cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg.base)
	if err := c.login(cfg.admin, cfg.password); err != nil {
		return err
	}
	hdr := bearerHeaders(c.token)

	scenarios := []struct {
		name  string
		steps []int
		make  func() vegeta.Targeter
	}{
		{"login", []int{5, 10, 20, 40, 60, 80, 100, 125, 150, 200, 250, 300}, func() vegeta.Targeter { return loginTarget(cfg.base, cfg.admin, cfg.password) }},
		{"read-member", []int{20, 50, 100, 200, 300, 400, 600, 800, 1200, 1600, 2000, 3000, 5000, 10000}, func() vegeta.Targeter { return readMemberTarget(fx, cfg.base, hdr) }},
		{"list-classes", []int{20, 50, 100, 200, 300, 400, 600, 800, 1200, 1600, 2000, 3000, 5000, 10000}, func() vegeta.Targeter { return listClassesTarget(fx, cfg.base, hdr) }},
		{"check-in", []int{20, 50, 100, 200, 300, 400, 600, 800, 1200, 1600, 2000, 3000}, func() vegeta.Targeter { return checkInTarget(fx, cfg.base, hdr) }},
		{"create-booking", []int{20, 50, 100, 200, 300, 400, 600, 800, 1200, 1600, 2000, 3000}, func() vegeta.Targeter { return createBookingTarget(fx, cfg.base, hdr) }},
		{"invoices-create", []int{20, 50, 100, 200, 300, 400, 600, 800, 1200, 1600, 2000, 3000}, func() vegeta.Targeter { return createInvoiceTarget(fx, cfg.base, hdr) }},
	}

	var all []*report
	for _, sc := range scenarios {
		fmt.Printf("ramping %s\n", sc.name)
		peaked := 0
		for _, rps := range sc.steps {
			rep, err := runAttack(sc.make(), rps, 30*time.Second, fmt.Sprintf("%s@%drps", sc.name, rps))
			if err != nil {
				return err
			}
			rep.print()
			all = append(all, rep)
			if rep.failedShare() > 0.05 || rep.P95Ms > 5000 {
				fmt.Printf("    breakpoint at %d rps (p95=%dms err=%.2f%%)\n", rps, rep.P95Ms, rep.failedShare()*100)
				break
			}
			peaked = rps
		}
		fmt.Printf("  %s stable through %d rps\n\n", sc.name, peaked)
	}
	return writeReport(cfg, all)
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
