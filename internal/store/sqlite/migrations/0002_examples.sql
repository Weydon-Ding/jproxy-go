CREATE TABLE IF NOT EXISTS sonarr_example (
    hash TEXT PRIMARY KEY,
    original_text TEXT NOT NULL,
    format_text TEXT,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS radarr_example (
    hash TEXT PRIMARY KEY,
    original_text TEXT NOT NULL,
    format_text TEXT,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
