package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// createMigration writes a {version}_{name}.up.sql/.down.sql pair.
func createMigration(dir, name string) error {
	if name == "" {
		return fmt.Errorf("create requires -name")
	}
	version, err := nextVersion(dir)
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("%06d", version)
	base := prefix + "_" + name
	for _, ext := range []string{".up.sql", ".down.sql"} {
		path := filepath.Join(dir, base+ext)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return err
		}
		fmt.Println("created", path)
	}
	return nil
}

func nextVersion(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := strings.SplitN(e.Name(), "_", 2)[0]
		n, err := strconv.Atoi(base)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}
