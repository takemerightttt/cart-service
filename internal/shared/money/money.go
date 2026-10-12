// Package money represents amounts of money in integer minor units (cents) with an ISO 4217
// currency code, so that arithmetic never loses precision to floating point.
package money

import (
	"errors"
	"fmt"
	"math"
)

// ErrInvalid is returned when an amount is negative or a currency code is not three
// uppercase ASCII letters.
var ErrInvalid = errors.New("money: invalid amount or currency")

// ErrCurrencyMismatch is returned when two amounts in different currencies are combined.
var ErrCurrencyMismatch = errors.New("money: currency mismatch")

// ErrOverflow is returned when an operation would not fit into int64 minor units.
var ErrOverflow = errors.New("money: overflow")

// Money is a non-negative amount in minor units of Currency.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// New builds a Money value and validates both the amount and the currency code.
func New(amount int64, currency string) (Money, error) {
	if amount < 0 || !ValidCurrency(currency) {
		return Money{}, ErrInvalid
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// Zero returns a zero amount in the given currency.
func Zero(currency string) Money {
	return Money{Currency: currency}
}

// ValidCurrency reports whether code looks like an ISO 4217 alphabetic code.
func ValidCurrency(code string) bool {
	if len(code) != 3 {
		return false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < 'A' || code[i] > 'Z' {
			return false
		}
	}
	return true
}

// Add returns m + o. Both values must share a currency.
func (m Money) Add(o Money) (Money, error) {
	if m.Currency != o.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	if o.Amount > 0 && m.Amount > math.MaxInt64-o.Amount {
		return Money{}, ErrOverflow
	}
	return Money{Amount: m.Amount + o.Amount, Currency: m.Currency}, nil
}

// Mul returns m multiplied by a non-negative quantity.
func (m Money) Mul(qty int64) (Money, error) {
	if qty < 0 {
		return Money{}, ErrInvalid
	}
	if qty != 0 && m.Amount > math.MaxInt64/qty {
		return Money{}, ErrOverflow
	}
	return Money{Amount: m.Amount * qty, Currency: m.Currency}, nil
}

// String formats the amount with two decimal places, e.g. "12.34 USD".
func (m Money) String() string {
	return fmt.Sprintf("%d.%02d %s", m.Amount/100, m.Amount%100, m.Currency)
}

// Sum adds up amounts that all share currency. An empty list yields zero in currency.
func Sum(currency string, amounts ...Money) (Money, error) {
	total := Zero(currency)
	for _, a := range amounts {
		var err error
		if total, err = total.Add(a); err != nil {
			return Money{}, err
		}
	}
	return total, nil
}
