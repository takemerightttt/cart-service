package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/takemerightttt/cart-service/internal/auth"
	"github.com/takemerightttt/cart-service/internal/shared/httpjson"
)

const (
	Header         = "Idempotency-Key"
	ReplayedHeader = "Idempotent-Replayed"
	maxKeyLength   = 255
)

var ErrInvalidKey = errors.New("Idempotency-Key must be 1-255 printable ASCII characters")

type ErrorWriter func(w http.ResponseWriter, r *http.Request, err error)

func Middleware(store *Store, writeErr ErrorWriter, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(Header)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !validKey(key) {
				writeErr(w, r, ErrInvalidKey)
				return
			}
			userID, _ := auth.UserID(r.Context())
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpjson.DefaultMaxBytes))
			if err != nil {
				writeErr(w, r, httpjson.ErrTooLarge)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			sum := sha256.New()
			sum.Write([]byte(r.Method + " " + r.URL.Path + "\n"))
			sum.Write(body)
			stored, err := store.Begin(r.Context(), userID, key, sum.Sum(nil))
			if err != nil {
				writeErr(w, r, err)
				return
			}
			if stored != nil {
				w.Header().Set(ReplayedHeader, "true")
				httpjson.WriteRaw(w, stored.Status, stored.Body)
				return
			}

			rec := &recorder{header: http.Header{}, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			ctx := context.WithoutCancel(r.Context())
			if rec.status >= http.StatusInternalServerError {
				if err := store.Release(ctx, userID, key); err != nil {
					log.ErrorContext(ctx, "release idempotency key", "error", err)
				}
			} else if err := store.Complete(ctx, userID, key, Response{Status: rec.status, Body: rec.body.Bytes()}); err != nil {
				log.ErrorContext(ctx, "complete idempotency key", "error", err)
			}
			for k, v := range rec.header {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.status)
			_, _ = w.Write(rec.body.Bytes())
		})
	}
}

func validKey(k string) bool {
	if len(k) > maxKeyLength {
		return false
	}
	for i := 0; i < len(k); i++ {
		if k[i] < 0x21 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

type recorder struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status, r.wroteHeader = status, true
}

func (r *recorder) Write(b []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	return r.body.Write(b)
}
