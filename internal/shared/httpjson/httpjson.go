// Package httpjson reads strict JSON request bodies and writes JSON responses.
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// DefaultMaxBytes is the request body limit used by Decode.
const DefaultMaxBytes = 1 << 20

// ErrUnsupportedMediaType is returned when the request is not application/json.
var ErrUnsupportedMediaType = errors.New("content type must be application/json")

// ErrTooLarge is returned when the body exceeds the size limit.
var ErrTooLarge = errors.New("request body is too large")

// SyntaxError is returned for malformed JSON, unknown fields, wrong types, an empty body or
// trailing data after the JSON object.
type SyntaxError struct {
	Msg string
}

func (e *SyntaxError) Error() string { return e.Msg }

// Decode reads exactly one JSON value from r into dst. Unknown fields are rejected and the
// body is limited to DefaultMaxBytes.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	return DecodeLimit(w, r, dst, DefaultMaxBytes)
}

// DecodeLimit is Decode with an explicit body size limit in bytes.
func DecodeLimit(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil || mt != "application/json" {
			return ErrUnsupportedMediaType
		}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return classify(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return ErrTooLarge
		}
		return &SyntaxError{Msg: "body must contain a single JSON object"}
	}
	return nil
}

func classify(err error) error {
	var (
		syntax   *json.SyntaxError
		typeErr  *json.UnmarshalTypeError
		tooLarge *http.MaxBytesError
	)
	switch {
	case errors.As(err, &tooLarge):
		return ErrTooLarge
	case errors.Is(err, io.EOF):
		return &SyntaxError{Msg: "body must not be empty"}
	case errors.Is(err, io.ErrUnexpectedEOF), errors.As(err, &syntax):
		return &SyntaxError{Msg: "body contains malformed JSON"}
	case errors.As(err, &typeErr):
		return &SyntaxError{Msg: fmt.Sprintf("field %q has the wrong type", typeErr.Field)}
	default:
		const prefix = "json: unknown field "
		if msg := err.Error(); len(msg) > len(prefix) && msg[:len(prefix)] == prefix {
			return &SyntaxError{Msg: "unknown field " + msg[len(prefix):]}
		}
		return &SyntaxError{Msg: "body could not be decoded"}
	}
}

// Write sends v as JSON with the given status code.
func Write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteRaw sends an already encoded JSON body with the given status code. Problem bodies keep
// their application/problem+json media type.
func WriteRaw(w http.ResponseWriter, status int, body []byte) {
	ct := "application/json"
	if status >= http.StatusBadRequest {
		ct = "application/problem+json"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
