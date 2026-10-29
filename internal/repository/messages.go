package repository

import (
	"database/sql"
	"errors"
	"time"

	"forum/internal/model"
)

const MessagePageSize = 10

var ErrInvalidMessageCursor = errors.New("message cursor is invalid")

// MessageCursor identifies one message in the stable (created_at, id) order.
type MessageCursor struct {
	CreatedAt time.Time
	ID        int64
}

// MessageRepository owns message persistence and participant-scoped history.
type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create persists a message and advances conversation activity in the same
// transaction. Nonparticipants receive the same not-found result as an
// unknown conversation so the repository does not reveal private pairs.
func (r *MessageRepository) Create(conversationID, senderID int64, body string) (model.Message, error) {
	if conversationID <= 0 || senderID <= 0 {
		return model.Message{}, ErrConversationNotFound
	}
	tx, err := r.db.Begin()
	if err != nil {
		return model.Message{}, err
	}
	defer tx.Rollback()

	if err := participantExists(tx, conversationID, senderID); err != nil {
		return model.Message{}, err
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	createdAtValue := createdAt.Format(time.RFC3339)
	result, err := tx.Exec(`
		INSERT INTO messages (conversation_id, sender_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`, conversationID, senderID, body, createdAtValue)
	if err != nil {
		return model.Message{}, err
	}
	messageID, err := result.LastInsertId()
	if err != nil {
		return model.Message{}, err
	}
	if _, err := tx.Exec(`
		UPDATE conversations
		SET last_message_at = ?
		WHERE id = ?
	`, createdAtValue, conversationID); err != nil {
		return model.Message{}, err
	}
	var senderUsername string
	if err := tx.QueryRow("SELECT username FROM users WHERE id = ?", senderID).Scan(&senderUsername); err != nil {
		return model.Message{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, err
	}

	return model.Message{
		ID:             messageID,
		ConversationID: conversationID,
		SenderID:       senderID,
		SenderUsername: senderUsername,
		Body:           body,
		CreatedAt:      createdAt,
	}, nil
}

// LatestForUser returns the latest ten messages in chronological display
// order, and only when userID belongs to the conversation.
func (r *MessageRepository) LatestForUser(conversationID, userID int64) ([]model.Message, error) {
	if err := validateConversationUser(conversationID, userID); err != nil {
		return nil, err
	}
	if err := participantExists(r.db, conversationID, userID); err != nil {
		return nil, err
	}
	return r.list(`
		SELECT m.id, m.conversation_id, m.sender_id, u.username, m.body, m.created_at
		FROM (
			SELECT id, conversation_id, sender_id, body, created_at
			FROM messages
			WHERE conversation_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		) m
		JOIN users u ON u.id = m.sender_id
		ORDER BY m.created_at ASC, m.id ASC
	`, conversationID, MessagePageSize)
}

// OlderForUser returns up to ten messages strictly older than before, using
// (created_at, id) as a stable cursor and returning them chronologically.
func (r *MessageRepository) OlderForUser(conversationID, userID int64, before MessageCursor) ([]model.Message, error) {
	if err := validateConversationUser(conversationID, userID); err != nil {
		return nil, err
	}
	if before.ID <= 0 || before.CreatedAt.IsZero() {
		return nil, ErrInvalidMessageCursor
	}
	if err := participantExists(r.db, conversationID, userID); err != nil {
		return nil, err
	}
	createdAt := before.CreatedAt.UTC().Format(time.RFC3339)
	return r.list(`
		SELECT m.id, m.conversation_id, m.sender_id, u.username, m.body, m.created_at
		FROM (
			SELECT id, conversation_id, sender_id, body, created_at
			FROM messages
			WHERE conversation_id = ?
			  AND (created_at < ? OR (created_at = ? AND id < ?))
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		) m
		JOIN users u ON u.id = m.sender_id
		ORDER BY m.created_at ASC, m.id ASC
	`, conversationID, createdAt, createdAt, before.ID, MessagePageSize)
}

type conversationParticipantQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func participantExists(db conversationParticipantQuerier, conversationID, userID int64) error {
	var exists int
	err := db.QueryRow(`
		SELECT 1
		FROM conversations
		WHERE id = ? AND (user_low_id = ? OR user_high_id = ?)
	`, conversationID, userID, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationNotFound
	}
	return err
}

func validateConversationUser(conversationID, userID int64) error {
	if conversationID <= 0 || userID <= 0 {
		return ErrConversationNotFound
	}
	return nil
}

func (r *MessageRepository) list(query string, args ...any) ([]model.Message, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]model.Message, 0, MessagePageSize)
	for rows.Next() {
		var message model.Message
		var createdAt string
		if err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.SenderID,
			&message.SenderUsername,
			&message.Body,
			&createdAt,
		); err != nil {
			return nil, err
		}
		message.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}
