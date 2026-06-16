-- Accounts hold a current balance snapshot. id is supplied by the client.
-- accounts 保存当前余额快照；id 由客户端指定。
CREATE TABLE IF NOT EXISTS accounts (
    id         BIGINT         PRIMARY KEY,
    balance    NUMERIC(38, 18) NOT NULL,
    version    BIGINT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    -- Balance can never go negative; safety net behind app-level checks.
    -- 余额永不为负；应用层校验之外的安全网。
    CONSTRAINT balance_non_negative CHECK (balance >= 0)
);

-- Immutable deposit log for initial account funding.
-- 不可变的充值日志，用于追溯账户的初始资金来源。
CREATE TABLE IF NOT EXISTS deposits (
    id         BIGSERIAL      PRIMARY KEY,
    account_id BIGINT         NOT NULL REFERENCES accounts(id),
    amount     NUMERIC(38, 18) NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- Immutable transfer log. Holds the optional idempotency key.
-- 不可变的转账日志；保存可选的幂等键。
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
-- 仅在提供了幂等键时强制唯一（幂等约束）。
CREATE UNIQUE INDEX IF NOT EXISTS uq_transfers_idempotency_key
    ON transfers (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_transfers_source      ON transfers (source_id);
CREATE INDEX IF NOT EXISTS idx_transfers_destination ON transfers (destination_id);

-- Double-entry ledger: two rows per transfer (debit + credit).
-- balance_after gives a per-account snapshot for audit and replay.
-- 复式记账：每笔转账两行（借记 + 贷记）。
-- balance_after 提供按账户的余额快照，便于审计与回放。
CREATE TABLE IF NOT EXISTS ledger_entries (
    id            BIGSERIAL      PRIMARY KEY,
    transfer_id   BIGINT         NOT NULL REFERENCES transfers(id),
    account_id    BIGINT         NOT NULL REFERENCES accounts(id),
    direction     TEXT           NOT NULL,  -- 'debit' (out) or 'credit' (in) — 借记(出)或贷记(入)
    amount        NUMERIC(38, 18) NOT NULL CHECK (amount > 0),
    balance_after NUMERIC(38, 18) NOT NULL,
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- Account statement lookups, ordered by time.
-- 账户流水查询，按时间排序。
CREATE INDEX IF NOT EXISTS idx_ledger_account  ON ledger_entries (account_id, id);
CREATE INDEX IF NOT EXISTS idx_ledger_transfer ON ledger_entries (transfer_id);
