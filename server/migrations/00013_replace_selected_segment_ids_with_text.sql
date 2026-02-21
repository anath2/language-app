-- +goose Up
ALTER TABLE translation_chat_messages DROP COLUMN selected_segment_ids_json;
ALTER TABLE translation_chat_messages ADD COLUMN selected_text TEXT;

-- +goose Down
ALTER TABLE translation_chat_messages DROP COLUMN selected_text;
ALTER TABLE translation_chat_messages ADD COLUMN selected_segment_ids_json TEXT NOT NULL DEFAULT '[]';
