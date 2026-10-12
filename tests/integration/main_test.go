package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/takemerightttt/cart-service/internal/app"
	"github.com/takemerightttt/cart-service/internal/postgres"
)

var (
	server *httptest.Server
	pool   *pgxpool.Pool
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("cart"),
		tcpostgres.WithUsername("cart"),
		tcpostgres.WithPassword("cart"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dsn:", err)
		return 1
	}
	pool, err = postgres.Connect(ctx, dsn, 20)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		return 1
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}

	log := app.NewLogger(slog.NewTextHandler(io.Discard, nil))
	server = httptest.NewServer(app.NewHandler(pool, log, app.Options{IdempotencyTTL: time.Hour}))
	defer server.Close()
	return m.Run()
}

type client struct {
	t      *testing.T
	userID uuid.UUID
}

func newClient(t *testing.T) *client {
	t.Helper()
	return &client{t: t, userID: uuid.New()}
}

type response struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

func (c *client) do(method, path string, body any, headers ...string) response {
	c.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			c.t.Fatalf("marshal: %v", err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, server.URL+path, rd)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userID != uuid.Nil {
		req.Header.Set("X-User-ID", c.userID.String())
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read body: %v", err)
	}
	return response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

func expectStatus(t *testing.T, r response, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, want, r.Body)
	}
}
