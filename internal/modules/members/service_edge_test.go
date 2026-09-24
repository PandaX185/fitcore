package members

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

var errSentinel = errors.New("repo exploded")

func TestServiceCreatePropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		create: func(context.Context, *Member) error { return errSentinel },
	})
	if _, err := svc.Create(context.Background(), uuid.New(), "Ada", "a@b.com", ""); !errors.Is(err, errSentinel) {
		t.Fatalf("Create error = %v, want %v", err, errSentinel)
	}
}

func TestServiceCreateKeepsNameAndPhone(t *testing.T) {
	var created *Member
	svc := NewService(fakeRepo{
		create: func(_ context.Context, m *Member) error {
			created = m
			return nil
		},
	})
	if _, err := svc.Create(context.Background(), uuid.New(), "  Ada Lovelace  ", "a@b.com", "+1 555 0100"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Only the email is normalized; the human-entered name and phone pass
	// through untouched.
	if created.Name != "  Ada Lovelace  " {
		t.Fatalf("name = %q, want passed through unchanged", created.Name)
	}
	if created.Phone != "+1 555 0100" {
		t.Fatalf("phone = %q", created.Phone)
	}
	if created.Email != "a@b.com" {
		t.Fatalf("email = %q", created.Email)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not set: %+v", created)
	}
}

func TestServiceCreateRequiresAtSignVariants(t *testing.T) {
	branchID := uuid.New()
	// The domain contract is deliberately light: non-blank and contains '@'.
	// Strict RFC parsing happens at the API boundary.
	for _, email := range []string{"no-at-sign", "a", "", "   ", "\t\n"} {
		svc := NewService(fakeRepo{})
		if _, err := svc.Create(context.Background(), branchID, "Ada", email, ""); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Create(email=%q) = %v, want ErrInvalidInput", email, err)
		}
	}
	// Loose but accepted by design; the stored value is still normalized.
	for _, email := range []string{"@", "a@", "@b", "a@b@c", "A@B"} {
		svc := NewService(fakeRepo{})
		if _, err := svc.Create(context.Background(), branchID, "Ada", email, ""); err != nil {
			t.Fatalf("Create(email=%q) = %v, want accepted", email, err)
		}
	}
}

func TestServiceGetPropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		getByID: func(context.Context, uuid.UUID) (*Member, error) {
			return nil, errSentinel
		},
	})
	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, errSentinel) {
		t.Fatalf("Get error = %v, want %v", err, errSentinel)
	}
}

func TestServiceUpdateNormalizesEmail(t *testing.T) {
	id := uuid.New()
	var seen string
	svc := NewService(fakeRepo{
		update: func(_ context.Context, got uuid.UUID, patch *Patch) error {
			if patch.Email == nil {
				t.Fatal("patch email nil")
			}
			seen = *patch.Email
			return nil
		},
		getByID: func(_ context.Context, id uuid.UUID) (*Member, error) { return &Member{ID: id}, nil },
	})
	email := "  Ada@Example.COM  "
	if _, err := svc.Update(context.Background(), id, Patch{Email: &email}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if seen != "ada@example.com" {
		t.Fatalf("repo email = %q, want normalized", seen)
	}
}

func TestServiceUpdateRejectsWhitespaceEmail(t *testing.T) {
	svc := NewService(fakeRepo{})
	for _, email := range []string{"", "   ", "\t\n"} {
		patch := Patch{Email: &email}
		if _, err := svc.Update(context.Background(), uuid.New(), patch); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Update(email=%q) = %v, want ErrInvalidInput", email, err)
		}
	}
}

func TestServiceUpdatePropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		update: func(context.Context, uuid.UUID, *Patch) error { return errSentinel },
	})
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{}); !errors.Is(err, errSentinel) {
		t.Fatalf("Update error = %v, want %v", err, errSentinel)
	}
}

func TestServiceUpdatePropagatesRefetchErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		update:  func(context.Context, uuid.UUID, *Patch) error { return nil },
		getByID: func(context.Context, uuid.UUID) (*Member, error) { return nil, errSentinel },
	})
	if _, err := svc.Update(context.Background(), uuid.New(), Patch{}); !errors.Is(err, errSentinel) {
		t.Fatalf("Update error = %v, want %v", err, errSentinel)
	}
}

func TestServiceDeletePropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		delete: func(context.Context, uuid.UUID) error { return errSentinel },
	})
	if err := svc.Delete(context.Background(), uuid.New()); !errors.Is(err, errSentinel) {
		t.Fatalf("Delete error = %v, want %v", err, errSentinel)
	}
}

func TestServiceListEmpty(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(context.Context) ([]*Member, error) { return nil, nil },
	})
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %d items, want 0", len(got))
	}
}

func TestServiceListPropagatesRepoErrors(t *testing.T) {
	svc := NewService(fakeRepo{
		list: func(context.Context) ([]*Member, error) { return nil, errSentinel },
	})
	if _, err := svc.List(context.Background()); !errors.Is(err, errSentinel) {
		t.Fatalf("List error = %v, want %v", err, errSentinel)
	}
}
