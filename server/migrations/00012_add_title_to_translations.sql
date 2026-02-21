-- +goose Up
ALTER TABLE translations ADD COLUMN title TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN easily; no-op
