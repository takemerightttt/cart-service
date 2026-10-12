package money_test

import (
	"errors"
	"math"
	"testing"

	"github.com/takemerightttt/cart-service/internal/shared/money"
)

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		amount   int64
		currency string
		wantErr  error
	}{
		{"valid", 1999, "USD", nil},
		{"zero", 0, "EUR", nil},
		{"negative", -1, "USD", money.ErrInvalid},
		{"lowercase currency", 100, "usd", money.ErrInvalid},
		{"short currency", 100, "US", money.ErrInvalid},
		{"long currency", 100, "USDT", money.ErrInvalid},
		{"digits", 100, "US1", money.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := money.New(tt.amount, tt.currency)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New(%d, %q) error = %v, want %v", tt.amount, tt.currency, err, tt.wantErr)
			}
			if err == nil && (m.Amount != tt.amount || m.Currency != tt.currency) {
				t.Fatalf("New(%d, %q) = %+v", tt.amount, tt.currency, m)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		a, b    money.Money
		want    money.Money
		wantErr error
	}{
		{"same currency", money.Money{Amount: 150, Currency: "USD"}, money.Money{Amount: 250, Currency: "USD"}, money.Money{Amount: 400, Currency: "USD"}, nil},
		{"mismatch", money.Money{Amount: 1, Currency: "USD"}, money.Money{Amount: 1, Currency: "EUR"}, money.Money{}, money.ErrCurrencyMismatch},
		{"overflow", money.Money{Amount: math.MaxInt64, Currency: "USD"}, money.Money{Amount: 1, Currency: "USD"}, money.Money{}, money.ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.a.Add(tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Add = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMul(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		m       money.Money
		qty     int64
		want    money.Money
		wantErr error
	}{
		{"by three", money.Money{Amount: 333, Currency: "USD"}, 3, money.Money{Amount: 999, Currency: "USD"}, nil},
		{"by zero", money.Money{Amount: 333, Currency: "USD"}, 0, money.Money{Amount: 0, Currency: "USD"}, nil},
		{"negative qty", money.Money{Amount: 1, Currency: "USD"}, -1, money.Money{}, money.ErrInvalid},
		{"overflow", money.Money{Amount: math.MaxInt64/2 + 1, Currency: "USD"}, 2, money.Money{}, money.ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.m.Mul(tt.qty)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Mul error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Mul = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSum(t *testing.T) {
	t.Parallel()
	got, err := money.Sum("USD", money.Money{Amount: 100, Currency: "USD"}, money.Money{Amount: 205, Currency: "USD"})
	if err != nil || got != (money.Money{Amount: 305, Currency: "USD"}) {
		t.Fatalf("Sum = %+v, %v", got, err)
	}
	empty, err := money.Sum("EUR")
	if err != nil || empty != money.Zero("EUR") {
		t.Fatalf("Sum() = %+v, %v", empty, err)
	}
	if _, err := money.Sum("USD", money.Money{Amount: 1, Currency: "EUR"}); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("Sum mismatch error = %v", err)
	}
}

func TestString(t *testing.T) {
	t.Parallel()
	tests := map[money.Money]string{
		{Amount: 1234, Currency: "USD"}: "12.34 USD",
		{Amount: 5, Currency: "EUR"}:    "0.05 EUR",
		{Amount: 0, Currency: "RSD"}:    "0.00 RSD",
	}
	for m, want := range tests {
		if got := m.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", m, got, want)
		}
	}
}
