CREATE TABLE IF NOT EXISTS billables (
    id VARCHAR(255) PRIMARY KEY,
    amount INTEGER,
    principal INTEGER,
    dur_week INTEGER,
    created_at DATETIME,
    due_at DATETIME
);

CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(255) PRIMARY KEY,
    billable_id VARCHAR(255),
    amount INTEGER,
    amount_accumulated INTEGER,
    paid_at DATETIME,
    created_at DATETIME,
    FOREIGN KEY (billable_id) REFERENCES billables(id)
);

CREATE INDEX IF NOT EXISTS idx_billable_id ON payments (billable_id);
CREATE INDEX IF NOT EXISTS idx_payment_billable_id_paid_at_desc ON payments (billable_id, paid_at DESC);