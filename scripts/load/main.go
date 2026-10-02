// Command load drives the FitCore stress-test harness: fixture seeding plus
// vegeta-backed SLO, ramp, correctness and soak scenarios against an isolated
// load stack (deploy/docker-compose.load.yml).
//
// Usage:
//
//	load -mode seed|warmup|slo|ramp|correctness|soak [-base url] [-admin email]
//	     [-password pw] [-rate RPS] [-duration 2m] [-capacity 30] [-out dir]
//
// The admin account must already exist in the load DB (seed it with
// `just load-seed-admin`). "seed" must run once before any loaded scenario.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

type config struct {
	mode         string
	base         string
	admin        string
	password     string
	rate         int
	duration     time.Duration
	capacity     int
	out          string
	thresholdP95 time.Duration
	maxErrRate   float64
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatalf("load: %v", err)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.mode, "mode", "slo", "seed|warmup|slo|ramp|correctness|soak")
	flag.StringVar(&cfg.base, "base", "http://localhost:8081", "load stack base URL")
	flag.StringVar(&cfg.admin, "admin", "admin@load.local", "admin email with all permissions")
	flag.StringVar(&cfg.password, "password", "Password1!", "admin password")
	flag.IntVar(&cfg.rate, "rate", 150, "target requests/second for slo/soak modes")
	flag.DurationVar(&cfg.duration, "duration", 0, "scenario duration (slo/soak need it; ramp uses fixed per-step)")
	flag.IntVar(&cfg.capacity, "capacity", 30, "contention-class capacity for correctness mode")
	flag.StringVar(&cfg.out, "out", "artifacts/load", "report output directory")
	flag.DurationVar(&cfg.thresholdP95, "slo-p95", 400*time.Millisecond, "SLO max p95 latency")
	flag.Float64Var(&cfg.maxErrRate, "slo-err", 0.005, "SLO max server-error share (5xx + transport)")
	flag.Parse()
	if cfg.mode == "" {
		flag.Usage()
		os.Exit(2)
	}
	return cfg
}

func run(cfg config) error {
	switch cfg.mode {
	case "seed":
		return seed(cfg)
	case "warmup":
		return runWarmup(cfg)
	case "slo":
		if cfg.duration == 0 {
			cfg.duration = 5 * time.Minute
		}
		return runSLO(cfg)
	case "ramp":
		return runRamp(cfg)
	case "correctness":
		return runCorrectness(cfg)
	case "soak":
		if cfg.duration == 0 {
			cfg.duration = 45 * time.Minute
		}
		return runSoak(cfg)
	default:
		return fmt.Errorf("unknown mode %q", cfg.mode)
	}
}
