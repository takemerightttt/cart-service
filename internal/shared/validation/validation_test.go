package validation_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/takemerightttt/cart-service/internal/shared/problem"
	"github.com/takemerightttt/cart-service/internal/shared/validation"
)

type item struct {
	SKU      string `json:"sku" validate:"required,min=3,max=8"`
	Quantity int    `json:"quantity" validate:"gte=1,lte=99"`
	Currency string `json:"currency" validate:"required,currency"`
	Product  string `json:"product_id" validate:"required,uuid"`
}

func valid() item {
	return item{SKU: "ABC-1", Quantity: 2, Currency: "USD", Product: "6f1c1f7e-3c8a-4b7e-9f3a-2b1d9c0e5a11"}
}

func TestStruct(t *testing.T) {
	t.Parallel()
	v := validation.New()
	tests := []struct {
		name   string
		mutate func(*item)
		want   []problem.FieldError
	}{
		{"valid", func(*item) {}, nil},
		{"missing sku", func(i *item) { i.SKU = "" }, []problem.FieldError{{Field: "sku", Message: "is required"}}},
		{"short sku", func(i *item) { i.SKU = "AB" }, []problem.FieldError{{Field: "sku", Message: "must be at least 3 characters"}}},
		{"quantity zero", func(i *item) { i.Quantity = 0 }, []problem.FieldError{{Field: "quantity", Message: "must be at least 1"}}},
		{"quantity too big", func(i *item) { i.Quantity = 100 }, []problem.FieldError{{Field: "quantity", Message: "must be at most 99"}}},
		{"bad currency", func(i *item) { i.Currency = "usd" }, []problem.FieldError{{Field: "currency", Message: "must be an ISO 4217 code such as USD"}}},
		{"bad uuid", func(i *item) { i.Product = "nope" }, []problem.FieldError{{Field: "product_id", Message: "must be a valid UUID"}}},
		{"two fields", func(i *item) { i.SKU = ""; i.Quantity = 0 }, []problem.FieldError{
			{Field: "sku", Message: "is required"}, {Field: "quantity", Message: "must be at least 1"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := valid()
			tt.mutate(&in)
			err := v.Struct(in)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var verr *validation.Error
			if !errors.As(err, &verr) {
				t.Fatalf("error = %v, want *validation.Error", err)
			}
			if len(verr.Fields) != len(tt.want) {
				t.Fatalf("fields = %+v, want %+v", verr.Fields, tt.want)
			}
			for i := range tt.want {
				if verr.Fields[i] != tt.want[i] {
					t.Fatalf("field %d = %+v, want %+v", i, verr.Fields[i], tt.want[i])
				}
			}
		})
	}
}

func TestStructNested(t *testing.T) {
	t.Parallel()
	type order struct {
		Item item `json:"item"`
	}
	in := order{Item: valid()}
	in.Item.Quantity = 0
	var verr *validation.Error
	if err := validation.New().Struct(in); !errors.As(err, &verr) || verr.Fields[0].Field != "item.quantity" {
		t.Fatalf("nested error = %v", err)
	}
}

func TestErrorMessage(t *testing.T) {
	t.Parallel()
	err := &validation.Error{Fields: []problem.FieldError{{Field: "a", Message: "is required"}, {Field: "b", Message: "is invalid"}}}
	if got, want := err.Error(), "validation failed: a: is required; b: is invalid"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestConcurrentUse(t *testing.T) {
	t.Parallel()
	v := validation.New()
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := v.Struct(valid()); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
}
