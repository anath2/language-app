-- +goose Up
ALTER TABLE translation_paragraphs RENAME TO translation_sentences;
ALTER TABLE translation_sentences RENAME COLUMN paragraph_idx TO sentence_idx;
ALTER TABLE translation_segments RENAME COLUMN paragraph_idx TO sentence_idx;

-- +goose Down
ALTER TABLE translation_sentences RENAME TO translation_paragraphs;
ALTER TABLE translation_sentences RENAME COLUMN sentence_idx TO paragraph_idx;
ALTER TABLE translation_segments RENAME COLUMN sentence_idx TO paragraph_idx;
