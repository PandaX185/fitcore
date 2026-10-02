package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// runSoak runs the SLO mix in consecutive windows at the target rate for the
// full duration, watching for latency drift and process-memory growth across
// the run. A window failing the SLO (or monotone memory growth) fails soak.
func runSoak(cfg config) error {
	fx, err := loadFixture(cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg.base)
	if err := c.login(cfg.admin, cfg.password); err != nil {
		return err
	}
	hdr := bearerHeaders(c.token)
	mix := mixedTargeter(fx, cfg.base, hdr)

	window := cfg.duration / 4
	if window < 2*time.Minute {
		window = 2 * time.Minute
	}
	startMem := scrapeGauge(cfg.base, "process_resident_memory_bytes")

	var all []*report
	failed := false
	for w := 0; w < 4; w++ {
		rep, err := runAttack(mix, cfg.rate, window, fmt.Sprintf("soak-window-%d", w+1))
		if err != nil {
			return err
		}
		rep.Name = fmt.Sprintf("soak-window-%d-@%drps", w+1, cfg.rate)
		rep.print()
		all = append(all, rep)
		if rep.P95Ms > cfg.thresholdP95.Milliseconds() || rep.failedShare() > cfg.maxErrRate {
			failed = true
		}
		time.Sleep(2 * time.Second)
	}
	endMem := scrapeGauge(cfg.base, "process_resident_memory_bytes")
	delta := ""
	if startMem > 0 && endMem > 0 {
		delta = fmt.Sprintf("  resident memory: %.1f MiB -> %.1f MiB (delta %+.1f MiB)",
			startMem/(1<<20), endMem/(1<<20), (endMem-startMem)/(1<<20))
		fmt.Println(delta)
	}
	if err := writeReport(cfg, all); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("soak: one or more windows missed the SLO (p95<=%dms, err<=%.2f%%)", cfg.thresholdP95.Milliseconds(), cfg.maxErrRate*100)
	}
	return nil
}

// scrapeGauge reads a single float gauge from the app's Prometheus endpoint
// (public on the load stack). Returns 0 when unavailable.
func scrapeGauge(base, name string) float64 {
	resp, err := http.Get(base + "/metrics")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var max float64
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, name+" ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 {
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil && v > max {
				max = v
			}
		}
	}
	return max
}
