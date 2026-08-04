CREATE TABLE IF NOT EXISTS system_user (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL,
    password TEXT,
    role TEXT DEFAULT 'admin',
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS system_config (
    id INTEGER PRIMARY KEY,
    "key" TEXT NOT NULL,
    value TEXT,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sonarr_title (
    id INTEGER PRIMARY KEY,
    tvdb_id INTEGER NOT NULL,
    sno INTEGER NOT NULL DEFAULT 0,
    main_title TEXT NOT NULL,
    title TEXT NOT NULL,
    clean_title TEXT NOT NULL,
    season_number INTEGER NOT NULL DEFAULT 1,
    monitored INTEGER NOT NULL DEFAULT 1,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    series_id INTEGER
);
CREATE INDEX IF NOT EXISTS sonarr_title_tvdb_id_idx ON sonarr_title (tvdb_id);
CREATE INDEX IF NOT EXISTS sonarr_title_clean_title_idx ON sonarr_title (clean_title);
CREATE TABLE IF NOT EXISTS radarr_title (
    id INTEGER PRIMARY KEY,
    tmdb_id INTEGER NOT NULL,
    sno INTEGER NOT NULL DEFAULT 0,
    main_title TEXT NOT NULL,
    title TEXT NOT NULL,
    clean_title TEXT NOT NULL,
    year INTEGER NOT NULL,
    monitored INTEGER NOT NULL DEFAULT 1,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    movie_id INTEGER
);
CREATE TABLE IF NOT EXISTS tmdb_title (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tvdb_id INTEGER NOT NULL,
    tmdb_id INTEGER,
    language TEXT NOT NULL,
    title TEXT NOT NULL,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS tmdb_title_tvdb_id_idx ON tmdb_title (tvdb_id);
CREATE INDEX IF NOT EXISTS tmdb_title_tmdb_id_idx ON tmdb_title (tmdb_id);
CREATE TABLE IF NOT EXISTS sonarr_rule (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 1000,
    regex TEXT NOT NULL,
    replacement TEXT NOT NULL DEFAULT '',
    offset INTEGER NOT NULL DEFAULT 0,
    example TEXT NOT NULL DEFAULT '',
    remark TEXT,
    author TEXT,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS radarr_rule (
    id TEXT PRIMARY KEY,
    token TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 1000,
    regex TEXT NOT NULL,
    replacement TEXT NOT NULL DEFAULT '',
    offset INTEGER NOT NULL DEFAULT 0,
    example TEXT NOT NULL DEFAULT '',
    remark TEXT,
    author TEXT,
    valid_status INTEGER NOT NULL DEFAULT 1,
    create_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME DEFAULT CURRENT_TIMESTAMP
);
