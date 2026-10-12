package app

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/takemerightttt/cart-service/internal/auth"
	"github.com/takemerightttt/cart-service/internal/cart"
	"github.com/takemerightttt/cart-service/internal/idempotency"
	"github.com/takemerightttt/cart-service/internal/product"
	"github.com/takemerightttt/cart-service/internal/shared/httpjson"
	"github.com/takemerightttt/cart-service/internal/shared/pagination"
	"github.com/takemerightttt/cart-service/internal/shared/problem"
	"github.com/takemerightttt/cart-service/internal/shared/validation"
)

var statusByError = []struct {
	err    error
	status int
}{
	{auth.ErrUnauthenticated, http.StatusUnauthorized},
	{product.ErrNotFound, http.StatusNotFound},
	{product.ErrSKUTaken, http.StatusConflict},
	{cart.ErrNotFound, http.StatusNotFound},
	{cart.ErrItemNotFound, http.StatusNotFound},
	{cart.ErrInsufficientStock, http.StatusConflict},
	{cart.ErrQuantityLimit, http.StatusUnprocessableEntity},
	{cart.ErrTooManyItems, http.StatusUnprocessableEntity},
	{cart.ErrCurrencyMismatch, http.StatusUnprocessableEntity},
	{idempotency.ErrInProgress, http.StatusConflict},
	{idempotency.ErrKeyReused, http.StatusUnprocessableEntity},
	{idempotency.ErrInvalidKey, http.StatusBadRequest},
	{pagination.ErrInvalidCursor, http.StatusBadRequest},
	{pagination.ErrInvalidLimit, http.StatusBadRequest},
	{httpjson.ErrUnsupportedMediaType, http.StatusUnsupportedMediaType},
	{httpjson.ErrTooLarge, http.StatusRequestEntityTooLarge},
}

func errorWriter(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var verr *validation.Error
		if errors.As(err, &verr) {
			problem.Write(w, r, problem.Validation(verr.Fields))
			return
		}
		var serr *httpjson.SyntaxError
		if errors.As(err, &serr) {
			problem.Write(w, r, problem.New(http.StatusBadRequest, serr.Msg))
			return
		}
		for _, m := range statusByError {
			if errors.Is(err, m.err) {
				problem.Write(w, r, problem.New(m.status, m.err.Error()))
				return
			}
		}
		log.ErrorContext(r.Context(), "unhandled error", "error", err, "path", r.URL.Path)
		problem.Write(w, r, problem.New(http.StatusInternalServerError, "internal error"))
	}
}
