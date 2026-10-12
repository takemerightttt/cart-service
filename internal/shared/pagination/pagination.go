// Package pagination implements keyset (cursor) pagination over a (created_at, id) ordering.
// Cursors are opaque to clients: base64url-encoded JSON.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidCursor is returned when a cursor cannot be decoded.
var ErrInvalidCursor = errors.New("invalid cursor")

// ErrInvalidLimit is returned when a limit is not a positive integer.
var ErrInvalidLimit = errors.New("invalid limit")

// Cursor points just past the last row of a page in (CreatedAt DESC, ID DESC) order.
type Cursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"id"`
}

// Encode returns the opaque string form of c.
func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Decode parses a cursor produced by Encode. An empty string yields (nil, nil): the first page.
func Decode(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == uuid.Nil || c.CreatedAt.IsZero() {
		return nil, ErrInvalidCursor
	}
	return &c, nil
}

// ParseLimit parses a page size. Empty means def; values above max are clamped to max.
func ParseLimit(s string, def, max int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, ErrInvalidLimit
	}
	return min(n, max), nil
}

// Page is one page of items plus the cursor of the next page; NextCursor is empty on the last page.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// NewPage builds a page from up to limit+1 fetched rows: the extra row only signals that another
// page exists. key extracts the cursor of a row.
func NewPage[T any](rows []T, limit int, key func(T) Cursor) Page[T] {
	if len(rows) <= limit {
		if rows == nil {
			rows = []T{}
		}
		return Page[T]{Items: rows}
	}
	rows = rows[:limit]
	return Page[T]{Items: rows, NextCursor: key(rows[len(rows)-1]).Encode()}
}
