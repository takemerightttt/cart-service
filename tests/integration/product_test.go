package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCreateProduct(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	sku := "SKU-" + uuid.NewString()[:8]
	r := c.do(http.MethodPost, "/v1/products", map[string]any{
		"sku": "  " + sku + " ", "name": "Coffee beans", "price_minor": 1299, "currency": "EUR", "stock": 7,
	})
	expectStatus(t, r, http.StatusCreated)
	var p productDTO
	r.decode(t, &p)
	if p.SKU != sku || p.Price != (money{Amount: 1299, Currency: "EUR"}) || p.Stock != 7 {
		t.Fatalf("product = %+v", p)
	}
	if loc := r.Header.Get("Location"); loc != "/v1/products/"+p.ID.String() {
		t.Fatalf("location = %q", loc)
	}

	got := c.do(http.MethodGet, "/v1/products/"+p.ID.String(), nil)
	expectStatus(t, got, http.StatusOK)
	var fetched productDTO
	got.decode(t, &fetched)
	if fetched != p {
		t.Fatalf("fetched = %+v, want %+v", fetched, p)
	}

	dup := c.do(http.MethodPost, "/v1/products", map[string]any{
		"sku": sku, "name": "Other", "price_minor": 1, "currency": "EUR", "stock": 0,
	})
	expectStatus(t, dup, http.StatusConflict)
}

func TestCreateProductValidation(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	tests := []struct {
		name       string
		body       any
		status     int
		wantFields []string
	}{
		{"all invalid", map[string]any{"sku": "", "name": "", "price_minor": 0, "currency": "usd", "stock": -1},
			http.StatusUnprocessableEntity, []string{"sku", "name", "price_minor", "currency", "stock"}},
		{"unknown field", `{"sku":"ABC","name":"x","price_minor":1,"currency":"USD","stock":1,"discount":5}`, http.StatusBadRequest, nil},
		{"malformed", `{"sku":`, http.StatusBadRequest, nil},
		{"wrong type", `{"sku":"ABC","name":"x","price_minor":"1","currency":"USD","stock":1}`, http.StatusBadRequest, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := c.do(http.MethodPost, "/v1/products", tt.body)
			expectStatus(t, r, tt.status)
			p := problemOf(t, r)
			if len(p.Errors) != len(tt.wantFields) {
				t.Fatalf("errors = %+v, want fields %v", p.Errors, tt.wantFields)
			}
			for i, f := range tt.wantFields {
				if p.Errors[i].Field != f {
					t.Fatalf("error %d field = %q, want %q", i, p.Errors[i].Field, f)
				}
			}
		})
	}
}

func TestCreateProductRejectsNonJSON(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	r := c.do(http.MethodPost, "/v1/products", `sku=ABC`, "Content-Type", "application/x-www-form-urlencoded")
	expectStatus(t, r, http.StatusUnsupportedMediaType)
}

func TestGetProductNotFound(t *testing.T) {
	t.Parallel()
	c := &client{t: t}
	expectStatus(t, c.do(http.MethodGet, "/v1/products/"+uuid.NewString(), nil), http.StatusNotFound)
	expectStatus(t, c.do(http.MethodGet, "/v1/products/not-a-uuid", nil), http.StatusNotFound)
}

func TestListProductsPagination(t *testing.T) {
	c := &client{t: t}
	created := map[uuid.UUID]bool{}
	for range 5 {
		created[createProduct(t, 100, "USD", 1).ID] = true
	}

	seen := map[uuid.UUID]bool{}
	cursor, pages := "", 0
	for {
		path := "/v1/products?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := c.do(http.MethodGet, path, nil)
		expectStatus(t, r, http.StatusOK)
		var page struct {
			Items      []productDTO `json:"items"`
			NextCursor string       `json:"next_cursor"`
		}
		r.decode(t, &page)
		if len(page.Items) > 2 {
			t.Fatalf("page has %d items, limit 2", len(page.Items))
		}
		for _, p := range page.Items {
			if seen[p.ID] {
				t.Fatalf("product %s returned twice", p.ID)
			}
			seen[p.ID] = true
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	for id := range created {
		if !seen[id] {
			t.Fatalf("product %s missing from pages", id)
		}
	}
	if pages < 3 {
		t.Fatalf("pages = %d, want at least 3", pages)
	}

	expectStatus(t, c.do(http.MethodGet, "/v1/products?limit=0", nil), http.StatusBadRequest)
	expectStatus(t, c.do(http.MethodGet, "/v1/products?cursor=garbage", nil), http.StatusBadRequest)
}
