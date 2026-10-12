package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/takemerightttt/cart-service/internal/auth"
	"github.com/takemerightttt/cart-service/internal/cart"
	"github.com/takemerightttt/cart-service/internal/idempotency"
	"github.com/takemerightttt/cart-service/internal/product"
	"github.com/takemerightttt/cart-service/internal/shared/httpjson"
	"github.com/takemerightttt/cart-service/internal/shared/problem"
	"github.com/takemerightttt/cart-service/internal/shared/validation"
)

type Options struct {
	IdempotencyTTL time.Duration
}

func NewHandler(pool *pgxpool.Pool, log *slog.Logger, opts Options) http.Handler {
	writeErr := errorWriter(log)
	v := validation.New()
	mux := http.NewServeMux()

	product.NewHandler(product.NewService(pool, v), writeErr).Register(mux)

	idem := idempotency.Middleware(idempotency.NewStore(pool, opts.IdempotencyTTL), writeErr, log)
	cart.NewHandler(cart.NewService(pool, v), writeErr, idem).Register(mux, auth.Require(writeErr))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			problem.Write(w, r, problem.New(http.StatusServiceUnavailable, "database is unavailable"))
			return
		}
		httpjson.Write(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	return withRequestID(withLogging(log)(withRecover(log)(problemFallback(mux))))
}

func problemFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		if allow := rec.Header().Get("Allow"); allow != "" {
			w.Header().Set("Allow", allow)
		}
		detail := "route not found"
		if rec.Code == http.StatusMethodNotAllowed {
			detail = "method not allowed"
		}
		problem.Write(w, r, problem.New(rec.Code, detail))
	})
}

func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}
