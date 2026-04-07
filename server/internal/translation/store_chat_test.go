package translation

import (
	"path/filepath"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
)

func newChatStoreWithMigrations(t *testing.T) (*TranslationStore, *ChatStore) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "translations.db")
	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := migrations.RunUp(dbPath, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	return NewTranslationStore(db), NewChatStore(db)
}

func TestChatThreadAndMessagesLifecycle(t *testing.T) {
	ts, cs := newChatStoreWithMigrations(t)
	tr, err := ts.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	threadA, err := cs.EnsureChatForTranslation(tr.ID)
	if err != nil {
		t.Fatalf("ensure chat first call: %v", err)
	}
	threadB, err := cs.EnsureChatForTranslation(tr.ID)
	if err != nil {
		t.Fatalf("ensure chat second call: %v", err)
	}
	if threadA.ID != threadB.ID {
		t.Fatalf("expected one chat per translation, got %q and %q", threadA.ID, threadB.ID)
	}

	userMsg, err := cs.AppendChatMessage(tr.ID, ChatRoleUser, "What does this mean?", "")
	if err != nil {
		t.Fatalf("append user message: %v", err)
	}
	aiMsg, err := cs.AppendChatMessage(tr.ID, ChatRoleAI, "It means hello world.", "")
	if err != nil {
		t.Fatalf("append ai message: %v", err)
	}
	if userMsg.MessageIdx != 0 || aiMsg.MessageIdx != 1 {
		t.Fatalf("expected message order 0,1 got %d,%d", userMsg.MessageIdx, aiMsg.MessageIdx)
	}

	msgs, err := cs.ListChatMessages(tr.ID)
	if err != nil {
		t.Fatalf("list chat messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != ChatRoleUser || msgs[1].Role != ChatRoleAI {
		t.Fatalf("unexpected roles: %#v", msgs)
	}
}

func TestClearChatMessages(t *testing.T) {
	ts, cs := newChatStoreWithMigrations(t)
	tr, err := ts.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}
	if _, err := cs.AppendChatMessage(tr.ID, ChatRoleUser, "test", ""); err != nil {
		t.Fatalf("append message: %v", err)
	}
	if err := cs.ClearChatMessages(tr.ID); err != nil {
		t.Fatalf("clear chat messages: %v", err)
	}

	msgs, err := cs.ListChatMessages(tr.ID)
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no messages after clear, got %d", len(msgs))
	}
}
