package middleware_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/PandaX185/fitcore/internal/httpapi/middleware"
	"github.com/PandaX185/fitcore/internal/modules/auth"
)

var paramRe = regexp.MustCompile(`\{([^}]+)\}`)

// ginPath converts an OpenAPI path template to the gin ":param" form the
// registry is keyed on.
func ginPath(specPath string) string {
	return paramRe.ReplaceAllString(specPath, ":$1")
}

// specDir returns the repository root (parent of internal/httpapi/middleware).
func specDir() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "api", "openapi.yaml")); err == nil {
			return filepath.Join(dir, "api", "openapi.yaml")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

type specOp struct {
	public bool
	perms  []auth.Permission
}

func loadSpecOps(t *testing.T) map[string]specOp {
	t.Helper()
	specPath := specDir()
	if specPath == "" {
		t.Fatal("could not locate api/openapi.yaml from test working directory")
	}
	// The spec is a checked-in repo artifact; reading it is not user input.
	//nolint:gosec
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("spec has no paths object")
	}

	ops := map[string]specOp{}
	for specPath, item := range paths {
		methods, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("path %q is not an object", specPath)
		}
		for method, op := range methods {
			if !isHTTPMethod(method) {
				continue // path-item keys like "parameters" are not operations
			}
			opMap, ok := op.(map[string]any)
			if !ok {
				t.Fatalf("%s %s is not an object", method, specPath)
			}
			o := specOp{public: isPublic(opMap)}
			if xperm, ok := opMap["x-permission"].(string); ok && strings.TrimSpace(xperm) != "" {
				o.perms = []auth.Permission{auth.Permission(xperm)}
			}
			ops[strings.ToUpper(method)+" "+ginPath(specPath)] = o
		}
	}
	return ops
}

func isHTTPMethod(m string) bool {
	switch m {
	case "get", "put", "post", "patch", "delete", "head", "options", "trace":
		return true
	}
	return false
}

func isPublic(op map[string]any) bool {
	sec, ok := op["security"]
	if !ok {
		return false
	}
	arr, ok := sec.([]any)
	return ok && len(arr) == 0
}

// TestRegistryMatchesSpec is the drift test: every operation in the OpenAPI
// spec must have a matching rule in DefaultRegistry with identical public-ness
// and permissions, and every registry-visible route must exist in the spec.
func TestRegistryMatchesSpec(t *testing.T) {
	ops := loadSpecOps(t)
	registry := middleware.DefaultRegistry()

	allowlist := map[string]bool{
		"GET /openapi.yaml": true,
		"GET /swagger":      true,
		"GET /swagger/*any": true,
	}

	for key, want := range ops {
		rule, ok := registry.For(strings.SplitN(key, " ", 2)[0], strings.SplitN(key, " ", 2)[1])
		if !ok {
			t.Errorf("registry missing rule for %s (add a row in DefaultRegistry)", key)
			continue
		}
		if rule.Public != want.public {
			t.Errorf("%s: registry public=%v, spec says %v", key, rule.Public, want.public)
		}
		if !permsEqual(rule.Permissions, want.perms) {
			t.Errorf("%s: registry perms=%v, spec x-permission=%v", key, rule.Permissions, want.perms)
		}
	}

	for key := range registry {
		if _, ok := ops[key]; !ok && !allowlist[key] {
			t.Errorf("registry route %s is not present in api/openapi.yaml", key)
		}
	}
}

func permsEqual(a, b []auth.Permission) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestRegistryPermissionsKnown guards the registry against typos: every
// permission named there must be part of the defined vocabulary.
func TestRegistryPermissionsKnown(t *testing.T) {
	for key, rule := range middleware.DefaultRegistry() {
		if rule.Public {
			if len(rule.Permissions) != 0 {
				t.Errorf("%s is public but lists permissions %v", key, rule.Permissions)
			}
			continue
		}
		if !auth.Known(rule.Permissions) {
			t.Errorf("%s references unknown permissions %v", key, rule.Permissions)
		}
	}
}
