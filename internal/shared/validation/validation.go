// Package validation checks request structs declared with `validate` tags and turns the
// failures into problem.FieldError values named after the struct's JSON fields.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/takemerightttt/cart-service/internal/shared/money"
	"github.com/takemerightttt/cart-service/internal/shared/problem"
)

// Error is returned by Struct when one or more fields are invalid.
type Error struct {
	Fields []problem.FieldError
}

func (e *Error) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Validator wraps a configured go-playground validator. It is safe for concurrent use.
type Validator struct {
	v *validator.Validate
}

// New returns a Validator that reports JSON field names and knows the "currency" tag
// (an ISO 4217 alphabetic code, see money.ValidCurrency).
func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	_ = v.RegisterValidation("currency", func(fl validator.FieldLevel) bool {
		return money.ValidCurrency(fl.Field().String())
	})
	return &Validator{v: v}
}

// Struct validates s. It returns *Error for invalid input and nil when s is valid.
func (val *Validator) Struct(s any) error {
	err := val.v.Struct(s)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return fmt.Errorf("validation: %w", err)
	}
	out := &Error{Fields: make([]problem.FieldError, 0, len(verrs))}
	for _, fe := range verrs {
		out.Fields = append(out.Fields, problem.FieldError{Field: fieldPath(fe), Message: message(fe)})
	}
	return out
}

func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if _, rest, ok := strings.Cut(ns, "."); ok {
		return rest
	}
	return fe.Field()
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		if fe.Kind() == reflect.String {
			return "must be at least " + fe.Param() + " characters"
		}
		return "must be at least " + fe.Param()
	case "max":
		if fe.Kind() == reflect.String {
			return "must be at most " + fe.Param() + " characters"
		}
		return "must be at most " + fe.Param()
	case "gt":
		return "must be greater than " + fe.Param()
	case "gte":
		return "must be at least " + fe.Param()
	case "lte":
		return "must be at most " + fe.Param()
	case "uuid", "uuid4":
		return "must be a valid UUID"
	case "currency":
		return "must be an ISO 4217 code such as USD"
	case "printascii":
		return "must contain printable ASCII characters only"
	default:
		return "is invalid (" + fe.Tag() + ")"
	}
}
