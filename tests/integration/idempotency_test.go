package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIdempotentAddItemReplays(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	p := createProduct(t, 100, "USD", 10)
	key := uuid.NewString()

	first := addItem(c, cart.ID, p.ID, 2, "Idempotency-Key", key)
	expectStatus(t, first, http.StatusOK)
	second := addItem(c, cart.ID, p.ID, 2, "Idempotency-Key", key)
	expectStatus(t, second, http.StatusOK)
	if second.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("second response was not a replay")
	}
	if string(second.Body) != string(first.Body) {
		t.Fatalf("replayed body differs:\n%s\n%s", first.Body, second.Body)
	}

	var got cartDTO
	c.do(http.MethodGet, "/v1/carts/"+cart.ID.String(), nil).decode(t, &got)
	if got.Items[0].Quantity != 2 {
		t.Fatalf("quantity = %d, want 2 (the retry must not add again)", got.Items[0].Quantity)
	}
}

func TestIdempotencyKeyReusedWithDifferentBody(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	p := createProduct(t, 100, "USD", 10)
	key := uuid.NewString()
	expectStatus(t, addItem(c, cart.ID, p.ID, 1, "Idempotency-Key", key), http.StatusOK)
	expectStatus(t, addItem(c, cart.ID, p.ID, 3, "Idempotency-Key", key), http.StatusUnprocessableEntity)
}

func TestIdempotencyReplaysErrors(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	p := createProduct(t, 100, "USD", 1)
	key := uuid.NewString()
	expectStatus(t, addItem(c, cart.ID, p.ID, 5, "Idempotency-Key", key), http.StatusConflict)
	r := addItem(c, cart.ID, p.ID, 5, "Idempotency-Key", key)
	expectStatus(t, r, http.StatusConflict)
	if r.Header.Get("Idempotent-Replayed") != "true" || problemOf(t, r).Status != http.StatusConflict {
		t.Fatalf("error was not replayed as problem+json: %s", r.Body)
	}
}

func TestIdempotencyKeysAreScopedPerUser(t *testing.T) {
	t.Parallel()
	a, b := newClient(t), newClient(t)
	cartA, cartB := openCart(t, a), openCart(t, b)
	p := createProduct(t, 100, "USD", 10)
	key := uuid.NewString()
	expectStatus(t, addItem(a, cartA.ID, p.ID, 1, "Idempotency-Key", key), http.StatusOK)
	r := addItem(b, cartB.ID, p.ID, 1, "Idempotency-Key", key)
	expectStatus(t, r, http.StatusOK)
	if r.Header.Get("Idempotent-Replayed") != "" {
		t.Fatalf("user B got user A's replay")
	}
}

func TestInvalidIdempotencyKey(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	p := createProduct(t, 100, "USD", 10)
	expectStatus(t, addItem(c, cart.ID, p.ID, 1, "Idempotency-Key", strings.Repeat("k", 256)), http.StatusBadRequest)
	expectStatus(t, addItem(c, cart.ID, p.ID, 1, "Idempotency-Key", "has space"), http.StatusBadRequest)
}
