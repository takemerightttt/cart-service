# cart-service

A shopping cart REST API in Go: a product catalog and per-user carts with stock checks, quantity
limits, consistent money arithmetic and idempotent writes. Built as a compact example of a
production-style Go service.

## Stack

- Go 1.26, standard library `net/http` routing
- PostgreSQL 18, [pgx v5](https://github.com/jackc/pgx) connection pool
- [goose](https://github.com/pressly/goose) migrations, embedded into the binary and applied on start
- [validator v10](https://github.com/go-playground/validator) for request validation
- `log/slog` structured JSON logs with request IDs
- [testcontainers-go](https://golang.testcontainers.org/) for integration tests against a real PostgreSQL
- golangci-lint, GitHub Actions, distroless Docker image

## Quick start

```sh
docker compose up --build
```

The API listens on `http://localhost:8080`. Run it from source against your own database:

```sh
cp .env.example .env
export $(cat .env | xargs)
make run
```

| Variable | Default | Meaning |
|---|---|---|
| `DATABASE_URL` | required | PostgreSQL connection string |
| `HTTP_ADDR` | `:8080` | listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DB_MAX_CONNS` | `10` | connection pool size |

## API

All cart endpoints require the `X-User-ID` header (a UUID). It stands in for an identity that an
API gateway would extract from a verified token; carts are always looked up by `(cart id, user id)`,
so another user's cart answers `404`, never `403`.

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/products` | Create a product |
| `GET` | `/v1/products?limit=&cursor=` | List products, newest first, keyset pagination |
| `GET` | `/v1/products/{id}` | Get a product |
| `POST` | `/v1/carts` | Open the caller's cart: `201` when created, `200` when it already exists |
| `GET` | `/v1/carts/{cartID}` | Get a cart with line totals and the cart total |
| `POST` | `/v1/carts/{cartID}/items` | Add a product; repeated adds increase the quantity. Supports `Idempotency-Key` |
| `PATCH` | `/v1/carts/{cartID}/items/{productID}` | Set the quantity of an item |
| `DELETE` | `/v1/carts/{cartID}/items/{productID}` | Remove an item |
| `DELETE` | `/v1/carts/{cartID}/items` | Remove all items |
| `GET` | `/healthz`, `/readyz` | Liveness, and readiness including a database ping |

```sh
curl -s localhost:8080/v1/products -H 'Content-Type: application/json' \
  -d '{"sku":"COFFEE-1KG","name":"Coffee beans 1 kg","price_minor":1299,"currency":"EUR","stock":25}'

USER=6f1c1f7e-3c8a-4b7e-9f3a-2b1d9c0e5a11
curl -s -X POST localhost:8080/v1/carts -H "X-User-ID: $USER"

curl -s localhost:8080/v1/carts/$CART/items -H "X-User-ID: $USER" \
  -H 'Content-Type: application/json' -H "Idempotency-Key: $(uuidgen)" \
  -d '{"product_id":"'$PRODUCT'","quantity":2}'
```

A cart response:

```json
{
  "id": "0199d7a8-...",
  "user_id": "6f1c1f7e-...",
  "items": [
    {
      "product_id": "0199d7a6-...",
      "sku": "COFFEE-1KG",
      "name": "Coffee beans 1 kg",
      "unit_price": {"amount": 1299, "currency": "EUR"},
      "quantity": 2,
      "line_total": {"amount": 2598, "currency": "EUR"},
      "added_at": "2026-10-12T08:30:00Z"
    }
  ],
  "item_count": 2,
  "total": {"amount": 2598, "currency": "EUR"},
  "created_at": "2026-10-12T08:29:41Z",
  "updated_at": "2026-10-12T08:30:00Z"
}
```

### Business rules

| Rule | Response |
|---|---|
| Quantity per request and per item is 1-99 | `422` |
| Requested quantity exceeds the product stock | `409` |
| At most 50 distinct products per cart | `422` |
| All items in a cart share one currency | `422` |
| Unknown product | `404` |

### Errors

Every error is an [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem document with
`Content-Type: application/problem+json`. Validation failures list the invalid fields:

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "request validation failed",
  "instance": "/v1/carts/0199d7a8-.../items",
  "errors": [{"field": "quantity", "message": "must be at most 99"}]
}
```

Request bodies are strict: unknown fields, trailing data, wrong types and bodies over 1 MiB are rejected.

## Design notes

- **Money** is stored and computed in integer minor units with an ISO 4217 code
  (`internal/shared/money`); overflow and currency mismatches are errors, never silent.
- **Concurrency.** Every cart change runs in one transaction that locks the cart row
  (`SELECT ... FOR UPDATE`) and reads the product `FOR SHARE`, so parallel adds to the same cart
  are serialized and can never exceed stock. `TestConcurrentAddsNeverExceedStock` fires 20
  parallel requests at a product with stock 5 and expects exactly 5 successes.
- **Idempotency.** `POST /v1/carts/{id}/items` accepts an `Idempotency-Key` header. The key is
  scoped to the user and bound to a hash of the method, path and body. A retry with the same key
  and body replays the stored status and body with `Idempotent-Replayed: true`; the same key with a
  different body is `422`; a retry while the first request is still running is `409`. Server errors
  release the key so the client can retry. Keys expire after 24 hours. The key record is written
  outside the business transaction: if the process dies between the two commits, that key answers
  `409` until it expires - a deliberate trade-off that keeps the middleware independent of handlers.
- **Prices in the cart** are a snapshot of the product price at the moment of the last change of
  that line.
- **Pagination** is keyset-based on `(created_at, id)` with an opaque cursor, so pages stay stable
  while rows are inserted.
- **Layout** is package-by-feature: `handler -> service -> repository`, one direction. Code under
  `internal/shared` has no knowledge of the domain and is documented with godoc comments.

## Project layout

```
cmd/api                 entry point: config, pool, migrations, HTTP server, graceful shutdown
internal/app            routing, error mapping, middleware (request ID, logging, recovery)
internal/auth           X-User-ID authentication middleware
internal/product        product catalog
internal/cart           carts and cart items
internal/idempotency    Idempotency-Key store and middleware
internal/postgres       pool, transactions, migrations
internal/shared         money, problem details, validation, strict JSON, pagination
migrations              SQL migrations (goose)
tests/integration       end-to-end API tests on a disposable PostgreSQL
```

## Testing

```sh
make test-unit          # internal/shared, table-driven, with the race detector
make test-integration   # the HTTP API against PostgreSQL in Docker (testcontainers)
make lint
make cover
```

Integration tests need a running Docker daemon.
