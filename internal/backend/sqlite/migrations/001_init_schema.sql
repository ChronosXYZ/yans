-- +goose Up

CREATE TABLE IF NOT EXISTS groups(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS articles(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    header TEXT,
    thread TEXT,
    body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS articles_to_groups(
    article_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE groups;
DROP TABLE articles;
DROP TABLE articles_to_groups;