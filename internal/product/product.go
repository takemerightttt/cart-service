package product

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/takemerightttt/cart-service/internal/shared/money"
	"github.com/takemerightttt/cart-service/internal/shared/pagination"
)

var (
	ErrNotFound = errors.New("product not found")
	ErrSKUTaken = errors.New("product sku already exists")
)

type Product struct {
	ID        uuid.UUID   `json:"id"`
	SKU       string      `json:"sku"`
	Name      string      `json:"name"`
	Price     money.Money `json:"price"`
	Stock     int         `json:"stock"`
	CreatedAt time.Time   `json:"created_at"`
}

func (p Product) Cursor() pagination.Cursor {
	return pagination.Cursor{CreatedAt: p.CreatedAt, ID: p.ID}
}

type CreateInput struct {
	SKU        string `json:"sku" validate:"required,min=3,max=64,printascii"`
	Name       string `json:"name" validate:"required,min=1,max=200"`
	PriceMinor int64  `json:"price_minor" validate:"gt=0,lte=100000000"`
	Currency   string `json:"currency" validate:"required,currency"`
	Stock      int    `json:"stock" validate:"gte=0,lte=1000000"`
}
