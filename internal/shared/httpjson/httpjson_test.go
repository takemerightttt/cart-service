package httpjson_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takemerightttt/cart-service/internal/shared/httpjson"
)

type payload struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

func request(body, contentType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func TestDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        string
		contentType string
		wantErr     error
		wantSyntax  string
	}{
		{name: "valid", body: `{"name":"a","quantity":2}`, contentType: "application/json"},
		{name: "charset param", body: `{"name":"a"}`, contentType: "application/json; charset=utf-8"},
		{name: "no content type", body: `{"name":"a"}`},
		{name: "wrong content type", body: `{}`, contentType: "text/plain", wantErr: httpjson.ErrUnsupportedMediaType},
		{name: "empty", body: ``, contentType: "application/json", wantSyntax: "body must not be empty"},
		{name: "malformed", body: `{"name":`, contentType: "application/json", wantSyntax: "body contains malformed JSON"},
		{name: "bad syntax", body: `{"name" "a"}`, contentType: "application/json", wantSyntax: "body contains malformed JSON"},
		{name: "unknown field", body: `{"nope":1}`, contentType: "application/json", wantSyntax: `unknown field "nope"`},
		{name: "wrong type", body: `{"quantity":"two"}`, contentType: "application/json", wantSyntax: `field "quantity" has the wrong type`},
		{name: "two objects", body: `{"name":"a"}{"name":"b"}`, contentType: "application/json", wantSyntax: "body must contain a single JSON object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var dst payload
			err := httpjson.Decode(httptest.NewRecorder(), request(tt.body, tt.contentType), &dst)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
			case tt.wantSyntax != "":
				var se *httpjson.SyntaxError
				if !errors.As(err, &se) || se.Msg != tt.wantSyntax {
					t.Fatalf("error = %v, want syntax error %q", err, tt.wantSyntax)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDecodeLimit(t *testing.T) {
	t.Parallel()
	body := `{"name":"` + strings.Repeat("x", 100) + `"}`
	var dst payload
	err := httpjson.DecodeLimit(httptest.NewRecorder(), request(body, "application/json"), &dst, 32)
	if !errors.Is(err, httpjson.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpjson.Write(rec, http.StatusCreated, payload{Name: "a", Quantity: 1})
	if rec.Code != http.StatusCreated || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"name":"a","quantity":1}` {
		t.Fatalf("body = %s", got)
	}
}

func TestWriteNoBody(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpjson.Write(rec, http.StatusNoContent, nil)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestWriteRaw(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		wantCT string
	}{
		{http.StatusOK, "application/json"},
		{http.StatusCreated, "application/json"},
		{http.StatusConflict, "application/problem+json"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		httpjson.WriteRaw(rec, tt.status, []byte(`{"a":1}`))
		if rec.Code != tt.status || rec.Header().Get("Content-Type") != tt.wantCT || rec.Body.String() != `{"a":1}` {
			t.Errorf("WriteRaw(%d): status %d, content type %q, body %q", tt.status, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
}
