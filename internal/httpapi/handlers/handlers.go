package handlers

import (
	"log/slog"

	"github.com/PandaX185/fitcore/internal/modules/attendance"
	"github.com/PandaX185/fitcore/internal/modules/billing"
	"github.com/PandaX185/fitcore/internal/modules/bookings"
	"github.com/PandaX185/fitcore/internal/modules/branches"
	"github.com/PandaX185/fitcore/internal/modules/classes"
	"github.com/PandaX185/fitcore/internal/modules/members"
	"github.com/PandaX185/fitcore/internal/modules/memberships"
	"github.com/PandaX185/fitcore/internal/modules/packages"
	"github.com/PandaX185/fitcore/internal/modules/staff"
	"github.com/PandaX185/fitcore/internal/modules/trainers"
	"github.com/PandaX185/fitcore/internal/platform/postgres"
	"github.com/PandaX185/fitcore/internal/platform/telemetry"
)

// Handler implements the generated openapi.ServerInterface. Module adapters
// are embedded so their promoted methods cover the whole interface.
type Handler struct {
	*authHandler
	*branchesHandler
	*membersHandler
	*membershipsHandler
	*packagesHandler
	*classesHandler
	*bookingsHandler
	*attendanceHandler
	*billingHandler
	*staffHandler
	*trainersHandler
}

func New(log *slog.Logger, metrics *telemetry.Metrics, db *postgres.DB, authSvc authService) *Handler {
	txm := postgres.NewTransactionManager(db.Gorm())
	branchSvc := branches.NewService(postgres.NewBranchRepository(db))
	memberRepo := postgres.NewMemberRepository(db)
	membershipRepo := postgres.NewMembershipRepository(db)
	invoiceRepo := postgres.NewInvoiceRepository(db)
	attendanceRepo := postgres.NewAttendanceRepository(db)
	classRepo := postgres.NewClassRepository(db)
	bookingRepo := postgres.NewBookingRepository(db)
	memberSvc := members.NewService(memberRepo, membershipRepo, invoiceRepo, attendanceRepo, txm)
	packageSvc := packages.NewService(postgres.NewPackageRepository(db))
	membershipSvc := memberships.NewService(membershipRepo, packageSvc, memberSvc, branchSvc, txm)
	staffSvc := staff.NewService(postgres.NewStaffRepository(db), branchSvc)
	trainerSvc := trainers.NewService(postgres.NewTrainerRepository(db), branchSvc)
	classSvc := classes.NewService(classRepo, branchSvc, trainerSvc, bookingRepo, txm)
	bookingSvc := bookings.NewService(bookingRepo, classSvc, memberSvc, txm)
	attendanceSvc := attendance.NewService(attendanceRepo, memberSvc, membershipSvc, txm)
	billingSvc := billing.NewService(invoiceRepo, memberSvc, membershipSvc, txm)

	return &Handler{
		authHandler: &authHandler{
			svc:     authSvc,
			log:     log,
			metrics: metrics,
		},
		branchesHandler: &branchesHandler{
			svc:     branchSvc,
			log:     log,
			metrics: metrics,
		},
		membersHandler: &membersHandler{
			svc:     memberSvc,
			log:     log,
			metrics: metrics,
		},
		membershipsHandler: &membershipsHandler{
			svc:     membershipSvc,
			log:     log,
			metrics: metrics,
		},
		packagesHandler: &packagesHandler{
			svc:     packageSvc,
			log:     log,
			metrics: metrics,
		},
		classesHandler: &classesHandler{
			svc:     classSvc,
			log:     log,
			metrics: metrics,
		},
		bookingsHandler: &bookingsHandler{
			svc:     bookingSvc,
			log:     log,
			metrics: metrics,
		},
		attendanceHandler: &attendanceHandler{
			svc:     attendanceSvc,
			log:     log,
			metrics: metrics,
		},
		billingHandler: &billingHandler{
			svc:     billingSvc,
			log:     log,
			metrics: metrics,
		},
		staffHandler: &staffHandler{
			svc:     staffSvc,
			log:     log,
			metrics: metrics,
		},
		trainersHandler: &trainersHandler{
			svc:     trainerSvc,
			log:     log,
			metrics: metrics,
		},
	}
}
