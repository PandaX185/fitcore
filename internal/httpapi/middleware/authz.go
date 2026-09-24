package middleware

import (
	"github.com/PandaX185/fitcore/internal/modules/auth"
)

// RouteAuth describes how a matched route is guarded.
type RouteAuth struct {
	// Public routes skip authentication entirely (health, metrics, login).
	Public bool
	// Permissions required of the caller; empty means any authenticated
	// principal is allowed.
	Permissions []auth.Permission
}

// Registry maps a gin route ("METHOD /path/:param") to its authz rule. It is
// the runtime source of enforcement; api/openapi.yaml carries the matching
// x-permission annotations, and TestRegistryMatchesSpec keeps the two in
// sync.
type Registry map[string]RouteAuth

// For returns the rule for a method + gin route path.
func (r Registry) For(method, path string) (RouteAuth, bool) {
	rule, ok := r[method+" "+path]
	return rule, ok
}

type routeSpec struct {
	method string
	path   string
	public bool
	perms  []auth.Permission
}

// DefaultRegistry returns the full route authz table. Adding an endpoint
// means adding a row here and the matching x-permission in the OpenAPI spec.
func DefaultRegistry() Registry {
	specs := []routeSpec{
		{method: "GET", path: "/healthz", public: true},
		{method: "GET", path: "/readyz", public: true},
		{method: "GET", path: "/metrics", public: true},
		{method: "GET", path: "/openapi.yaml", public: true},
		{method: "GET", path: "/swagger", public: true},
		{method: "GET", path: "/swagger/*any", public: true},
		{method: "POST", path: "/auth/login", public: true},
		{method: "POST", path: "/auth/refresh", public: true},
		{method: "POST", path: "/auth/logout"},

		{method: "GET", path: "/branches", perms: perms(auth.PermBranchesRead)},
		{method: "POST", path: "/branches", perms: perms(auth.PermBranchesCreate)},
		{method: "GET", path: "/branches/:branchId", perms: perms(auth.PermBranchesRead)},
		{method: "PATCH", path: "/branches/:branchId", perms: perms(auth.PermBranchesUpdate)},
		{method: "GET", path: "/branches/:branchId/staff", perms: perms(auth.PermStaffRead)},
		{method: "GET", path: "/branches/:branchId/trainers", perms: perms(auth.PermTrainersRead)},

		{method: "GET", path: "/members", perms: perms(auth.PermMembersRead)},
		{method: "POST", path: "/members", perms: perms(auth.PermMembersCreate)},
		{method: "GET", path: "/members/:memberId", perms: perms(auth.PermMembersRead)},
		{method: "PATCH", path: "/members/:memberId", perms: perms(auth.PermMembersUpdate)},
		{method: "DELETE", path: "/members/:memberId", perms: perms(auth.PermMembersDelete)},
		{method: "GET", path: "/members/:memberId/memberships", perms: perms(auth.PermMembershipsRead)},
		{method: "GET", path: "/members/:memberId/attendance", perms: perms(auth.PermAttendanceRead)},
		{method: "GET", path: "/members/:memberId/invoices", perms: perms(auth.PermBillingRead)},

		{method: "POST", path: "/memberships", perms: perms(auth.PermMembershipsCreate)},
		{method: "GET", path: "/memberships/:membershipId", perms: perms(auth.PermMembershipsRead)},
		{method: "PATCH", path: "/memberships/:membershipId", perms: perms(auth.PermMembershipsUpdate)},

		{method: "GET", path: "/packages", perms: perms(auth.PermPackagesRead)},
		{method: "POST", path: "/packages", perms: perms(auth.PermPackagesCreate)},
		{method: "GET", path: "/packages/:packageId", perms: perms(auth.PermPackagesRead)},
		{method: "PATCH", path: "/packages/:packageId", perms: perms(auth.PermPackagesUpdate)},

		{method: "GET", path: "/classes", perms: perms(auth.PermClassesRead)},
		{method: "POST", path: "/classes", perms: perms(auth.PermClassesCreate)},
		{method: "GET", path: "/classes/:classId", perms: perms(auth.PermClassesRead)},
		{method: "PATCH", path: "/classes/:classId", perms: perms(auth.PermClassesUpdate)},
		{method: "DELETE", path: "/classes/:classId", perms: perms(auth.PermClassesDelete)},
		{method: "GET", path: "/classes/:classId/bookings", perms: perms(auth.PermBookingsRead)},

		{method: "POST", path: "/bookings", perms: perms(auth.PermBookingsCreate)},
		{method: "GET", path: "/bookings/:bookingId", perms: perms(auth.PermBookingsRead)},
		{method: "POST", path: "/bookings/:bookingId/cancel", perms: perms(auth.PermBookingsUpdate)},

		{method: "POST", path: "/attendance/check-in", perms: perms(auth.PermAttendanceCreate)},
		{method: "POST", path: "/attendance/check-out", perms: perms(auth.PermAttendanceUpdate)},
		{method: "GET", path: "/attendance/:attendanceId", perms: perms(auth.PermAttendanceRead)},

		{method: "POST", path: "/invoices", perms: perms(auth.PermBillingCreate)},
		{method: "GET", path: "/invoices/:invoiceId", perms: perms(auth.PermBillingRead)},
		{method: "PATCH", path: "/invoices/:invoiceId", perms: perms(auth.PermBillingUpdate)},

		{method: "POST", path: "/staff", perms: perms(auth.PermStaffCreate)},
		{method: "GET", path: "/staff/:staffId", perms: perms(auth.PermStaffRead)},
		{method: "PATCH", path: "/staff/:staffId", perms: perms(auth.PermStaffUpdate)},

		{method: "POST", path: "/trainers", perms: perms(auth.PermTrainersCreate)},
		{method: "GET", path: "/trainers/:trainerId", perms: perms(auth.PermTrainersRead)},
		{method: "PATCH", path: "/trainers/:trainerId", perms: perms(auth.PermTrainersUpdate)},
	}

	r := make(Registry, len(specs))
	for _, s := range specs {
		r[s.method+" "+s.path] = RouteAuth{Public: s.public, Permissions: s.perms}
	}
	return r
}

func perms(p ...auth.Permission) []auth.Permission { return p }
