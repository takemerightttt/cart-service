package integration

import (
	"net/http"
	"testing"
)

func TestHealthAndReadiness(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	expectStatus(t, c.do(http.MethodGet, "/healthz", nil), http.StatusOK)
	expectStatus(t, c.do(http.MethodGet, "/readyz", nil), http.StatusOK)
}

func TestRequestIDAndUnknownRoute(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	r := c.do(http.MethodGet, "/nope", nil, "X-Request-ID", "trace-123")
	expectStatus(t, r, http.StatusNotFound)
	if r.Header.Get("X-Request-ID") != "trace-123" {
		t.Fatalf("request id = %q", r.Header.Get("X-Request-ID"))
	}
	problemOf(t, r)
	if generated := c.do(http.MethodGet, "/healthz", nil).Header.Get("X-Request-ID"); generated == "" {
		t.Fatalf("no request id generated")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	r := c.do(http.MethodPut, "/v1/products", nil)
	expectStatus(t, r, http.StatusMethodNotAllowed)
	if r.Header.Get("Allow") == "" {
		t.Fatalf("no Allow header")
	}
	problemOf(t, r)
}
