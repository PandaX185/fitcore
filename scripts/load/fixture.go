package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// fixturePath returns the JSON file where seed persists the generated IDs so
// later scenarios reuse the same members/classes/branches.
func fixturePath(out string) string {
	return filepath.Join(out, "fixture.json")
}

func saveFixture(cfg config, fx *fixture) error {
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fixturePath(cfg.out), raw, 0o644)
}

func loadFixture(cfg config) (*fixture, error) {
	raw, err := os.ReadFile(fixturePath(cfg.out))
	if err != nil {
		return nil, fmt.Errorf("load fixture (run `load -mode seed` first): %w", err)
	}
	fx := &fixture{Contention: map[string]string{}, capacityMap: map[string]int{}}
	if err := json.Unmarshal(raw, fx); err != nil {
		return nil, err
	}
	return fx, nil
}
