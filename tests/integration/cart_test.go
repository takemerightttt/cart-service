package integration

import (
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestOpenCartIsIdempotentPerUser(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	first := openCart(t, c)
	if first.UserID != c.userID || len(first.Items) != 0 || first.Total != nil {
		t.Fatalf("new cart = %+v", first)
	}
	again := c.do(http.MethodPost, "/v1/carts", nil)
	expectStatus(t, again, http.StatusOK)
	var second cartDTO
	again.decode(t, &second)
	if second.ID != first.ID {
		t.Fatalf("second open returned cart %s, want %s", second.ID, first.ID)
	}
}

func TestCartRequiresUser(t *testing.T) {
	t.Parallel()
	anon := &client{t: t}
	expectStatus(t, anon.do(http.MethodPost, "/v1/carts", nil), http.StatusUnauthorized)
	bad := &client{t: t}
	expectStatus(t, bad.do(http.MethodPost, "/v1/carts", nil, "X-User-ID", "not-a-uuid"), http.StatusUnauthorized)
}

func TestAddItemsAndTotals(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	coffee := createProduct(t, 1299, "EUR", 10)
	mug := createProduct(t, 450, "EUR", 10)

	expectStatus(t, addItem(c, cart.ID, coffee.ID, 2), http.StatusOK)
	expectStatus(t, addItem(c, cart.ID, mug.ID, 1), http.StatusOK)
	r := addItem(c, cart.ID, coffee.ID, 1)
	expectStatus(t, r, http.StatusOK)

	var got cartDTO
	r.decode(t, &got)
	if len(got.Items) != 2 || got.ItemCount != 4 {
		t.Fatalf("cart = %+v", got)
	}
	if got.Items[0].ProductID != coffee.ID || got.Items[0].Quantity != 3 || got.Items[0].LineTotal != (money{3897, "EUR"}) {
		t.Fatalf("coffee line = %+v", got.Items[0])
	}
	if got.Total == nil || *got.Total != (money{4347, "EUR"}) {
		t.Fatalf("total = %+v, want 4347 EUR", got.Total)
	}

	fetched := c.do(http.MethodGet, "/v1/carts/"+cart.ID.String(), nil)
	expectStatus(t, fetched, http.StatusOK)
	var again cartDTO
	fetched.decode(t, &again)
	if again.Total == nil || *again.Total != *got.Total {
		t.Fatalf("fetched total = %+v", again.Total)
	}
}

func TestAddItemValidation(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	p := createProduct(t, 100, "USD", 500)
	tests := []struct {
		name   string
		body   any
		status int
		field  string
	}{
		{"zero quantity", map[string]any{"product_id": p.ID, "quantity": 0}, http.StatusUnprocessableEntity, "quantity"},
		{"quantity over 99", map[string]any{"product_id": p.ID, "quantity": 100}, http.StatusUnprocessableEntity, "quantity"},
		{"missing product", map[string]any{"quantity": 1}, http.StatusUnprocessableEntity, "product_id"},
		{"bad product id", map[string]any{"product_id": "abc", "quantity": 1}, http.StatusUnprocessableEntity, "product_id"},
		{"unknown product", map[string]any{"product_id": uuid.New(), "quantity": 1}, http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := c.do(http.MethodPost, "/v1/carts/"+cart.ID.String()+"/items", tt.body)
			expectStatus(t, r, tt.status)
			pr := problemOf(t, r)
			if tt.field != "" && (len(pr.Errors) != 1 || pr.Errors[0].Field != tt.field) {
				t.Fatalf("errors = %+v, want field %q", pr.Errors, tt.field)
			}
		})
	}
}

func TestBusinessRules(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)

	scarce := createProduct(t, 100, "USD", 3)
	expectStatus(t, addItem(c, cart.ID, scarce.ID, 2), http.StatusOK)
	r := addItem(c, cart.ID, scarce.ID, 2)
	expectStatus(t, r, http.StatusConflict)
	if p := problemOf(t, r); p.Detail != "not enough stock for the requested quantity" {
		t.Fatalf("detail = %q", p.Detail)
	}

	plenty := createProduct(t, 100, "USD", 1000)
	expectStatus(t, addItem(c, cart.ID, plenty.ID, 60), http.StatusOK)
	expectStatus(t, addItem(c, cart.ID, plenty.ID, 40), http.StatusUnprocessableEntity)

	euro := createProduct(t, 100, "EUR", 10)
	expectStatus(t, addItem(c, cart.ID, euro.ID, 1), http.StatusUnprocessableEntity)
}

