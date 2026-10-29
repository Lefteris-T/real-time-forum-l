package repository

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"forum/internal/model"
)

func TestMessageRepositoryCreateChecksParticipantsAndUpdatesActivity(t *testing.T) {
	db, conversations, messages, users := newMessagingRepositories(t)
	firstID := addMessagingTestUser(t, users, "message-first")
	secondID := addMessagingTestUser(t, users, "message-second")
	outsiderID := addMessagingTestUser(t, users, "message-outsider")
	conversation, err := conversations.FindOrCreate(firstID, secondID)
	if err != nil {
		t.Fatalf("FindOrCreate(): %v", err)
	}

	message, err := messages.Create(conversation.ID, secondID, "persisted message")
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if message.ID <= 0 || message.ConversationID != conversation.ID || message.SenderID != secondID || message.SenderUsername != "message-second" || message.Body != "persisted message" || message.CreatedAt.IsZero() {
		t.Fatalf("created message = %+v", message)
	}
	updated, err := conversations.FindOrCreate(secondID, firstID)
	if err != nil {
		t.Fatalf("find conversation after message: %v", err)
	}
	if updated.LastMessageAt == nil || !updated.LastMessageAt.Equal(message.CreatedAt) {
		t.Fatalf("last activity = %v, want %v", updated.LastMessageAt, message.CreatedAt)
	}

	var storedBody string
	if err := db.QueryRow(`SELECT body FROM messages WHERE id = ?`, message.ID).Scan(&storedBody); err != nil {
		t.Fatalf("query stored message: %v", err)
	}
	if storedBody != message.Body {
		t.Fatalf("stored body = %q, want %q", storedBody, message.Body)
	}
	if _, err := messages.Create(conversation.ID, outsiderID, "not allowed"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("Create() from outsider error = %v, want conversation not found", err)
	}
	if _, err := messages.Create(conversation.ID+100, firstID, "missing conversation"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("Create() for missing conversation error = %v, want conversation not found", err)
	}
}

func TestMessageRepositoryPaginatesStableTimestampAndIDCursor(t *testing.T) {
	db, conversations, messages, users := newMessagingRepositories(t)
	firstID := addMessagingTestUser(t, users, "history-first")
	secondID := addMessagingTestUser(t, users, "history-second")
	outsiderID := addMessagingTestUser(t, users, "history-outsider")
	conversation, err := conversations.FindOrCreate(firstID, secondID)
	if err != nil {
		t.Fatalf("FindOrCreate(): %v", err)
	}
	const count = 23
	const createdAt = "2026-10-29T10:00:00Z"
	for i := 0; i < count; i++ {
		_, err := db.Exec(`
			INSERT INTO messages (conversation_id, sender_id, body, created_at)
			VALUES (?, ?, ?, ?)
		`, conversation.ID, firstID, fmt.Sprintf("message-%02d", i), createdAt)
		if err != nil {
			t.Fatalf("insert message %d: %v", i, err)
		}
	}

	latest, err := messages.LatestForUser(conversation.ID, secondID)
	if err != nil {
		t.Fatalf("LatestForUser(): %v", err)
	}
	if len(latest) != MessagePageSize {
		t.Fatalf("latest count = %d, want %d", len(latest), MessagePageSize)
	}
	for _, message := range latest {
		if message.SenderUsername != "history-first" {
			t.Fatalf("message sender username = %q, want history-first", message.SenderUsername)
		}
	}
	assertMessageBodies(t, latest, 13, 22)

	cursor := MessageCursor{CreatedAt: latest[0].CreatedAt, ID: latest[0].ID}
	older, err := messages.OlderForUser(conversation.ID, firstID, cursor)
	if err != nil {
		t.Fatalf("OlderForUser(): %v", err)
	}
	if len(older) != MessagePageSize {
		t.Fatalf("older count = %d, want %d", len(older), MessagePageSize)
	}
	assertMessageBodies(t, older, 3, 12)

	oldestCursor := MessageCursor{CreatedAt: older[0].CreatedAt, ID: older[0].ID}
	firstPage, err := messages.OlderForUser(conversation.ID, firstID, oldestCursor)
	if err != nil {
		t.Fatalf("OlderForUser() for final page: %v", err)
	}
	if len(firstPage) != 3 {
		t.Fatalf("final page size = %d, want 3", len(firstPage))
	}
	assertMessageBodies(t, firstPage, 0, 2)

	if _, err := messages.LatestForUser(conversation.ID, outsiderID); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("LatestForUser() for outsider error = %v, want conversation not found", err)
	}
	if _, err := messages.OlderForUser(conversation.ID, outsiderID, cursor); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("OlderForUser() for outsider error = %v, want conversation not found", err)
	}
	if _, err := messages.OlderForUser(conversation.ID, firstID, MessageCursor{}); !errors.Is(err, ErrInvalidMessageCursor) {
		t.Fatalf("OlderForUser() with invalid cursor error = %v, want invalid cursor", err)
	}
}

func TestMessageRepositoryReturnsEmptyHistoryForParticipant(t *testing.T) {
	_, conversations, messages, users := newMessagingRepositories(t)
	firstID := addMessagingTestUser(t, users, "empty-first")
	secondID := addMessagingTestUser(t, users, "empty-second")
	conversation, err := conversations.FindOrCreate(firstID, secondID)
	if err != nil {
		t.Fatalf("FindOrCreate(): %v", err)
	}
	got, err := messages.LatestForUser(conversation.ID, firstID)
	if err != nil {
		t.Fatalf("LatestForUser(): %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty history = %#v, want non-nil empty slice", got)
	}
	if _, err := messages.OlderForUser(conversation.ID, firstID, MessageCursor{
		CreatedAt: time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC), ID: 1,
	}); err != nil {
		t.Fatalf("OlderForUser() empty history: %v", err)
	}
}

func assertMessageBodies(t *testing.T, messages []model.Message, first, last int) {
	t.Helper()
	if len(messages) != last-first+1 {
		t.Fatalf("message count = %d, want %d", len(messages), last-first+1)
	}
	for i, message := range messages {
		want := fmt.Sprintf("message-%02d", first+i)
		if message.Body != want {
			t.Fatalf("messages[%d].Body = %q, want %q", i, message.Body, want)
		}
	}
}
