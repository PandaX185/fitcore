package auth

import (
	"testing"
)

func TestHas(t *testing.T) {
	tests := []struct {
		name string
		set  []Permission
		want Permission
		got  bool
	}{
		{name: "present", set: []Permission{PermBranchesRead}, want: PermBranchesRead, got: true},
		{name: "absent", set: []Permission{PermBranchesCreate}, want: PermBranchesRead, got: false},
		{name: "empty set", set: nil, want: PermBranchesRead, got: false},
		{name: "empty want", set: []Permission{PermBranchesRead}, want: "", got: false},
		{name: "duplicate set", set: []Permission{PermBranchesRead, PermBranchesRead}, want: PermBranchesRead, got: true},
		{name: "multiple entries", set: []Permission{PermMembersRead, PermStaffRead}, want: PermStaffRead, got: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Has(tt.set, tt.want); got != tt.got {
				t.Fatalf("Has(%v, %q) = %v, want %v", tt.set, tt.want, got, tt.got)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name string
		set  []Permission
		want []Permission
		got  bool
	}{
		{name: "single hit", set: []Permission{PermBranchesRead}, want: []Permission{PermBranchesRead}, got: true},
		{name: "any of several", set: []Permission{PermBranchesRead}, want: []Permission{PermBranchesCreate, PermBranchesUpdate, PermBranchesRead}, got: true},
		{name: "none match", set: []Permission{PermBranchesRead}, want: []Permission{PermBranchesCreate, PermBranchesUpdate}, got: false},
		{name: "empty want", set: []Permission{PermBranchesRead, PermMembersRead}, want: nil, got: false},
		{name: "empty set", set: nil, want: []Permission{PermBranchesRead}, got: false},
		{name: "both empty", set: nil, want: nil, got: false},
		{name: "duplicate want", set: []Permission{PermStaffRead}, want: []Permission{PermStaffRead, PermStaffRead}, got: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsAny(tt.set, tt.want); got != tt.got {
				t.Fatalf("ContainsAny(%v, %v) = %v, want %v", tt.set, tt.want, got, tt.got)
			}
		})
	}
}

func TestKnown(t *testing.T) {
	// All real permissions must be recognised.
	if !Known(All()) {
		t.Fatal("Known(All()) = false, want true")
	}
	// Each constant individually is known.
	for _, p := range all {
		if !Known([]Permission{p}) {
			t.Fatalf("Known([%s]) = false, want true", p)
		}
	}
}

func TestKnownRejectsUnknown(t *testing.T) {
	for _, tt := range []struct {
		name string
		perm []Permission
		want bool
	}{
		{name: "unknown string", perm: []Permission{Permission("branches:explode")}, want: false},
		{name: "mixed", perm: []Permission{PermBranchesRead, Permission("nope")}, want: false},
		{name: "empty string", perm: []Permission{Permission("")}, want: false},
		{name: "wrong case", perm: []Permission{Permission("BRANCHES:READ")}, want: false},
		{name: "trailing space", perm: []Permission{Permission("branches:read ")}, want: false},
		{name: "unknown in large set", perm: append(All(), Permission("hack:all")), want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Known(tt.perm); got != tt.want {
				t.Fatalf("Known(%v) = %v, want %v", tt.perm, got, tt.want)
			}
		})
	}
}

func TestKnownNil(t *testing.T) {
	// An empty set is vacuously "all known": the guard only rejects a set
	// that contains an unknown grant.
	if !Known(nil) {
		t.Fatal("Known(nil) = false, want true")
	}
	if !Known([]Permission{}) {
		t.Fatal("Known([]) = false, want true")
	}
}

func TestAll(t *testing.T) {
	got := All()
	want := all
	if len(got) != len(want) {
		t.Fatalf("All() = %d permissions, want %d", len(got), len(want))
	}
	seen := map[Permission]bool{}
	for i, p := range got {
		if seen[p] {
			t.Fatalf("All() contains duplicate %q", p)
		}
		seen[p] = true
		if want[i] != p {
			t.Fatalf("All()[%d] = %q, want %q", i, p, want[i])
		}
	}
	// All() must not alias the package-level slice: mutating the copy must
	// not change a second call's result.
	got[0] = Permission("mutated")
	if got2 := All(); got2[0] == Permission("mutated") {
		t.Fatal("All() returned a shared backing array")
	}
}

func TestPermStrings(t *testing.T) {
	in := []Permission{PermBranchesRead, PermMembersDelete}
	out := permStrings(in)
	if len(out) != 2 || out[0] != "branches:read" || out[1] != "members:delete" {
		t.Fatalf("permStrings(%v) = %v", in, out)
	}
	if got := permStrings(nil); len(got) != 0 {
		t.Fatalf("permStrings(nil) = %v, want empty", got)
	}
}
