package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type productDTO struct {
	ID    uuid.UUID `json:"id"`
	SKU   string    `json:"sku"`
	Name  string    `json:"name"`
	Price money     `json:"price"`
	Stock int       `json:"stock"`
}

type itemDTO struct {
	ProductID uuid.UUID `json:"product_id"`
	SKU       string    `json:"sku"`
	UnitPrice money     `json:"unit_price"`
	Quantity  int       `json:"quantity"`
	LineTotal money     `json:"line_total"`
}

type cartDTO struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Items     []itemDTO `json:"items"`
	ItemCount int       `json:"item_count"`
	Total     *money    `json:"total"`
}

type problemDTO struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Errors []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"errors"`
}

func createProduct(t *testing.T, price int64, currency string, stock int) productDTO {
	t.Helper()
	c := &client{t: t}
	r := c.do(http.MethodPost, "/v1/products", map[string]any{
		"sku": "SKU-" + uuid.NewString()[:8], "name": "Test product", "price_minor": price, "currency": currency, "stock": stock,
	})
	expectStatus(t, r, http.StatusCreated)
	var p productDTO
	r.decode(t, &p)
	return p
}

func openCart(t *testing.T, c *client) cartDTO {
	t.Helper()
	r := c.do(http.MethodPost, "/v1/carts", nil)
	expectStatus(t, r, http.StatusCreated)
	var cart cartDTO
	r.decode(t, &cart)
	return cart
}

func addItem(c *client, cartID, productID uuid.UUID, qty int, headers ...string) response {
	c.t.Helper()
	return c.do(http.MethodPost, "/v1/carts/"+cartID.String()+"/items",
		map[string]any{"product_id": productID, "quantity": qty}, headers...)
}

func problemOf(t *testing.T, r response) problemDTO {
	t.Helper()
	if ct := r.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q, want application/problem+json; body %s", ct, r.Body)
	}
	var p problemDTO
	r.decode(t, &p)
	return p
}
