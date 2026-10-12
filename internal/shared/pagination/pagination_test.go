package pagination_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/takemerightttt/cart-service/internal/shared/pagination"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()
	c := pagination.Cursor{CreatedAt: time.Date(2026, 10, 12, 8, 30, 0, 123456000, time.UTC), ID: uuid.New()}
	got, err := pagination.Decode(c.Encode())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) || got.ID != c.ID {
		t.Fatalf("round trip = %+v, want %+v", got, c)
	}
}

func TestDecode(t *testing.T) {
	t.Parallel()
	if c, err := pagination.Decode(""); c != nil || err != nil {
		t.Fatalf("empty cursor = %v, %v", c, err)
	}
	bad := []string{"%%%", "bm90LWpzb24", "e30", pagination.Cursor{ID: uuid.New()}.Encode()}
	for _, s := range bad {
		if _, err := pagination.Decode(s); !errors.Is(err, pagination.ErrInvalidCursor) {
			t.Errorf("Decode(%q) error = %v, want ErrInvalidCursor", s, err)
		}
	}
}

func TestParseLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", 20, false},
		{"5", 5, false},
		{"100", 100, false},
		{"500", 100, false},
		{"0", 0, true},
		{"-3", 0, true},
		{"ten", 0, true},
	}
	for _, tt := range tests {
		got, err := pagination.ParseLimit(tt.in, 20, 100)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseLimit(%q) = %d, %v; want %d, err %v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestNewPage(t *testing.T) {
	t.Parallel()
	key := func(n int) pagination.Cursor {
		return pagination.Cursor{CreatedAt: time.Unix(int64(n), 0).UTC(), ID: uuid.NewSHA1(uuid.Nil, []byte{byte(n)})}
	}
	last := pagination.NewPage([]int{1, 2}, 2, key)
	if len(last.Items) != 2 || last.NextCursor != "" {
		t.Fatalf("last page = %+v", last)
	}
	more := pagination.NewPage([]int{1, 2, 3}, 2, key)
	if len(more.Items) != 2 || more.NextCursor != key(2).Encode() {
		t.Fatalf("page with more = %+v", more)
	}
	empty := pagination.NewPage[int](nil, 2, key)
	if empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("empty page items = %#v, want empty slice", empty.Items)
	}
}
