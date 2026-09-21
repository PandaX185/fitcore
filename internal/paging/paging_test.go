package paging

import (
	"testing"

	"github.com/google/uuid"
)

func TestLimit(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: DefaultLimit},
		{in: -5, want: DefaultLimit},
		{in: 1, want: 1},
		{in: DefaultLimit, want: DefaultLimit},
		{in: MaxLimit, want: MaxLimit},
		{in: 5000, want: MaxLimit},
	}
	for _, tt := range tests {
		if got := Limit(tt.in); got != tt.want {
			t.Fatalf("Limit(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestMaybeLimit(t *testing.T) {
	if got := MaybeLimit(nil); got != DefaultLimit {
		t.Fatalf("MaybeLimit(nil) = %d, want %d", got, DefaultLimit)
	}
	n := 5
	if got := MaybeLimit(&n); got != 5 {
		t.Fatalf("MaybeLimit(&5) = %d, want 5", got)
	}
	huge := 5000
	if got := MaybeLimit(&huge); got != MaxLimit {
		t.Fatalf("MaybeLimit(&5000) = %d, want %d", got, MaxLimit)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	c := Cursor{Key: "Alpha Gym", ID: uuid.New()}
	got, err := DecodeCursor(c.Encode())
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if got != c {
		t.Fatalf("round trip = %+v, want %+v", got, c)
	}
}

func TestDecodeCursorEmpty(t *testing.T) {
	got, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("DecodeCursor(\"\"): unexpected error %v", err)
	}
	if got != (Cursor{}) {
		t.Fatalf("DecodeCursor(\"\") = %+v, want zero cursor", got)
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	// "eyJrZXkiOiJuIn0" is base64 for {"key":"n"} with no id.
	for _, s := range []string{"%%%", "not-base64-!", "eyJrZXkiOiJuIn0"} {
		t.Run(s, func(t *testing.T) {
			if _, err := DecodeCursor(s); err == nil {
				t.Fatalf("DecodeCursor(%q) expected an error", s)
			}
		})
	}
}
