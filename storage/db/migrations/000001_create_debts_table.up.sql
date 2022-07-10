CREATE TABLE IF NOT EXISTS debts (
    id TEXT PRIMARY KEY,
    borrower_id TEXT NOT NULL,
    lender_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    amount REAL NOT NULL,
    initial_transport TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    created_at DATETIME NOT NULL
);