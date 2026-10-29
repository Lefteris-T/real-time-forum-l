package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestConversationSchemaConstraintsIndexesAndMessageOrder(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	userIDs := make([]int64, 3)
	for i, name := range []string{"conversation-a", "conversation-b", "conversation-c"} {
		result, err := db.Exec(`
			INSERT INTO users (
				email, username, password_hash, created_at,
				age, gender, first_name, last_name
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, name+"@example.com", name, "hash", "2026-10-29T10:00:00Z", 30, "test", "Test", name)
		if err != nil {
			t.Fatalf("insert user %q: %v", name, err)
		}
		userIDs[i], err = result.LastInsertId()
		if err != nil {
			t.Fatalf("get user ID for %q: %v", name, err)
		}
	}

	conversationID := insertConversation(t, db, userIDs[0], userIDs[1], "2026-10-29T10:01:00Z")
	if _, err := db.Exec(`
		INSERT INTO conversations (user_low_id, user_high_id, created_at)
		VALUES (?, ?, ?)
	`, userIDs[0], userIDs[1], "2026-10-29T10:02:00Z"); err == nil {
		t.Fatal("duplicate user pair conversation was accepted")
	}
	if _, err := db.Exec(`
		INSERT INTO conversations (user_low_id, user_high_id, created_at)
		VALUES (?, ?, ?)
	`, userIDs[1], userIDs[1], "2026-10-29T10:03:00Z"); err == nil {
		t.Fatal("self conversation was accepted")
	}
	if _, err := db.Exec(`
		INSERT INTO conversations (user_low_id, user_high_id, created_at)
		VALUES (?, ?, ?)
	`, userIDs[1], userIDs[0], "2026-10-29T10:03:30Z"); err == nil {
		t.Fatal("noncanonical user pair ordering was accepted")
	}
	if _, err := db.Exec(`
		INSERT INTO conversations (user_low_id, user_high_id, created_at)
		VALUES (?, ?, ?)
	`, userIDs[0], userIDs[2], "2026-10-29T10:04:00Z"); err != nil {
		t.Fatalf("insert another valid conversation: %v", err)
	}

	for _, body := range []string{"First", "Second", "Third"} {
		if _, err := db.Exec(`
			INSERT INTO messages (conversation_id, sender_id, body, created_at)
			VALUES (?, ?, ?, ?)
		`, conversationID, userIDs[0], body, "2026-10-29T10:05:00Z"); err != nil {
			t.Fatalf("insert valid message %q: %v", body, err)
		}
	}
	if _, err := db.Exec(`
		INSERT INTO messages (conversation_id, sender_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`, conversationID+999, userIDs[0], "Missing conversation", "2026-10-29T10:06:00Z"); err == nil {
		t.Fatal("message with nonexistent conversation was accepted")
	}
	if _, err := db.Exec(`
		INSERT INTO messages (conversation_id, sender_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`, conversationID, userIDs[2]+999, "Missing sender", "2026-10-29T10:07:00Z"); err == nil {
		t.Fatal("message with nonexistent sender was accepted")
	}

	rows, err := db.Query(`
		SELECT body
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at ASC, id ASC
	`, conversationID)
	if err != nil {
		t.Fatalf("query message history: %v", err)
	}
	defer rows.Close()
	want := []string{"First", "Second", "Third"}
	for i := range want {
		if !rows.Next() {
			t.Fatalf("message history ended before item %d", i)
		}
		var body string
		if err := rows.Scan(&body); err != nil {
			t.Fatalf("scan message history: %v", err)
		}
		if body != want[i] {
			t.Fatalf("message %d = %q, want %q", i, body, want[i])
		}
	}
	if rows.Next() {
		t.Fatal("message history contains unexpected extra rows")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate message history: %v", err)
	}

	assertIndexExists(t, db, "conversations", "idx_conversations_activity")
	assertIndexColumns(t, db, "idx_conversations_activity", []string{"last_message_at", "id"})
	assertIndexExists(t, db, "conversations", "idx_conversations_user_high")
	assertIndexColumns(t, db, "idx_conversations_user_high", []string{"user_high_id", "user_low_id"})
	assertIndexExists(t, db, "messages", "idx_messages_conversation_history")
	assertIndexColumns(t, db, "idx_messages_conversation_history", []string{"conversation_id", "created_at", "id"})
}

func insertConversation(t *testing.T, db *sql.DB, lowID, highID int64, createdAt string) int64 {
	t.Helper()
	result, err := db.Exec(`
		INSERT INTO conversations (user_low_id, user_high_id, created_at)
		VALUES (?, ?, ?)
	`, lowID, highID, createdAt)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get conversation ID: %v", err)
	}
	return id
}
