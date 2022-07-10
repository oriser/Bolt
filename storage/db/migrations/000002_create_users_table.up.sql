CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    full_name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT NULL,
    timezone TEXT NOT NULL,
    transport_id TEXT NOT NULL,
    created_at DATETIME NOT NULL
);