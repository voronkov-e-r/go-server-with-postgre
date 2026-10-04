-- +goose Up
CREATE TABLE books (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    year INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'available',
    tags TEXT[] NOT NULL DEFAULT '{}',
    publisher JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_books_author ON books(author);
CREATE INDEX idx_books_status ON books(status);
CREATE INDEX idx_books_tags ON books USING GIN(tags);

-- +goose Down
DROP TABLE books;