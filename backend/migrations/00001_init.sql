-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE stores (
    id               uuid PRIMARY KEY,
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    icon_url         text,
    wallet_address   text UNIQUE,
    wallet_linked_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK ((wallet_address IS NULL) = (wallet_linked_at IS NULL))
);

CREATE TABLE users (
    id           uuid PRIMARY KEY,
    store_id     uuid NOT NULL REFERENCES stores (id) ON DELETE RESTRICT,
    role         text NOT NULL CHECK (role IN ('cashier', 'store_owner', 'consignor')),
    email        text NOT NULL UNIQUE CHECK (email = lower(email)),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    auth_subject text UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (store_id, id)
);

CREATE TABLE consignors (
    id                 uuid PRIMARY KEY,
    store_id           uuid NOT NULL,
    user_id            uuid NOT NULL UNIQUE,
    name               text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    wallet_address     text UNIQUE,
    wallet_verified_at timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (store_id, user_id) REFERENCES users (store_id, id) ON DELETE RESTRICT,
    CHECK (wallet_verified_at IS NULL OR wallet_address IS NOT NULL)
);
CREATE INDEX consignors_store_created_idx ON consignors (store_id, created_at DESC, id DESC);
CREATE INDEX consignors_store_user_idx ON consignors (store_id, user_id);

CREATE TABLE items (
    id                  uuid PRIMARY KEY,
    consignor_id        uuid NOT NULL REFERENCES consignors (id) ON DELETE RESTRICT,
    name                text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    price_idr           bigint NOT NULL CHECK (price_idr BETWEEN 1 AND 10000000000),
    commission_bps      integer NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000),
    stock_on_hand       integer NOT NULL CHECK (stock_on_hand BETWEEN 0 AND 1000000),
    active              boolean NOT NULL DEFAULT true,
    certificate_enabled boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX items_consignor_active_idx ON items (consignor_id, active);
CREATE INDEX items_created_idx ON items (created_at DESC, id DESC);
CREATE INDEX items_name_trgm_idx ON items USING gin (name gin_trgm_ops);

CREATE TABLE orders (
    id              uuid PRIMARY KEY,
    created_by      uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'confirmed', 'expired', 'failed')),
    reference       text NOT NULL UNIQUE,
    idr_per_sol     bigint NOT NULL CHECK (idr_per_sol > 0),
    rate_quoted_at  timestamptz NOT NULL,
    payer_wallet    text,
    signature       text UNIQUE,
    failure_code    text CHECK (failure_code IN ('TRANSACTION_FAILED', 'VALIDATION_MISMATCH')),
    idempotency_key text NOT NULL,
    request_hash    text NOT NULL,
    expires_at      timestamptz NOT NULL,
    confirmed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (created_by, idempotency_key),
    CHECK ((status = 'failed') = (failure_code IS NOT NULL)),
    CHECK (status <> 'confirmed'
           OR (signature IS NOT NULL AND payer_wallet IS NOT NULL AND confirmed_at IS NOT NULL))
);
CREATE INDEX orders_pending_expiry_idx ON orders (expires_at) WHERE status = 'pending';
CREATE INDEX orders_creator_created_idx ON orders (created_by, created_at DESC, id DESC);
CREATE INDEX orders_confirmed_idx ON orders (confirmed_at DESC, id DESC) WHERE status = 'confirmed';

CREATE TABLE order_lines (
    id                uuid PRIMARY KEY,
    order_id          uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    item_id           uuid NOT NULL REFERENCES items (id) ON DELETE RESTRICT,
    quantity          integer NOT NULL CHECK (quantity BETWEEN 1 AND 1000),
    unit_price_idr    bigint NOT NULL CHECK (unit_price_idr > 0),
    commission_bps    integer NOT NULL CHECK (commission_bps BETWEEN 0 AND 10000),
    discount_idr      bigint NOT NULL DEFAULT 0 CHECK (discount_idr >= 0),
    discount_borne_by text CHECK (discount_borne_by IN ('store', 'consignor')),
    UNIQUE (order_id, item_id),
    CHECK ((discount_idr = 0) = (discount_borne_by IS NULL))
);
CREATE INDEX order_lines_item_idx ON order_lines (item_id);

CREATE TABLE order_splits (
    id              uuid PRIMARY KEY,
    order_id        uuid NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    consignor_id    uuid REFERENCES consignors (id) ON DELETE RESTRICT,
    wallet_address  text NOT NULL,
    amount_lamports bigint NOT NULL CHECK (amount_lamports >= 0),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (order_id, consignor_id)
);
CREATE INDEX order_splits_consignor_idx ON order_splits (consignor_id);

CREATE TABLE refunds (
    id                      uuid PRIMARY KEY,
    order_line_id           uuid NOT NULL REFERENCES order_lines (id) ON DELETE RESTRICT,
    created_by              uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status                  text NOT NULL DEFAULT 'awaiting_signature'
                            CHECK (status IN ('awaiting_signature', 'confirming', 'completed', 'failed', 'expired')),
    amount_idr              bigint NOT NULL CHECK (amount_idr >= 0),
    amount_lamports         bigint NOT NULL CHECK (amount_lamports > 0),
    consignor_deduction_idr bigint NOT NULL CHECK (consignor_deduction_idr >= 0),
    source_wallet           text NOT NULL,
    reference               text NOT NULL UNIQUE,
    signature               text UNIQUE,
    reason                  text CHECK (length(reason) <= 500),
    idempotency_key         text NOT NULL,
    request_hash            text NOT NULL,
    expires_at              timestamptz NOT NULL,
    completed_at            timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (created_by, idempotency_key),
    CHECK (status <> 'completed' OR (signature IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE INDEX refunds_line_idx ON refunds (order_line_id);
CREATE UNIQUE INDEX refunds_one_running_per_line_idx ON refunds (order_line_id)
    WHERE status IN ('awaiting_signature', 'confirming');
CREATE INDEX refunds_running_expiry_idx ON refunds (expires_at)
    WHERE status IN ('awaiting_signature', 'confirming');
CREATE INDEX refunds_created_idx ON refunds (created_at DESC, id DESC);

CREATE TABLE deductions (
    id         uuid PRIMARY KEY,
    refund_id  uuid NOT NULL UNIQUE REFERENCES refunds (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE deduction_settlements (
    id             uuid PRIMARY KEY,
    deduction_id   uuid NOT NULL REFERENCES deductions (id) ON DELETE RESTRICT,
    order_split_id uuid NOT NULL REFERENCES order_splits (id) ON DELETE RESTRICT,
    amount_idr     bigint NOT NULL CHECK (amount_idr > 0),
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (deduction_id, order_split_id)
);
CREATE INDEX deduction_settlements_split_idx ON deduction_settlements (order_split_id);

CREATE TABLE certificates (
    id            uuid PRIMARY KEY,
    order_line_id uuid NOT NULL UNIQUE REFERENCES order_lines (id) ON DELETE RESTRICT,
    status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    mint_address  text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CHECK (status <> 'sent' OR mint_address IS NOT NULL)
);

CREATE TABLE wallet_nonces (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    address     text NOT NULL,
    nonce       text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX wallet_nonces_user_idx ON wallet_nonces (user_id);

-- +goose Down
DROP TABLE wallet_nonces;
DROP TABLE certificates;
DROP TABLE deduction_settlements;
DROP TABLE deductions;
DROP TABLE refunds;
DROP TABLE order_splits;
DROP TABLE order_lines;
DROP TABLE orders;
DROP TABLE items;
DROP TABLE consignors;
DROP TABLE users;
DROP TABLE stores;
