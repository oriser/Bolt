CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    original_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    db_created_at DATETIME NOT NULL,
    receiver TEXT NOT NULL,
    venue_name TEXT NOT NULL,
    venue_id TEXT NOT NULL,
    venue_link TEXT NULL,
    venue_city TEXT NULL,
    host TEXT NOT NULL,
    host_id TEXT NULL,
    status INTEGER NOT NULL,
    participants JSON NULL,
    delivery_rate INTEGER NULL
);