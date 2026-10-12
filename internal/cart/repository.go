package cart

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/takemerightttt/cart-service/internal/postgres"
	"github.com/takemerightttt/cart-service/internal/shared/money"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

type header struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r *Repository) CreateForUser(ctx context.Context, id, userID uuid.UUID) (header, bool, error) {
	var h header
	err := r.db.QueryRow(ctx,
		`INSERT INTO carts (id, user_id) VALUES ($1, $2)
		 ON CONFLICT (user_id) DO NOTHING
		 RETURNING id, user_id, created_at, updated_at`, id, userID).
		Scan(&h.ID, &h.UserID, &h.CreatedAt, &h.UpdatedAt)
	if err == nil {
		return h, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return header{}, false, fmt.Errorf("insert cart: %w", err)
	}
	err = r.db.QueryRow(ctx,
		`SELECT id, user_id, created_at, updated_at FROM carts WHERE user_id = $1`, userID).
		Scan(&h.ID, &h.UserID, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return header{}, false, fmt.Errorf("select cart by user: %w", err)
	}
	return h, false, nil
}

func (r *Repository) Header(ctx context.Context, id, userID uuid.UUID) (header, error) {
	return r.header(ctx, `SELECT id, user_id, created_at, updated_at FROM carts WHERE id = $1 AND user_id = $2`, id, userID)
}

func (r *Repository) LockHeader(ctx context.Context, id, userID uuid.UUID) (header, error) {
	return r.header(ctx, `SELECT id, user_id, created_at, updated_at FROM carts WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, userID)
}

func (r *Repository) header(ctx context.Context, query string, id, userID uuid.UUID) (header, error) {
	var h header
	err := r.db.QueryRow(ctx, query, id, userID).Scan(&h.ID, &h.UserID, &h.CreatedAt, &h.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return header{}, ErrNotFound
	}
	if err != nil {
		return header{}, fmt.Errorf("select cart: %w", err)
	}
	return h, nil
}

func (r *Repository) Items(ctx context.Context, cartID uuid.UUID) ([]Item, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ci.product_id, p.sku, p.name, ci.unit_price_minor, ci.currency, ci.quantity, ci.added_at
		 FROM cart_items ci
		 JOIN products p ON p.id = ci.product_id
		 WHERE ci.cart_id = $1
		 ORDER BY ci.added_at, ci.product_id`, cartID)
	if err != nil {
		return nil, fmt.Errorf("select cart items: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Item, error) {
		var it Item
		err := row.Scan(&it.ProductID, &it.SKU, &it.Name, &it.UnitPrice.Amount, &it.UnitPrice.Currency, &it.Quantity, &it.AddedAt)
		return it, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan cart items: %w", err)
	}
	return items, nil
}

type contents struct {
	Quantity int
	Exists   bool
	Distinct int
	Currency string
}

func (r *Repository) Contents(ctx context.Context, cartID, productID uuid.UUID) (contents, error) {
	var c contents
	var currency *string
	err := r.db.QueryRow(ctx,
		`SELECT
		     COALESCE(MAX(quantity) FILTER (WHERE product_id = $2), 0),
		     COUNT(*) FILTER (WHERE product_id = $2) > 0,
		     COUNT(*),
		     MIN(currency)
		 FROM cart_items WHERE cart_id = $1`, cartID, productID).
		Scan(&c.Quantity, &c.Exists, &c.Distinct, &currency)
	if err != nil {
		return contents{}, fmt.Errorf("select cart contents: %w", err)
	}
	if currency != nil {
		c.Currency = *currency
	}
	return c, nil
}

func (r *Repository) UpsertItem(ctx context.Context, cartID, productID uuid.UUID, quantity int, price money.Money) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO cart_items (cart_id, product_id, quantity, unit_price_minor, currency)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (cart_id, product_id) DO UPDATE
		 SET quantity = EXCLUDED.quantity,
		     unit_price_minor = EXCLUDED.unit_price_minor,
		     currency = EXCLUDED.currency`,
		cartID, productID, quantity, price.Amount, price.Currency)
	if err != nil {
		return fmt.Errorf("upsert cart item: %w", err)
	}
	return nil
}

func (r *Repository) DeleteItem(ctx context.Context, cartID, productID uuid.UUID) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1 AND product_id = $2`, cartID, productID)
	if err != nil {
		return false, fmt.Errorf("delete cart item: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) DeleteItems(ctx context.Context, cartID uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1`, cartID); err != nil {
		return fmt.Errorf("delete cart items: %w", err)
	}
	return nil
}

func (r *Repository) Touch(ctx context.Context, cartID uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `UPDATE carts SET updated_at = now() WHERE id = $1`, cartID); err != nil {
		return fmt.Errorf("touch cart: %w", err)
	}
	return nil
}
