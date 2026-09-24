package auth

// Permission is a single capability grant. Naming follows resource:action
// (for example "branches:create"). A staff member's effective permission set
// is stored on the staff row and embedded in every issued access token.
type Permission string

const (
	PermBranchesRead   Permission = "branches:read"
	PermBranchesCreate Permission = "branches:create"
	PermBranchesUpdate Permission = "branches:update"

	PermMembersRead   Permission = "members:read"
	PermMembersCreate Permission = "members:create"
	PermMembersUpdate Permission = "members:update"
	PermMembersDelete Permission = "members:delete"

	PermMembershipsRead   Permission = "memberships:read"
	PermMembershipsCreate Permission = "memberships:create"
	PermMembershipsUpdate Permission = "memberships:update"

	PermPackagesRead   Permission = "packages:read"
	PermPackagesCreate Permission = "packages:create"
	PermPackagesUpdate Permission = "packages:update"

	PermClassesRead   Permission = "classes:read"
	PermClassesCreate Permission = "classes:create"
	PermClassesUpdate Permission = "classes:update"
	PermClassesDelete Permission = "classes:delete"

	PermBookingsRead   Permission = "bookings:read"
	PermBookingsCreate Permission = "bookings:create"
	PermBookingsUpdate Permission = "bookings:update"

	PermAttendanceRead   Permission = "attendance:read"
	PermAttendanceCreate Permission = "attendance:create"
	PermAttendanceUpdate Permission = "attendance:update"

	PermBillingRead   Permission = "billing:read"
	PermBillingCreate Permission = "billing:create"
	PermBillingUpdate Permission = "billing:update"

	PermStaffRead   Permission = "staff:read"
	PermStaffCreate Permission = "staff:create"
	PermStaffUpdate Permission = "staff:update"

	PermTrainersRead   Permission = "trainers:read"
	PermTrainersCreate Permission = "trainers:create"
	PermTrainersUpdate Permission = "trainers:update"
)

// all is every valid permission.
var all = []Permission{
	PermBranchesRead, PermBranchesCreate, PermBranchesUpdate,
	PermMembersRead, PermMembersCreate, PermMembersUpdate, PermMembersDelete,
	PermMembershipsRead, PermMembershipsCreate, PermMembershipsUpdate,
	PermPackagesRead, PermPackagesCreate, PermPackagesUpdate,
	PermClassesRead, PermClassesCreate, PermClassesUpdate, PermClassesDelete,
	PermBookingsRead, PermBookingsCreate, PermBookingsUpdate,
	PermAttendanceRead, PermAttendanceCreate, PermAttendanceUpdate,
	PermBillingRead, PermBillingCreate, PermBillingUpdate,
	PermStaffRead, PermStaffCreate, PermStaffUpdate,
	PermTrainersRead, PermTrainersCreate, PermTrainersUpdate,
}

// All returns every permission, for seeding a full-access operator.
func All() []Permission {
	out := make([]Permission, len(all))
	copy(out, all)
	return out
}

var validPerms = func() map[Permission]struct{} {
	m := make(map[Permission]struct{}, len(all))
	for _, p := range all {
		m[p] = struct{}{}
	}
	return m
}()

// Has reports whether the set contains the requested permission.
func Has(set []Permission, want Permission) bool {
	for _, p := range set {
		if p == want {
			return true
		}
	}
	return false
}

// ContainsAny reports whether the set contains any of the requested permissions.
func ContainsAny(set []Permission, want []Permission) bool {
	for _, w := range want {
		if Has(set, w) {
			return true
		}
	}
	return false
}

// Known reports whether every permission in the set is a known grant.
func Known(set []Permission) bool {
	for _, p := range set {
		if _, ok := validPerms[p]; !ok {
			return false
		}
	}
	return true
}

func permStrings(set []Permission) []string {
	out := make([]string, len(set))
	for i, p := range set {
		out[i] = string(p)
	}
	return out
}