func TestDistinctItemLimit(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	for range 50 {
		expectStatus(t, addItem(c, cart.ID, createProduct(t, 100, "USD", 5).ID, 1), http.StatusOK)
	}
	expectStatus(t, addItem(c, cart.ID, createProduct(t, 100, "USD", 5).ID, 1), http.StatusUnprocessableEntity)
}

func TestUpdateRemoveClear(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	a := createProduct(t, 200, "USD", 10)
	b := createProduct(t, 300, "USD", 10)
	expectStatus(t, addItem(c, cart.ID, a.ID, 1), http.StatusOK)
	expectStatus(t, addItem(c, cart.ID, b.ID, 1), http.StatusOK)

	base := "/v1/carts/" + cart.ID.String() + "/items/"
	r := c.do(http.MethodPatch, base+a.ID.String(), map[string]any{"quantity": 5})
	expectStatus(t, r, http.StatusOK)
	var got cartDTO
	r.decode(t, &got)
	if got.Total == nil || *got.Total != (money{1300, "USD"}) {
		t.Fatalf("total after update = %+v", got.Total)
	}
	expectStatus(t, c.do(http.MethodPatch, base+a.ID.String(), map[string]any{"quantity": 11}), http.StatusConflict)
	expectStatus(t, c.do(http.MethodPatch, base+uuid.NewString(), map[string]any{"quantity": 1}), http.StatusNotFound)

	r = c.do(http.MethodDelete, base+b.ID.String(), nil)
	expectStatus(t, r, http.StatusOK)
	r.decode(t, &got)
	if len(got.Items) != 1 || got.Items[0].ProductID != a.ID {
		t.Fatalf("items after remove = %+v", got.Items)
	}
	expectStatus(t, c.do(http.MethodDelete, base+b.ID.String(), nil), http.StatusNotFound)

	r = c.do(http.MethodDelete, "/v1/carts/"+cart.ID.String()+"/items", nil)
	expectStatus(t, r, http.StatusOK)
	r.decode(t, &got)
	if len(got.Items) != 0 || got.Total != nil || got.ItemCount != 0 {
		t.Fatalf("cart after clear = %+v", got)
	}
}

func TestOtherUserCannotSeeOrChangeCart(t *testing.T) {
	t.Parallel()
	owner, intruder := newClient(t), newClient(t)
	cart := openCart(t, owner)
	p := createProduct(t, 100, "USD", 10)
	expectStatus(t, addItem(owner, cart.ID, p.ID, 1), http.StatusOK)

	path := "/v1/carts/" + cart.ID.String()
	expectStatus(t, intruder.do(http.MethodGet, path, nil), http.StatusNotFound)
	expectStatus(t, addItem(intruder, cart.ID, p.ID, 1), http.StatusNotFound)
	expectStatus(t, intruder.do(http.MethodPatch, path+"/items/"+p.ID.String(), map[string]any{"quantity": 2}), http.StatusNotFound)
	expectStatus(t, intruder.do(http.MethodDelete, path+"/items/"+p.ID.String(), nil), http.StatusNotFound)
	expectStatus(t, intruder.do(http.MethodDelete, path+"/items", nil), http.StatusNotFound)

	r := owner.do(http.MethodGet, path, nil)
	var got cartDTO
	r.decode(t, &got)
	if len(got.Items) != 1 || got.Items[0].Quantity != 1 {
		t.Fatalf("owner cart changed: %+v", got.Items)
	}
}

func TestConcurrentAddsNeverExceedStock(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	cart := openCart(t, c)
	p := createProduct(t, 100, "USD", 5)

	const workers = 20
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		statuses = map[int]int{}
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := addItem(c, cart.ID, p.ID, 1)
			mu.Lock()
			statuses[r.Status]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if statuses[http.StatusOK] != 5 || statuses[http.StatusConflict] != workers-5 {
		t.Fatalf("statuses = %v, want 5 OK and %d conflicts", statuses, workers-5)
	}
	var got cartDTO
	c.do(http.MethodGet, "/v1/carts/"+cart.ID.String(), nil).decode(t, &got)
	if got.Items[0].Quantity != 5 {
		t.Fatalf("quantity = %d, want 5", got.Items[0].Quantity)
	}
}
