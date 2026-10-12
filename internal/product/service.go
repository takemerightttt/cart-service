package product

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/takemerightttt/cart-service/internal/shared/money"
	"github.com/takemerightttt/cart-service/internal/shared/pagination"
	"github.com/takemerightttt/cart-service/internal/shared/validation"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Service struct {
	repo     *Repository
	validate *validation.Validator
}

func NewService(pool *pgxpool.Pool, v *validation.Validator) *Service {
	return &Service{repo: NewRepository(pool), validate: v}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Product, error) {
	in.SKU = strings.TrimSpace(in.SKU)
	in.Name = strings.TrimSpace(in.Name)
	if err := s.validate.Struct(in); err != nil {
		return Product{}, err
	}
	price, err := money.New(in.PriceMinor, in.Currency)
	if err != nil {
		return Product{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Product{}, err
	}
	return s.repo.Create(ctx, Product{ID: id, SKU: in.SKU, Name: in.Name, Price: price, Stock: in.Stock})
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Product, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, cursor string, limit int) (pagination.Page[Product], error) {
	after, err := pagination.Decode(cursor)
	if err != nil {
		return pagination.Page[Product]{}, err
	}
	rows, err := s.repo.List(ctx, after, limit+1)
	if err != nil {
		return pagination.Page[Product]{}, err
	}
	return pagination.NewPage(rows, limit, Product.Cursor), nil
}
