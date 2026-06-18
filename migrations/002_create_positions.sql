CREATE TABLE positions (
    user_id    BIGINT NOT NULL REFERENCES users(id),
    market_id  BIGINT NOT NULL,
    outcome    TEXT NOT NULL,
    quantity   BIGINT NOT NULL DEFAULT 0,

    PRIMARY KEY (user_id, market_id, outcome),

    CONSTRAINT quantity_non_negative CHECK (quantity >= 0),
    CONSTRAINT outcome_valid CHECK (outcome IN ('YES', 'NO'))
);

CREATE TABLE share_ledger (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    market_id  BIGINT NOT NULL,
    outcome    TEXT NOT NULL,
    amount     BIGINT NOT NULL,
    reason     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_share_ledger_user ON share_ledger(user_id, market_id, outcome);