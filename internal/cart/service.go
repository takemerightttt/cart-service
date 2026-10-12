package cart

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/takemerightttt/cart-service/internal/postgres"
	"github.com/takemerightttt/cart-service/internal/product"
	"github.com/takemerightttt/cart-service/internal/shared/validation"
)

type Service struct {
	pool     *pgxpool.Pool
	validate *validation.Validator
}

func NewService(pool *pgxpool.Pool, v *validation.Validator) *Service {
	return &Service{pool: pool, validate: v}
}

func (s *Service) Open(ctx context.Context, userID uuid.UUID) (Cart, bool, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Cart{}, false, err
	}
	repo := NewRepository(s.pool)
	h, created, err := repo.CreateForUser(ctx, id, userID)
	if err != nil {
		return Cart{}, false, err
	}
	c, err := load(ctx, repo, h)
	return c, created, err
}

func (s *Service) Get(ctx context.Context, userID, cartID uuid.UUID) (Cart, error) {
	repo := NewRepository(s.pool)
	h, err := repo.Header(ctx, cartID, userID)
	if err != nil {
		return Cart{}, err
	}
	return load(ctx, repo, h)
}

func (s *Service) AddItem(ctx context.Context, userID, cartID uuid.UUID, in AddItemInput) (Cart, error) {
	if err := s.validate.Struct(in); err != nil {
		return Cart{}, err
	}
	productID := uuid.MustParse(in.ProductID)
	return s.mutate(ctx, userID, cartID, func(tx pgx.Tx, repo *Repository) error {
		p, err := product.NewRepository(tx).GetForShare(ctx, productID)
		if err != nil {
			return err
		}
		cur, err := repo.Contents(ctx, cartID, productID)
		if err != nil {
			return err
		}
		if !cur.Exists && cur.Distinct >= MaxDistinctItems {
			return ErrTooManyItems
		}
		if cur.Currency != "" && cur.Currency != p.Price.Currency {
			return ErrCurrencyMismatch
		}
		qty := cur.Quantity + in.Quantity
		if err := checkQuantity(qty, p.Stock); err != nil {
			return err
		}
		return repo.UpsertItem(ctx, cartID, productID, qty, p.Price)
	})
}

func (s *Service) UpdateItem(ctx context.Context, userID, cartID, productID uuid.UUID, in UpdateItemInput) (Cart, error) {
	if err := s.validate.Struct(in); err != nil {
		return Cart{}, err
	}
	return s.mutate(ctx, userID, cartID, func(tx pgx.Tx, repo *Repository) error {
		cur, err := repo.Contents(ctx, cartID, productID)
		if err != nil {
			return err
		}
		if !cur.Exists {
			return ErrItemNotFound
		}
		p, err := product.NewRepository(tx).GetForShare(ctx, productID)
		if err != nil {
			return err
		}
		if err := checkQuantity(in.Quantity, p.Stock); err != nil {
			return err
		}
		return repo.UpsertItem(ctx, cartID, productID, in.Quantity, p.Price)
	})
}

func (s *Service) RemoveItem(ctx context.Context, userID, cartID, productID uuid.UUID) (Cart, error) {
	return s.mutate(ctx, userID, cartID, func(_ pgx.Tx, repo *Repository) error {
		removed, err := repo.DeleteItem(ctx, cartID, productID)
		if err != nil {
			return err
		}
		if !removed {
			return ErrItemNotFound
		}
		return nil
	})
}

func (s *Service) Clear(ctx context.Context, userID, cartID uuid.UUID) (Cart, error) {
	return s.mutate(ctx, userID, cartID, func(_ pgx.Tx, repo *Repository) error {
		return repo.DeleteItems(ctx, cartID)
	})
}

func (s *Service) mutate(ctx context.Context, userID, cartID uuid.UUID, fn func(pgx.Tx, *Repository) error) (Cart, error) {
	var c Cart
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := NewRepository(tx)
		h, err := repo.LockHeader(ctx, cartID, userID)
		if err != nil {
			return err
		}
		if err := fn(tx, repo); err != nil {
			return err
		}
		if err := repo.Touch(ctx, cartID); err != nil {
			return err
		}
		h, err = repo.Header(ctx, h.ID, userID)
		if err != nil {
			return err
		}
		c, err = load(ctx, repo, h)
		return err
	})
	return c, err
}

func checkQuantity(qty, stock int) error {
	switch {
	case qty > MaxQuantityPerItem:
		return ErrQuantityLimit
	case qty > stock:
		return ErrInsufficientStock
	default:
		return nil
	}
}

func load(ctx context.Context, repo *Repository, h header) (Cart, error) {
	items, err := repo.Items(ctx, h.ID)
	if err != nil {
		return Cart{}, err
	}
	c := Cart{ID: h.ID, UserID: h.UserID, Items: items, CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt}
	if err := c.computeTotals(); err != nil {
		return Cart{}, errors.Join(errors.New("compute cart totals"), err)
	}
	return c, nil
}
