package branches

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PandaX185/fitcore/internal/paging"
)

type fakeRepo struct {
	create  func(ctx context.Context, b *Branch) error
	getByID func(ctx context.Context, id uuid.UUID) (*Branch, error)
	list    func(ctx context.Context, q *ListQuery) ([]*Branch, error)
	update  func(ctx context.Context, id uuid.UUID, patch *Patch) error
}

func (f fakeRepo) Create(ctx context.Context, b *Branch) error {
	if f.create == nil {
		return nil
	}
	return f.create(ctx, b)
}
func (f fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*Branch, error) {
	return f.getByID(ctx, id)
}
func (f fakeRepo) List(ctx context.Context, q *ListQuery) ([]*Branch, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, q)
}
func (f fakeRepo) Update(ctx context.Context, id uuid.UUID, patch *Patch) error {
	if f.update == nil {
		return nil
	}
	return f.update(ctx, id, patch)
}

func TestServiceGet(t *testing.T) {
	id := uuid.New()
	sentinel := &Branch{ID: id, Name: "Central"}
	svc := NewService(fakeRepo{
		getByID: func(ctx context.Context, got uuid.UUID) (*Branch, error) {
			if got != id {
				t.Fatalf("GetByID id = %v, want %v", got, id)
			}
			return sentinel, nil
		},
	})

	got, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: unexpected error %v", err)
	}
	if got != sentinel {
		t.Fatalf("Get = %v, want %v", got, sentinel)
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Branch, error) {
			return nil, ErrNotFound
		},
	})

	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestServiceGetInvalidID(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Branch, error) {
			t.Fatal("GetByID must not be called for a nil id")
			return nil, nil
		},
	})

	_, err := svc.Get(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Get error = %v, want ErrInvalid", err)
	}
}

func TestServiceCreate(t *testing.T) {
	var created *Branch
	svc := NewService(fakeRepo{
		create: func(_ context.Context, b *Branch) error {
			created = b
			return nil
		},
	})

	got, err := svc.Create(context.Background(), "Central", "1 Main St", 51.5, -0.1)
	if err != nil {
		t.Fatalf("Create: unexpected error %v", err)
	}
	if got != created {
		t.Fatalf("Create returned %v, want the persisted branch", got)
	}
	if created.ID == uuid.Nil {
		t.Fatal("Create did not assign an id")
	}
	if created.Name != "Central" || created.Address != "1 Main St" || created.Latitude != 51.5 || created.Longitude != -0.1 {
		t.Fatalf("Create branch = %+v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("Create timestamps not set: %+v", created)
	}
}

func TestServiceCreateInvalid(t *testing.T) {
	tests := []struct {
		name      string
		latitude  float64
		longitude float64
	}{
		{name: ""},
		{name: "   "},
		{name: "Central", latitude: 91, longitude: 0},
		{name: "Central", latitude: 0, longitude: -181},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{
				create: func(context.Context, *Branch) error {
					t.Fatal("repo must not be called for invalid input")
					return nil
				},
			})
			_, err := svc.Create(context.Background(), tt.name, "addr", tt.latitude, tt.longitude)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create(%q, %v, %v) error = %v, want ErrInvalid", tt.name, tt.latitude, tt.longitude, err)
			}
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	id := uuid.New()
	sentinel := &Branch{ID: id, Name: "Renamed"}
	svc := NewService(fakeRepo{
		update: func(_ context.Context, got uuid.UUID, patch *Patch) error {
			if got != id {
				t.Fatalf("Update id = %v, want %v", got, id)
			}
			if patch.Name == nil || *patch.Name != "Renamed" {
				t.Fatalf("Update patch = %+v", patch)
			}
			if patch.Address != nil {
				t.Fatalf("Update patch included address %q, want nil", *patch.Address)
			}
			return nil
		},
		getByID: func(context.Context, uuid.UUID) (*Branch, error) { return sentinel, nil },
	})

	got, err := svc.Update(context.Background(), id, Patch{Name: strPtr("Renamed")})
	if err != nil {
		t.Fatalf("Update: unexpected error %v", err)
	}
	if got != sentinel {
		t.Fatalf("Update = %v, want %v", got, sentinel)
	}
}

func TestServiceUpdateNotFound(t *testing.T) {
	svc := NewService(fakeRepo{
		update: func(context.Context, uuid.UUID, *Patch) error { return ErrNotFound },
	})

	_, err := svc.Update(context.Background(), uuid.New(), Patch{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update error = %v, want ErrNotFound", err)
	}
}

func TestServiceUpdateInvalid(t *testing.T) {
	tests := []struct {
		name  string
		id    uuid.UUID
		patch Patch
	}{
		{name: "nil id"},
		{name: "empty name", id: uuid.New(), patch: Patch{Name: strPtr(" ")}},
		{name: "latitude only", id: uuid.New(), patch: Patch{Latitude: floatPtr(51.5)}},
		{name: "longitude only", id: uuid.New(), patch: Patch{Longitude: floatPtr(-0.1)}},
		{name: "latitude out of range", id: uuid.New(), patch: Patch{Latitude: floatPtr(91), Longitude: floatPtr(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeRepo{
				update: func(context.Context, uuid.UUID, *Patch) error {
					t.Fatal("repo must not be called for invalid input")
					return nil
				},
			})
			_, err := svc.Update(context.Background(), tt.id, tt.patch)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Update error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestServiceListFirstPage(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Branch, error) {
			if q.Limit != paging.DefaultLimit+1 {
				t.Fatalf("Limit = %d, want %d", q.Limit, paging.DefaultLimit+1)
			}
			if q.AfterName != "" || q.AfterID != uuid.Nil {
				t.Fatalf("cursor start = %q/%v, want empty", q.AfterName, q.AfterID)
			}
			items := []*Branch{}
			for i := 0; i < paging.DefaultLimit+1; i++ {
				items = append(items, &Branch{ID: uuid.New(), Name: "B"})
			}
			return items, nil
		},
	})

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
		list: func(context.Context, *ListQuery) ([]*Branch, error) {
			return []*Branch{{ID: uuid.New(), Name: "Only"}}, nil
		},
	})

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
				list: func(_ context.Context, q *ListQuery) ([]*Branch, error) {
					if q.Limit != tt.want+1 {
						t.Fatalf("repo Limit = %d, want %d", q.Limit, tt.want+1)
					}
					return nil, nil
				},
			})
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
		list: func(_ context.Context, q *ListQuery) ([]*Branch, error) {
			if q.AfterName != name {
				t.Fatalf("AfterName = %q, want %q", q.AfterName, name)
			}
			if q.AfterID != id {
				t.Fatalf("AfterID = %v, want %v", q.AfterID, id)
			}
			return nil, nil
		},
	})

	if _, err := svc.List(context.Background(), ListParams{Cursor: cursor}); err != nil {
		t.Fatalf("List: unexpected error %v", err)
	}
}

func TestServiceListRejectsBadCursor(t *testing.T) {
	// "eyJuYW1lIjoibiJ9" is base64 for {"name":"n"} with no id.
	for _, cur := range []string{"%%%", "not-base64-!", "eyJuYW1lIjoibiJ9"} {
		t.Run(cur, func(t *testing.T) {
			svc := NewService(fakeRepo{
				list: func(context.Context, *ListQuery) ([]*Branch, error) {
					t.Fatal("repo must not be called for a bad cursor")
					return nil, nil
				},
			})
			_, err := svc.List(context.Background(), ListParams{Cursor: cur})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("List error = %v, want ErrInvalid", err)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
func floatPtr(f float64) *float64 {
	return &f
}
