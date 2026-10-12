package cart

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/takemerightttt/cart-service/internal/shared/money"
)

const (
	MaxQuantityPerItem = 99
	MaxDistinctItems   = 50
)

var (
	ErrNotFound          = errors.New("cart not found")
	ErrItemNotFound      = errors.New("item is not in the cart")
	ErrInsufficientStock = errors.New("not enough stock for the requested quantity")
	ErrQuantityLimit     = errors.New("quantity per item exceeds the limit")
	ErrTooManyItems      = errors.New("cart already holds the maximum number of distinct items")
	ErrCurrencyMismatch  = errors.New("product currency differs from the cart currency")
)

type Cart struct {
	ID        uuid.UUID    `json:"id"`
	UserID    uuid.UUID    `json:"user_id"`
	Items     []Item       `json:"items"`
	ItemCount int          `json:"item_count"`
	Total     *money.Money `json:"total"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Item struct {
	ProductID uuid.UUID   `json:"product_id"`
	SKU       string      `json:"sku"`
	Name      string      `json:"name"`
	UnitPrice money.Money `json:"unit_price"`
	Quantity  int         `json:"quantity"`
	LineTotal money.Money `json:"line_total"`
	AddedAt   time.Time   `json:"added_at"`
}

type AddItemInput struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"gte=1,lte=99"`
}

type UpdateItemInput struct {
	Quantity int `json:"quantity" validate:"gte=1,lte=99"`
}

func (c *Cart) computeTotals() error {
	if c.Items == nil {
		c.Items = []Item{}
	}
	c.ItemCount, c.Total = 0, nil
	if len(c.Items) == 0 {
		return nil
	}
	lines := make([]money.Money, 0, len(c.Items))
	for i := range c.Items {
		line, err := c.Items[i].UnitPrice.Mul(int64(c.Items[i].Quantity))
		if err != nil {
			return err
		}
		c.Items[i].LineTotal = line
		c.ItemCount += c.Items[i].Quantity
		lines = append(lines, line)
	}
	total, err := money.Sum(c.Items[0].UnitPrice.Currency, lines...)
	if err != nil {
		return err
	}
	c.Total = &total
	return nil
}
