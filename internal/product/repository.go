package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/takemerightttt/cart-service/internal/postgres"
	"github.com/takemerightttt/cart-service/internal/shared/pagination"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

const columns = `id, sku, name, price_minor, currency, stock, created_at`

func scan(row pgx.Row) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Price.Amount, &p.Price.Currency, &p.Stock, &p.CreatedAt)
	return p, err
}

func (r *Repository) Create(ctx context.Context, p Product) (Product, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO products (id, sku, name, price_minor, currency, stock)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+columns,
		p.ID, p.SKU, p.Name, p.Price.Amount, p.Price.Currency, p.Stock)
	created, err := scan(row)
	if postgres.IsUniqueViolation(err, "products_sku_key") {
		return Product{}, ErrSKUTaken
	}
	if err != nil {
		return Product{}, fmt.Errorf("insert product: %w", err)
	}
	return created, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Product, error) {
	return r.get(ctx, `SELECT `+columns+` FROM products WHERE id = $1`, id)
}

func (r *Repository) GetForShare(ctx context.Context, id uuid.UUID) (Product, error) {
	return r.get(ctx, `SELECT `+columns+` FROM products WHERE id = $1 FOR SHARE`, id)
}

func (r *Repository) get(ctx context.Context, query string, id uuid.UUID) (Product, error) {
	p, err := scan(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("select product: %w", err)
	}
	return p, nil
}

func (r *Repository) List(ctx context.Context, after *pagination.Cursor, limit int) ([]Product, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if after == nil {
		rows, err = r.db.Query(ctx,
			`SELECT `+columns+` FROM products ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT `+columns+` FROM products
			 WHERE (created_at, id) < ($1, $2)
			 ORDER BY created_at DESC, id DESC LIMIT $3`, after.CreatedAt, after.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	products, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Product, error) { return scan(row) })
	if err != nil {
		return nil, fmt.Errorf("scan products: %w", err)
	}
	return products, nil
}
