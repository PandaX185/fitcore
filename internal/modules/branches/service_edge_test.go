package branches

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
)

var errSentinel = errors.New("repo exploded")

func TestValidateCoordBoundaries(t *testing.T) {
	valid := []struct{ lat, lon float64 }{
		{0, 0},
		{90, 180},
		{-90, -180},
		{90, -180},
		{-90, 180},
		{51.5072, -0.1276},
		{90, 0},
		{0, 180},
	}
	for _, c := range valid {
		if err := validateCoord(c.lat, c.lon); err != nil {
			t.Fatalf("validateCoord(%v, %v) = %v, want nil", c.lat, c.lon, err)
		}
	}

	invalid := []struct{ lat, lon float64 }{
		{90.0001, 0},
		{-90.0001, 0},
		{0, 180.0001},
		{0, -180.0001},
		{math.Inf(1), 0},
		{math.Inf(-1), 0},
		{0, math.Inf(1)},
		{0, math.Inf(-1)},
		{math.NaN(), 0},
		{0, math.NaN()},
		{51.5, math.NaN()},
		{math.MaxFloat64, math.MaxFloat64},
	}
	for _, c := range invalid {
		if err := validateCoord(c.lat, c.lon); err == nil {
			t.Fatalf("validateCoord(%v, %v) = nil, want error", c.lat, c.lon)
		}
	}
}

func TestServiceCreateRejectsNaNAndInf(t *testing.T) {
	for _, c := range []struct{ lat, lon float64 }{
		{math.NaN(), 0},
		{0, math.NaN()},
		{math.Inf(1), 0},
		{0, math.Inf(-1)},
		{-math.Inf(1), 0},
	} {
		svc := NewService(fakeRepo{
			create: func(context.Context, *Branch) error {
				t.Fatal("repo must not be called for invalid coordinates")
				return nil
			},
		})
		if _, err := svc.Create(context.Background(), "X", "addr", c.lat, c.lon); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Create(lat=%v, lon=%v) error = %v, want ErrInvalid", c.lat, c.lon, err)
		}
	}
}

func TestServiceUpdateRejectsNaNAndInf(t *testing.T) {
	for _, c := range []struct{ lat, lon float64 }{
		{math.NaN(), 0},
		{0, math.NaN()},
		{math.Inf(1), 0},
		{0, math.Inf(-1)},
	} {
		svc := NewService(fakeRepo{
			update: func(context.Context, uuid.UUID, *Patch) error {
				t.Fatal("repo must not be called for invalid coordinates")
				return nil
			},
		})
		_, err := svc.Update(context.Background(), uuid.New(), Patch{Latitude: floatPtr(c.lat), Longitude: floatPtr(c.lon)})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Update(lat=%v, lon=%v) error = %v, want ErrInvalid", c.lat, c.lon, err)
		}
	}
}

func TestServiceCreateAcceptsBoundaryCoords(t *testing.T) {
	for _, c := range []struct{ lat, lon float64 }{
		{90, 180},
		{-90, -180},
	} {
		var created *Branch
		svc := NewService(fakeRepo{
			create: func(_ context.Context, b *Branch) error {
				created = b
				return nil
			},
		})
		if _, err := svc.Create(context.Background(), "X", "addr", c.lat, c.lon); err != nil {
			t.Fatalf("Create(%v, %v): %v", c.lat, c.lon, err)
		}
		if created.Latitude != c.lat || created.Longitude != c.lon {
			t.Fatalf("stored = (%v, %v)", created.Latitude, created.Longitude)
		}
	}
}

func TestServiceGetPropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Branch, error) {
			return nil, errSentinel
		},
	})
	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, errSentinel) {
		t.Fatalf("Get error = %v, want %v", err, errSentinel)
	}
}

func TestServiceCreatePropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		create: func(context.Context, *Branch) error { return errSentinel },
	})
	if _, err := svc.Create(context.Background(), "X", "addr", 0, 0); !errors.Is(err, errSentinel) {
		t.Fatalf("Create error = %v, want %v", err, errSentinel)
	}
}

func TestServiceUpdatePropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		update: func(context.Context, uuid.UUID, *Patch) error {
			return errSentinel
		},
	})
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{Name: strPtr("X")}); !errors.Is(err, errSentinel) {
		t.Fatalf("Update error = %v, want %v", err, errSentinel)
	}
}

func TestServiceUpdatePropagatesRefetchErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		update: func(context.Context, uuid.UUID, *Patch) error { return nil },
		getByID: func(context.Context, uuid.UUID) (*Branch, error) {
			return nil, errSentinel
		},
	})
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{Name: strPtr("X")}); !errors.Is(err, errSentinel) {
		t.Fatalf("Update error = %v, want %v", err, errSentinel)
	}
}

func TestServiceListPropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(context.Context, *ListQuery) ([]*Branch, error) {
			return nil, errSentinel
		},
	})
	if _, err := svc.List(context.Background(), ListParams{}); !errors.Is(err, errSentinel) {
		t.Fatalf("List error = %v, want %v", err, errSentinel)
	}
}

func TestServiceListExactLimitNoNextCursor(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Branch, error) {
			// q.Limit is limit+1; return exactly the requested page size.
			items := make([]*Branch, q.Limit-1)
			for i := range items {
				items[i] = &Branch{ID: uuid.New(), Name: "B"}
			}
			return items, nil
		},
	})
	res, err := svc.List(context.Background(), ListParams{Limit: 3})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(res.Items))
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want empty when repo returns exactly the limit", res.NextCursor)
	}
}

func TestServiceListEmptyResult(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(context.Context, *ListQuery) ([]*Branch, error) {
			return nil, nil
		},
	})
	res, err := svc.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if res == nil || len(res.Items) != 0 || res.NextCursor != "" {
		t.Fatalf("List = %+v, want empty page", res)
	}
}

func TestServiceListTrimsQuery(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(_ context.Context, q *ListQuery) ([]*Branch, error) {
			if q.Query != "Central" {
				t.Fatalf("Query = %q, want trimmed", q.Query)
			}
			return nil, nil
		},
	})
	if _, err := svc.List(context.Background(), ListParams{Query: "  Central  "}); err != nil {
		t.Fatalf("List: %v", err)
	}
}
