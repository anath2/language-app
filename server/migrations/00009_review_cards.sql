-- +goose Up
ALTER TABLE translation_chat_messages ADD COLUMN review_card_json TEXT NULL;

-- +goose Down
SELECT 1; -- SQLite <3.35 has no DROP COLUMN
