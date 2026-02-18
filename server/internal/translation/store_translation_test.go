package translation

import (
	"path/filepath"
	"testing"

	"github.com/anath2/language-app/internal/migrations"
)

func newTranslationStoreWithMigrations(t *testing.T) *TranslationStore {
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
	return NewTranslationStore(db)
}

func TestChatThreadAndMessagesLifecycle(t *testing.T) {
	store := newTranslationStoreWithMigrations(t)
	tr, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}

	threadA, err := store.EnsureChatForTranslation(tr.ID)
	if err != nil {
		t.Fatalf("ensure chat first call: %v", err)
	}
	threadB, err := store.EnsureChatForTranslation(tr.ID)
	if err != nil {
		t.Fatalf("ensure chat second call: %v", err)
	}
	if threadA.ID != threadB.ID {
		t.Fatalf("expected one chat per translation, got %q and %q", threadA.ID, threadB.ID)
	}

	userMsg, err := store.AppendChatMessage(tr.ID, ChatRoleUser, "What does this mean?", nil)
	if err != nil {
		t.Fatalf("append user message: %v", err)
	}
	aiMsg, err := store.AppendChatMessage(tr.ID, ChatRoleAI, "It means hello world.", nil)
	if err != nil {
		t.Fatalf("append ai message: %v", err)
	}
	if userMsg.MessageIdx != 0 || aiMsg.MessageIdx != 1 {
		t.Fatalf("expected message order 0,1 got %d,%d", userMsg.MessageIdx, aiMsg.MessageIdx)
	}

	msgs, err := store.ListChatMessages(tr.ID)
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

func TestLoadSelectedSegmentsByIDsPreservesOrder(t *testing.T) {
	store := newTranslationStoreWithMigrations(t)
	tr, err := store.Create("你好世界", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}
	err = store.UpdateTranslationSegments(tr.ID, 0, []SegmentResult{
		{Segment: "你好", Pinyin: "ni hao", English: "hello"},
		{Segment: "世界", Pinyin: "shi jie", English: "world"},
	})
	if err != nil {
		t.Fatalf("update translation segments: %v", err)
	}

	secondID := tr.ID + ":0:1"
	firstID := tr.ID + ":0:0"
	selected, err := store.LoadSelectedSegmentsByIDs(tr.ID, []string{secondID, firstID})
	if err != nil {
		t.Fatalf("load selected segments: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected segments, got %d", len(selected))
	}
	if selected[0].Segment != "世界" || selected[1].Segment != "你好" {
		t.Fatalf("selected order mismatch: %#v", selected)
	}
}

func TestClearChatMessages(t *testing.T) {
	store := newTranslationStoreWithMigrations(t)
	tr, err := store.Create("你好", "text")
	if err != nil {
		t.Fatalf("create translation: %v", err)
	}
	if _, err := store.AppendChatMessage(tr.ID, ChatRoleUser, "test", nil); err != nil {
		t.Fatalf("append message: %v", err)
	}
	if err := store.ClearChatMessages(tr.ID); err != nil {
		t.Fatalf("clear chat messages: %v", err)
	}

	msgs, err := store.ListChatMessages(tr.ID)
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no messages after clear, got %d", len(msgs))
	}
}
