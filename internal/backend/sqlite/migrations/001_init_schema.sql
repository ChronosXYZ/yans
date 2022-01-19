-- +goose Up

CREATE TABLE IF NOT EXISTS groups(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at UNSIGNED BIG INT NOT NULL
);
CREATE TABLE IF NOT EXISTS articles(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date INTEGER NOT NULL,
    path TEXT,
    reply_to TEXT,
    thread TEXT,
    subject TEXT NOT NULL,
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