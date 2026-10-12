package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

const UserHeader = "X-User-ID"

var ErrUnauthenticated = errors.New("missing or invalid " + UserHeader + " header")

type ctxKey struct{}

func WithUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

func Require(onErr func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(r.Header.Get(UserHeader))
			if err != nil || id == uuid.Nil {
				onErr(w, r, ErrUnauthenticated)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), id)))
		})
	}
}
