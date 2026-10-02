package members

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/paging"
)

type fakeRepo struct {
	create  func(ctx context.Context, m *Member) error
	getByID func(ctx context.Context, id uuid.UUID) (*Member, error)
	update  func(ctx context.Context, id uuid.UUID, patch *Patch) error
	delete  func(ctx context.Context, id uuid.UUID) error
	list    func(ctx context.Context, q *ListQuery) ([]*Member, error)
}

func (f fakeRepo) Create(ctx context.Context, m *Member) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, m)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Member, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}
func (f fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.delete == nil {
		return nil
	}
	return f.delete(ctx, id)
}
func (f fakeRepo) List(ctx context.Context, q *ListQuery) ([]*Member, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, q)
}

// fakeCheckers backs the delete-guard ports with programmable probes; the
// zero value reports no dependents.
type fakeCheckers struct {
	hasActive  func(ctx context.Context, memberID uuid.UUID) (bool, error)
	hasPending func(ctx context.Context, memberID uuid.UUID) (bool, error)
	hasOpen    func(ctx context.Context, memberID uuid.UUID) (bool, error)
}

func (f fakeCheckers) HasActiveByMember(ctx context.Context, memberID uuid.UUID) (bool, error) {
	if f.hasActive == nil {
		return false, nil
	}
	return f.hasActive(ctx, memberID)
}

func (f fakeCheckers) HasPendingByMember(ctx context.Context, memberID uuid.UUID) (bool, error) {
	if f.hasPending == nil {
		return false, nil
	}
	return f.hasPending(ctx, memberID)
}

func (f fakeCheckers) HasOpenByMember(ctx context.Context, memberID uuid.UUID) (bool, error) {
	if f.hasOpen == nil {
		return false, nil
	}
	return f.hasOpen(ctx, memberID)
}

type stubTx struct{}

