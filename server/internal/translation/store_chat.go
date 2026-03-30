package translation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *ChatStore) EnsureChatForTranslation(translationID string) (ChatThread, error) {
	for i := 0; i < 8; i++ {
		thread, err := s.ensureChatForTranslationOnce(translationID)
		if err == nil {
			return thread, nil
		}
		if isDBLocked(err) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return ChatThread{}, err
	}
	return ChatThread{}, fmt.Errorf("ensure chat: database remained locked")
}

func (s *ChatStore) AppendChatMessage(translationID string, role string, content string, selectedText string) (ChatMessage, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	if role != ChatRoleUser && role != ChatRoleAI && role != ChatRoleTool {
		return ChatMessage{}, errors.New("role must be user, ai, or tool")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ChatMessage{}, errors.New("content is required")
	}

	for i := 0; i < 8; i++ {
		msg, err := s.appendChatMessageOnce(translationID, role, content, selectedText)
		if err == nil {
			if selectedText != "" {
				msg.SelectedText = &selectedText
			}
			return msg, nil
		}
		if isDBLocked(err) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return ChatMessage{}, err
	}
	return ChatMessage{}, fmt.Errorf("append chat message: database remained locked")
}

func (s *ChatStore) ListChatMessages(translationID string) ([]ChatMessage, error) {
	for i := 0; i < 8; i++ {
		msgs, err := s.listChatMessagesOnce(translationID)
		if err == nil {
			return msgs, nil
		}
		if isDBLocked(err) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("list chat messages: database remained locked")
}

func (s *ChatStore) ClearChatMessages(translationID string) error {
	for i := 0; i < 8; i++ {
		err := s.clearChatMessagesOnce(translationID)
		if err == nil {
			return nil
		}
		if isDBLocked(err) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("clear chat messages: database remained locked")
}

func (s *ChatStore) ensureChatForTranslationOnce(translationID string) (ChatThread, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ChatThread{}, fmt.Errorf("begin ensure chat tx: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM translations WHERE id = ?`, translationID).Scan(&exists); err != nil {
		return ChatThread{}, fmt.Errorf("check translation exists: %w", err)
	}
	if exists == 0 {
		return ChatThread{}, ErrNotFound
	}

	thread, err := loadChatThreadTx(tx, translationID)
	if err == nil {
		return thread, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ChatThread{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	id, err := newID()
	if err != nil {
		return ChatThread{}, fmt.Errorf("new chat id: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO translation_chats (id, translation_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?)`,
		id,
		translationID,
		now,
		now,
	); err != nil {
		return ChatThread{}, fmt.Errorf("insert translation chat: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ChatThread{}, fmt.Errorf("commit ensure chat tx: %w", err)
	}
	return ChatThread{
		ID:            id,
		TranslationID: translationID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *ChatStore) appendChatMessageOnce(translationID string, role string, content string, selectedText string) (ChatMessage, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ChatMessage{}, fmt.Errorf("begin append chat message tx: %w", err)
	}
	defer tx.Rollback()

	thread, err := loadChatThreadTx(tx, translationID)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		chatID, idErr := newID()
		if idErr != nil {
			return ChatMessage{}, fmt.Errorf("new chat id: %w", idErr)
		}
		if _, existsErr := tx.Exec(
			`INSERT INTO translation_chats (id, translation_id, created_at, updated_at)
			 SELECT ?, ?, ?, ?
			 WHERE EXISTS(SELECT 1 FROM translations WHERE id = ?)`,
			chatID,
			translationID,
			now,
			now,
			translationID,
		); existsErr != nil {
			return ChatMessage{}, fmt.Errorf("insert translation chat: %w", existsErr)
		}
		thread, err = loadChatThreadTx(tx, translationID)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatMessage{}, ErrNotFound
		}
		return ChatMessage{}, err
	}

	var maxIdx sql.NullInt64
	if err := tx.QueryRow(
		`SELECT MAX(message_idx) FROM translation_chat_messages WHERE translation_id = ?`,
		translationID,
	).Scan(&maxIdx); err != nil {
		return ChatMessage{}, fmt.Errorf("query max message idx: %w", err)
	}
	nextIdx := 0
	if maxIdx.Valid {
		nextIdx = int(maxIdx.Int64) + 1
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	messageID, err := newID()
	if err != nil {
		return ChatMessage{}, fmt.Errorf("new chat message id: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO translation_chat_messages
		   (id, chat_id, translation_id, message_idx, role, content, selected_text, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		messageID,
		thread.ID,
		translationID,
		nextIdx,
		role,
		content,
		sql.NullString{String: selectedText, Valid: selectedText != ""},
		now,
	); err != nil {
		return ChatMessage{}, fmt.Errorf("insert chat message: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE translation_chats SET updated_at = ? WHERE id = ?`,
		now,
		thread.ID,
	); err != nil {
		return ChatMessage{}, fmt.Errorf("touch chat updated_at: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ChatMessage{}, fmt.Errorf("commit append chat message tx: %w", err)
	}

	return ChatMessage{
		ID:            messageID,
		ChatID:        thread.ID,
		TranslationID: translationID,
		MessageIdx:    nextIdx,
		Role:          role,
		Content:       content,
		CreatedAt:     now,
	}, nil
}

func (s *ChatStore) listChatMessagesOnce(translationID string) ([]ChatMessage, error) {
	thread, err := s.EnsureChatForTranslation(translationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, message_idx, role, content, selected_text, created_at, review_card_json
		 FROM translation_chat_messages
		 WHERE translation_id = ?
		 ORDER BY message_idx ASC`,
		translationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list chat messages query: %w", err)
	}
	defer rows.Close()

	out := make([]ChatMessage, 0)
	for rows.Next() {
		var msg ChatMessage
		var selectedText sql.NullString
		var reviewCardJSON sql.NullString
		if err := rows.Scan(&msg.ID, &msg.MessageIdx, &msg.Role, &msg.Content, &selectedText, &msg.CreatedAt, &reviewCardJSON); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		msg.ChatID = thread.ID
		msg.TranslationID = translationID
		if selectedText.Valid {
			msg.SelectedText = &selectedText.String
		}
		if reviewCardJSON.Valid {
			var card ChatReviewCard
			if err := json.Unmarshal([]byte(reviewCardJSON.String), &card); err != nil {
				return nil, fmt.Errorf("decode review card json: %w", err)
			}
			msg.ReviewCard = &card
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}
	return out, nil
}

func (s *ChatStore) SetReviewCard(messageID, chineseText, pinyin, english string) error {
	card := ChatReviewCard{
		ChineseText: chineseText,
		Pinyin:      pinyin,
		English:     english,
		Status:      "pending",
	}
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal review card: %w", err)
	}
	res, err := s.db.Exec(
		`UPDATE translation_chat_messages SET review_card_json = ? WHERE id = ?`,
		string(cardJSON),
		messageID,
	)
	if err != nil {
		return fmt.Errorf("set review card: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ChatStore) GetMessageReviewCard(messageID string) (*ChatReviewCard, error) {
	var reviewCardJSON sql.NullString
	err := s.db.QueryRow(
		`SELECT review_card_json FROM translation_chat_messages WHERE id = ?`,
		messageID,
	).Scan(&reviewCardJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get message review card: %w", err)
	}
	if !reviewCardJSON.Valid {
		return nil, nil
	}
	var card ChatReviewCard
	if err := json.Unmarshal([]byte(reviewCardJSON.String), &card); err != nil {
		return nil, fmt.Errorf("decode review card json: %w", err)
	}
	return &card, nil
}

func (s *ChatStore) AcceptMessageReviewCard(messageID string) error {
	card, err := s.GetMessageReviewCard(messageID)
	if err != nil {
		return err
	}
	if card == nil {
		return ErrNotFound
	}
	card.Status = "accepted"
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal accepted review card: %w", err)
	}
	_, err = s.db.Exec(
		`UPDATE translation_chat_messages SET review_card_json = ? WHERE id = ?`,
		string(cardJSON),
		messageID,
	)
	return err
}

func (s *ChatStore) RejectMessageReviewCard(messageID string) error {
	// Null the card only. The tool message itself is not rendered when review_card_json is NULL,
	// so no content update is needed (unlike when cards lived on the AI text message).
	_, err := s.db.Exec(
		`UPDATE translation_chat_messages SET review_card_json = NULL WHERE id = ?`,
		messageID,
	)
	return err
}

func (s *ChatStore) clearChatMessagesOnce(translationID string) error {
	thread, err := s.EnsureChatForTranslation(translationID)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin clear chat messages tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`DELETE FROM translation_chat_messages WHERE translation_id = ?`,
		translationID,
	); err != nil {
		return fmt.Errorf("clear chat messages: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE translation_chats SET updated_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		thread.ID,
	); err != nil {
		return fmt.Errorf("touch chat updated_at on clear: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit clear chat messages tx: %w", err)
	}
	return nil
}

func loadChatThreadTx(tx *sql.Tx, translationID string) (ChatThread, error) {
	var thread ChatThread
	row := tx.QueryRow(
		`SELECT id, translation_id, created_at, updated_at
		 FROM translation_chats
		 WHERE translation_id = ?`,
		translationID,
	)
	if err := row.Scan(&thread.ID, &thread.TranslationID, &thread.CreatedAt, &thread.UpdatedAt); err != nil {
		return ChatThread{}, err
	}
	return thread, nil
}
