-- +goose Up
CREATE TABLE products (
    id          uuid PRIMARY KEY,
    sku         text        NOT NULL,
    name        text        NOT NULL,
    price_minor bigint      NOT NULL CHECK (price_minor > 0),
    currency    char(3)     NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    stock       integer     NOT NULL CHECK (stock >= 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT products_sku_key UNIQUE (sku)
);

CREATE INDEX products_created_at_id_idx ON products (created_at DESC, id DESC);

CREATE TABLE carts (
    id         uuid PRIMARY KEY,
    user_id    uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX carts_user_id_key ON carts (user_id);

CREATE TABLE cart_items (
    cart_id          uuid        NOT NULL REFERENCES carts (id) ON DELETE CASCADE,
    product_id       uuid        NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    quantity         integer     NOT NULL CHECK (quantity BETWEEN 1 AND 99),
    unit_price_minor bigint      NOT NULL CHECK (unit_price_minor > 0),
    currency         char(3)     NOT NULL,
    added_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cart_id, product_id)
);

CREATE INDEX cart_items_product_id_idx ON cart_items (product_id);

CREATE TABLE idempotency_keys (
    user_id       uuid        NOT NULL,
    key           text        NOT NULL,
    request_hash  bytea       NOT NULL,
    status_code   integer,
    response_body bytea,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE cart_items;
DROP TABLE carts;
DROP TABLE products;