func (stubTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestServiceGet(t *testing.T) {
	id := uuid.New()
	sentinel := &Member{ID: id}
	svc := NewService(fakeRepo{
		getByID: func(ctx context.Context, got uuid.UUID) (*Member, error) {
			if got != id {
				t.Fatalf("GetByID id = %v, want %v", got, id)
			}
			return sentinel, nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})

	got, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if got != sentinel {
		t.Fatalf("Get = %v, want %v", got, sentinel)
	}
}

func TestServiceGetNilID(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if _, err := svc.Get(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Get nil id = %v, want ErrInvalidInput", err)
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Member, error) {
			return nil, ErrNotFound
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
}

func TestServiceCreate(t *testing.T) {
	branchID := uuid.New()
	var created *Member
	svc := NewService(fakeRepo{
		create: func(ctx context.Context, m *Member) error {
			created = m
			return nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})

	got, err := svc.Create(context.Background(), branchID, "Ada", "ada@example.com", "123")
	if err != nil {
		t.Fatalf("Create: unexpected error %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("Create returned %v, repo saw %v", got.ID, created.ID)
	}
	if created.BranchID != branchID {
		t.Fatalf("branchID = %v, want %v", created.BranchID, branchID)
	}
	if created.Status != StatusActive {
		t.Fatalf("status = %q, want %q", created.Status, StatusActive)
	}
	if created.Email != "ada@example.com" {
		t.Fatalf("email = %q", created.Email)
	}
}

func TestServiceCreateInvalid(t *testing.T) {
	branchID := uuid.New()
	tests := []struct {
		name     string
		branchID uuid.UUID
		member   string
		email    string
	}{
		{"nil branch", uuid.Nil, "Ada", "ada@example.com"},
		{"blank name", branchID, " ", "ada@example.com"},
		{"blank email", branchID, "Ada", ""},
		{"no at sign", branchID, "Ada", "not-an-email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
			_, err := svc.Create(context.Background(), tt.branchID, tt.member, tt.email, "")
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceCreateNormalizesEmail(t *testing.T) {
	branchID := uuid.New()
	var created *Member
	svc := NewService(fakeRepo{
		create: func(ctx context.Context, m *Member) error {
			created = m
			return nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if _, err := svc.Create(context.Background(), branchID, "Ada", "  Ada@Example.COM  ", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Email != "ada@example.com" {
		t.Fatalf("email = %q, want %q", created.Email, "ada@example.com")
	}
}

func TestServiceCreateDuplicate(t *testing.T) {
	svc := NewService(fakeRepo{
		create: func(context.Context, *Member) error {
			return ErrDuplicateEmail
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	_, err := svc.Create(context.Background(), uuid.New(), "Ada", "ada@example.com", "")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("Create dup = %v, want ErrDuplicateEmail", err)
	}
}

func TestServiceUpdate(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{
		update: func(ctx context.Context, got uuid.UUID, patch *Patch) error {
			if got != id {
				t.Fatalf("Update id = %v, want %v", got, id)
			}
			if patch.Status == nil || *patch.Status != StatusSuspended {
				t.Fatalf("patch = %+v", patch)
			}
			return nil
		},
		getByID: func(ctx context.Context, id uuid.UUID) (*Member, error) {
			return &Member{ID: id, Status: StatusSuspended}, nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	s := StatusSuspended
	got, err := svc.Update(context.Background(), id, Patch{Status: &s})
	if err != nil {
		t.Fatalf("Update: unexpected error %v", err)
	}
	if got.Status != StatusSuspended {
		t.Fatalf("status = %q, want %q", got.Status, StatusSuspended)
	}
}

func TestServiceUpdateInvalid(t *testing.T) {
	tests := []struct {
		name  string
		id    uuid.UUID
		patch Patch
	}{
		{"nil id", uuid.Nil, Patch{}},
		{"blank name", uuid.New(), Patch{Name: strPtr(" ")}},
		{"blank email", uuid.New(), Patch{Email: strPtr("")}},
		{"no at sign", uuid.New(), Patch{Email: strPtr("nope")}},
		{"bad status", uuid.New(), Patch{Status: statusPtr("frozen")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
			_, err := svc.Update(context.Background(), tt.id, tt.patch)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Update = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceUpdateDuplicate(t *testing.T) {
	svc := NewService(fakeRepo{
		update: func(context.Context, uuid.UUID, *Patch) error {
			return ErrDuplicateEmail
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	_, err := svc.Update(context.Background(), uuid.New(), Patch{})
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("Update dup = %v, want ErrDuplicateEmail", err)
	}
}

func TestServiceDelete(t *testing.T) {
	id := uuid.New()
	svc := NewService(fakeRepo{
		delete: func(ctx context.Context, got uuid.UUID) error {
			if got != id {
				t.Fatalf("Delete id = %v, want %v", got, id)
			}
			return nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: unexpected error %v", err)
	}
}

func TestServiceDeleteNilID(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if err := svc.Delete(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Delete nil id = %v, want ErrInvalidInput", err)
	}
}

func TestServiceDeleteNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		delete: func(context.Context, uuid.UUID) error {
			return ErrNotFound
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
}

func TestServiceListFirstPage(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Member, error) {
			if q.Limit != paging.DefaultLimit+1 {
				t.Fatalf("Limit = %d, want %d", q.Limit, paging.DefaultLimit+1)
			}
			if q.AfterName != "" || q.AfterID != uuid.Nil {
				t.Fatalf("cursor start = %q/%v, want empty", q.AfterName, q.AfterID)
			}
			items := []*Member{}
			for i := 0; i < paging.DefaultLimit+1; i++ {
				items = append(items, &Member{ID: uuid.New(), Name: "B"})
			}
			return items, nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})

	res, err := svc.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	if len(res.Items) != paging.DefaultLimit {
		t.Fatalf("items = %d, want %d", len(res.Items), paging.DefaultLimit)
	}
	if res.NextCursor == "" {
		t.Fatal("NextCursor empty, want a value when a page is full")
	}
}

func TestServiceListLastPage(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(context.Context, *ListQuery) ([]*Member, error) {
			return []*Member{{ID: uuid.New(), Name: "Only"}}, nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})

	res, err := svc.List(context.Background(), ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(res.Items))
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want empty on the last page", res.NextCursor)
	}
}

func TestServiceListClampsLimit(t *testing.T) {
	for _, tt := range []struct {
		in   int
		want int
	}{
		{in: 0, want: paging.DefaultLimit},
		{in: -5, want: paging.DefaultLimit},
		{in: 1000, want: paging.MaxLimit},
	} {
		t.Run("", func(t *testing.T) {
			svc := NewService(fakeRepo{
				list: func(_ context.Context, q *ListQuery) ([]*Member, error) {
					if q.Limit != tt.want+1 {
						t.Fatalf("repo Limit = %d, want %d", q.Limit, tt.want+1)
					}
					return nil, nil
				},
			}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
			if _, err := svc.List(context.Background(), ListParams{Limit: tt.in}); err != nil {
				t.Fatalf("List: unexpected error %v", err)
			}
		})
	}
}

func TestServiceListCursorRoundTrip(t *testing.T) {
	const name = "Next"
	id := uuid.New()
	cursor := paging.Cursor{Key: name, ID: id}.Encode()

	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Member, error) {
			if q.AfterName != name {
				t.Fatalf("AfterName = %q, want %q", q.AfterName, name)
			}
			if q.AfterID != id {
				t.Fatalf("AfterID = %v, want %v", q.AfterID, id)
			}
			return nil, nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})

	if _, err := svc.List(context.Background(), ListParams{Cursor: cursor}); err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
}

func TestServiceListRejectsBadCursor(t *testing.T) {
	// "eyJuYW1lIjoibiJ9" is base64 for {"name":"n"} with no id.
	for _, cur := range []string{"%%%", "not-base64-!", "eyJuYW1lIjoibiJ9"} {
		t.Run(cur, func(t *testing.T) {
			svc := NewService(fakeRepo{
				list: func(context.Context, *ListQuery) ([]*Member, error) {
					t.Fatal("repo must not be called for a bad cursor")
					return nil, nil
				},
			}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
			_, err := svc.List(context.Background(), ListParams{Cursor: cur})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("List error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
func statusPtr(s string) *Status {
	v := Status(s)
	return &v
}

func TestServiceDeleteGuarded(t *testing.T) {
	for _, tt := range []struct {
		name    string
		checker fakeCheckers
	}{
		{"active membership", fakeCheckers{hasActive: func(context.Context, uuid.UUID) (bool, error) { return true, nil }}},
		{"pending invoice", fakeCheckers{hasPending: func(context.Context, uuid.UUID) (bool, error) { return true, nil }}},
		{"open visit", fakeCheckers{hasOpen: func(context.Context, uuid.UUID) (bool, error) { return true, nil }}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var deleted bool
			// Each checker position gets its own probe so a hit in one
			// position does not leak into the others.
			active := fakeCheckers{hasActive: tt.checker.hasActive}
			pending := fakeCheckers{hasPending: tt.checker.hasPending}
			open := fakeCheckers{hasOpen: tt.checker.hasOpen}
			svc := NewService(fakeRepo{
				delete: func(context.Context, uuid.UUID) error {
					deleted = true
					return nil
				},
			}, active, pending, open, stubTx{})
			if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrHasDependents) {
				t.Fatalf("Delete = %v, want ErrHasDependents", err)
			}
			if deleted {
				t.Fatal("repo.Delete called despite dependents")
			}
		})
	}
}

func TestServiceDeleteClean(t *testing.T) {
	var deleted bool
	svc := NewService(fakeRepo{
		delete: func(context.Context, uuid.UUID) error {
			deleted = true
			return nil
		},
	}, fakeCheckers{}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if err := svc.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Delete clean: %v", err)
	}
	if !deleted {
		t.Fatal("repo.Delete not called for a member without dependents")
	}
}

func TestServiceDeletePropagatesCheckerErrors(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeCheckers{
		hasActive: func(context.Context, uuid.UUID) (bool, error) { return false, errSentinel },
	}, fakeCheckers{}, fakeCheckers{}, stubTx{})
	if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, errSentinel) {
		t.Fatalf("Delete = %v, want checker error", err)
	}
}
