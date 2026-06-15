-- Accounts hold a current balance snapshot. id is supplied by the client.
CREATE TABLE IF NOT EXISTS accounts (
    id         BIGINT         PRIMARY KEY,
    balance    NUMERIC(38, 18) NOT NULL,
    version    BIGINT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    -- Balance can never go negative; safety net behind app-level checks.
    CONSTRAINT balance_non_negative CHECK (balance >= 0)
);

-- Immutable transfer log. Holds the optional idempotency key.
CREATE TABLE IF NOT EXISTS transfers (
    id              BIGSERIAL      PRIMARY KEY,
    idempotency_key UUID,
    source_id       BIGINT         NOT NULL REFERENCES accounts(id),
    destination_id  BIGINT         NOT NULL REFERENCES accounts(id),
    amount          NUMERIC(38, 18) NOT NULL CHECK (amount > 0),
    status          TEXT           NOT NULL,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now(),
    -- Source and destination must differ.
    CONSTRAINT chk_diff_accounts CHECK (source_id <> destination_id)
);

-- Enforce idempotency only when a key is provided.
CREATE UNIQUE INDEX IF NOT EXISTS uq_transfers_idempotency_key
    ON transfers (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_transfers_source      ON transfers (source_id);
CREATE INDEX IF NOT EXISTS idx_transfers_destination ON transfers (destination_id);

-- Double-entry ledger: two rows per transfer (debit + credit).
-- balance_after gives a per-account snapshot for audit and replay.
CREATE TABLE IF NOT EXISTS ledger_entries (
    id            BIGSERIAL      PRIMARY KEY,
    transfer_id   BIGINT         NOT NULL REFERENCES transfers(id),
    account_id    BIGINT         NOT NULL REFERENCES accounts(id),
    direction     TEXT           NOT NULL,  -- 'debit' (out) or 'credit' (in)
    amount        NUMERIC(38, 18) NOT NULL CHECK (amount > 0),
    balance_after NUMERIC(38, 18) NOT NULL,
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- Account statement lookups, ordered by time.
CREATE INDEX IF NOT EXISTS idx_ledger_account  ON ledger_entries (account_id, id);
CREATE INDEX IF NOT EXISTS idx_ledger_transfer ON ledger_entries (transfer_id);
