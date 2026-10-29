package repository

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"forum/internal/database"
)

func TestConversationRepositoryFindOrCreateCanonicalizesPair(t *testing.T) {
	db, conversations, _, users := newMessagingRepositories(t)
	firstID := addMessagingTestUser(t, users, "first")
	secondID := addMessagingTestUser(t, users, "second")

	first, err := conversations.FindOrCreate(firstID, secondID)
	if err != nil {
		t.Fatalf("FindOrCreate(): %v", err)
	}
	second, err := conversations.FindOrCreate(secondID, firstID)
	if err != nil {
		t.Fatalf("FindOrCreate() with reversed users: %v", err)
	}
	if first.ID != second.ID || first.UserLowID != firstID || first.UserHighID != secondID {
		t.Fatalf("conversation pair = %+v / %+v", first, second)
	}
	if first.LastMessageAt != nil {
		t.Fatalf("LastMessageAt = %v, want nil before first message", first.LastMessageAt)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&count); err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if count != 1 {
		t.Fatalf("conversation count = %d, want 1", count)
	}

	if _, err := conversations.FindOrCreate(firstID, firstID); !errors.Is(err, ErrInvalidConversationPair) {
		t.Fatalf("FindOrCreate(self) error = %v, want invalid pair", err)
	}
	if _, err := conversations.FindOrCreate(firstID, 0); !errors.Is(err, ErrInvalidConversationPair) {
		t.Fatalf("FindOrCreate(invalid user) error = %v, want invalid pair", err)
	}
	if _, err := conversations.FindOrCreate(firstID, secondID+100); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FindOrCreate(missing user) error = %v, want user not found", err)
	}
}

func TestConversationRepositoryOrdersContactsByActivityThenAlphabetically(t *testing.T) {
	db, conversations, messages, users := newMessagingRepositories(t)
	currentID := addMessagingTestUser(t, users, "current")
	dianeID := addMessagingTestUser(t, users, "diane")
	benID := addMessagingTestUser(t, users, "ben")
	aliceID := addMessagingTestUser(t, users, "alice")
	caraID := addMessagingTestUser(t, users, "cara")

	dianeConversation, err := conversations.FindOrCreate(currentID, dianeID)
	if err != nil {
		t.Fatalf("create Diane conversation: %v", err)
	}
	benConversation, err := conversations.FindOrCreate(currentID, benID)
	if err != nil {
		t.Fatalf("create Ben conversation: %v", err)
	}
	caraConversation, err := conversations.FindOrCreate(currentID, caraID)
	if err != nil {
		t.Fatalf("create Cara conversation: %v", err)
	}
	if _, err := messages.Create(dianeConversation.ID, dianeID, "hello"); err != nil {
		t.Fatalf("create Diane message: %v", err)
	}
	if _, err := messages.Create(benConversation.ID, currentID, "hello"); err != nil {
		t.Fatalf("create Ben message: %v", err)
	}

	// Pin distinct activity values so the ordering assertion is independent of
	// test execution speed and the database clock's second precision.
	for conversationID, timestamp := range map[int64]string{
		dianeConversation.ID: "2026-10-28T12:00:00Z",
		benConversation.ID:   "2026-10-27T12:00:00Z",
	} {
		if _, err := db.Exec(`UPDATE conversations SET last_message_at = ? WHERE id = ?`, timestamp, conversationID); err != nil {
			t.Fatalf("set activity timestamp: %v", err)
		}
	}

	contacts, err := conversations.ListContacts(currentID)
	if err != nil {
		t.Fatalf("ListContacts(): %v", err)
	}
	wantNames := []string{"diane", "ben", "alice", "cara"}
	if len(contacts) != len(wantNames) {
		t.Fatalf("contact count = %d, want %d", len(contacts), len(wantNames))
	}
	for i, want := range wantNames {
		if contacts[i].Username != want {
			t.Fatalf("contacts[%d].Username = %q, want %q", i, contacts[i].Username, want)
		}
	}
	if contacts[0].ConversationID != dianeConversation.ID || contacts[0].LastMessageAt == nil ||
		!contacts[0].LastMessageAt.Equal(time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("Diane contact activity = %+v", contacts[0])
	}
	if contacts[2].UserID != aliceID || contacts[2].ConversationID != 0 || contacts[2].LastMessageAt != nil {
		t.Fatalf("Alice contact = %+v", contacts[2])
	}
	if contacts[3].UserID != caraID || contacts[3].ConversationID != caraConversation.ID || contacts[3].LastMessageAt != nil {
		t.Fatalf("Cara contact = %+v", contacts[3])
	}
}

func TestMessageReadReceiptsPersistUnreadCountsAndMarkRead(t *testing.T) {
	_, conversations, messages, users := newMessagingRepositories(t)
	senderID := addMessagingTestUser(t, users, "receipt-sender")
	recipientID := addMessagingTestUser(t, users, "receipt-recipient")
	outsiderID := addMessagingTestUser(t, users, "receipt-outsider")
	conversation, err := conversations.FindOrCreate(senderID, recipientID)
	if err != nil {
		t.Fatalf("FindOrCreate(): %v", err)
	}
	if _, err := messages.Create(conversation.ID, senderID, "offline message"); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	contacts, err := conversations.ListContacts(recipientID)
	if err != nil {
		t.Fatalf("ListContacts(): %v", err)
	}
	if len(contacts) != 2 || contacts[0].UserID != senderID || contacts[0].UnreadCount != 1 {
		t.Fatalf("unread contacts = %+v", contacts)
	}
	if err := messages.MarkReadForUser(conversation.ID, recipientID); err != nil {
		t.Fatalf("MarkReadForUser(): %v", err)
	}
	if err := messages.MarkReadForUser(conversation.ID, recipientID); err != nil {
		t.Fatalf("repeated MarkReadForUser(): %v", err)
	}
	contacts, err = conversations.ListContacts(recipientID)
	if err != nil || contacts[0].UnreadCount != 0 {
		t.Fatalf("contacts after mark read = %+v, error %v", contacts, err)
	}
	if err := messages.MarkReadForUser(conversation.ID, outsiderID); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("outsider MarkReadForUser() error = %v, want conversation not found", err)
	}
}

func newMessagingRepositories(t *testing.T) (*sql.DB, *ConversationRepository, *MessageRepository, *UserRepository) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("database.Migrate(): %v", err)
	}
	return db, NewConversationRepository(db), NewMessageRepository(db), NewUserRepository(db)
}

func addMessagingTestUser(t *testing.T, users *UserRepository, username string) int64 {
	t.Helper()
	id, err := insertTestUser(users, username+"@example.com", username, "password-hash")
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	return id
}
