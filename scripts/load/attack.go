package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tsenart/vegeta/v12/lib"
)

// runAttack drives vegeta at a fixed rate for the given duration and returns
// a classified report. Transport errors, 5xx and 4xx are counted separately:
// SLO thresholds key off 5xx + transport only, because under concurrency
// stateful 409s (already checked in, class full, cancelled) are legitimate
// API outcomes, not failures. They are still reported per scenario.
func runAttack(tr vegeta.Targeter, rate int, dur time.Duration, name string) (*report, error) {
	atk := vegeta.NewAttacker(
		vegeta.Timeout(30*time.Second),
		vegeta.Workers(50),
		vegeta.MaxWorkers(500),
		vegeta.KeepAlive(true),
		vegeta.MaxBody(1<<20),
	)
	pacer := vegeta.Rate{Freq: rate, Per: time.Second}

	m := &vegeta.Metrics{}
	var results []*vegeta.Result
	for res := range atk.Attack(tr, pacer, dur, name) {
		m.Add(res)
		results = append(results, res)
	}
	m.Close()

	rep := &report{Name: name}
	rep.DurationSeconds = m.Duration.Seconds()
	rep.Requests = m.Requests
	rep.Rate = round2(m.Rate)
	rep.Throughput = round2(m.Throughput)
	rep.P50Ms = m.Latencies.P50.Milliseconds()
	rep.P90Ms = m.Latencies.P90.Milliseconds()
	rep.P95Ms = m.Latencies.P95.Milliseconds()
	rep.P99Ms = m.Latencies.P99.Milliseconds()
	rep.MaxMs = m.Latencies.Max.Milliseconds()
	for code, n := range m.StatusCodes {
		c, err := strconv.Atoi(code)
		if err != nil {
			continue
		}
		switch {
		case c >= 500:
			rep.ServerErrors += uint64(n)
		case c >= 400:
			rep.ClientErrors += uint64(n)
		}
	}
	// Transport-level failures (refused/timeout/reset) carry code 0 plus a
	// non-empty Error.
	for _, r := range results {
		if r.Code == 0 && r.Error != "" {
			rep.TransportErrors++
		}
	}
	rep.Total = rep.ServerErrors + rep.ClientErrors + rep.TransportErrors
	rep.Errors = m.Errors
	return rep, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type report struct {
	Name            string   `json:"name"`
	Requests        uint64   `json:"requests"`
	Rate            float64  `json:"rate"`
	Throughput      float64  `json:"throughput_req_s"`
	P50Ms           int64    `json:"p50_ms"`
	P90Ms           int64    `json:"p90_ms"`
	P95Ms           int64    `json:"p95_ms"`
	P99Ms           int64    `json:"p99_ms"`
	MaxMs           int64    `json:"max_ms"`
	DurationSeconds float64  `json:"duration_s"`
	ServerErrors    uint64   `json:"server_5xx"`
	ClientErrors    uint64   `json:"client_4xx"`
	TransportErrors uint64   `json:"transport_errors"`
	Total           uint64   `json:"failed_total"`
	Errors          []string `json:"error_detail,omitempty"`
}

// p95Ms returns the p95 latency in milliseconds (0 when empty).
func (r *report) p95Ms() int64 { return r.P95Ms }

func (r *report) failedShare() float64 {
	if r.Requests == 0 {
		return 1
	}
	return float64(r.ServerErrors+r.TransportErrors) / float64(r.Requests)
}

func (r *report) print() {
	fmt.Printf("  %-30s req=%-7d %6.1f rps  p50=%-5d p90=%-5d p95=%-5d max=%-5d  5xx=%-4d 4xx=%-4d transport=%-3d err=%.2f%%\n",
		r.Name, r.Requests, r.Rate, r.P50Ms, r.P90Ms, r.P95Ms, r.MaxMs, r.ServerErrors, r.ClientErrors, r.TransportErrors, r.failedShare()*100)
}

// writeReport appends reports to a per-run JSON file in cfg.out.
func writeReport(cfg config, reps []*report) error {
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cfg.out, "report-"+time.Now().Format("20060102-150405")+".json")
	raw, err := json.MarshalIndent(reps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// bearerHeaders returns the Authorization header reused across requests.
func bearerHeaders(token string) http.Header {
	return http.Header{"Authorization": []string{"Bearer " + token}, "Content-Type": []string{"application/json"}}
}
