package problem_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takemerightttt/cart-service/internal/shared/problem"
)

func TestWrite(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/carts/42", nil)

	problem.Write(rec, req, problem.New(http.StatusNotFound, "cart not found"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("content type = %q", ct)
	}
	var got problem.Problem
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := problem.Problem{Type: "about:blank", Title: "Not Found", Status: 404, Detail: "cart not found", Instance: "/v1/carts/42"}
	if got.Type != want.Type || got.Title != want.Title || got.Status != want.Status || got.Detail != want.Detail || got.Instance != want.Instance {
		t.Fatalf("problem = %+v, want %+v", got, want)
	}
}

func TestValidation(t *testing.T) {
	t.Parallel()
	errs := []problem.FieldError{{Field: "quantity", Message: "must be at least 1"}}
	p := problem.Validation(errs)
	if p.Status != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0] != errs[0] {
		t.Fatalf("Validation = %+v", p)
	}
}

func TestWriteKeepsExplicitInstance(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	p := problem.New(http.StatusConflict, "x")
	p.Instance = "/custom"
	problem.Write(rec, httptest.NewRequest(http.MethodPost, "/v1/x", nil), p)
	var got problem.Problem
	_ = json.NewDecoder(rec.Body).Decode(&got)
	if got.Instance != "/custom" {
		t.Fatalf("instance = %q", got.Instance)
	}
}
