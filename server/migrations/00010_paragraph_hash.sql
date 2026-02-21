-- +goose Up
ALTER TABLE translation_paragraphs ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite doesn't support DROP COLUMN in older versions; no-op
