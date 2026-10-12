// Package problem writes HTTP error responses as RFC 9457 "problem details"
// (media type application/problem+json), the single error envelope of the API.
package problem

import (
	"encoding/json"
	"net/http"
)

// ContentType is the media type of every problem response.
const ContentType = "application/problem+json"

// FieldError describes one invalid input field. Field is the JSON name of the field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem is an RFC 9457 problem details object. Errors is an extension member that lists
// field-level validation failures.
type Problem struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Instance string       `json:"instance,omitempty"`
	Errors   []FieldError `json:"errors,omitempty"`
}

// New builds a problem with type "about:blank" and the standard reason phrase as its title.
func New(status int, detail string) Problem {
	return Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail}
}

// Validation builds a 422 problem that carries the given field errors.
func Validation(errs []FieldError) Problem {
	p := New(http.StatusUnprocessableEntity, "request validation failed")
	p.Errors = errs
	return p
}

// Write sends p with its status code. Instance defaults to the request path when empty.
func Write(w http.ResponseWriter, r *http.Request, p Problem) {
	if p.Instance == "" && r != nil {
		p.Instance = r.URL.Path
	}
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
