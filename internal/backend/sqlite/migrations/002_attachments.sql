-- +goose Up

CREATE TABLE IF NOT EXISTS attachments_articles_mapping (
    article_id INTEGER REFERENCES articles(id),
    content_type TEXT NOT NULL,
    attachment_id TEXT NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS attachments_articles_mapping;