package repository

import (
	"database/sql"
	"errors"
	"time"

	"forum/internal/model"
)

const MessagePageSize = 10

var (
	ErrInvalidMessageCursor = errors.New("message cursor is invalid")
	ErrMessageImageNotFound = errors.New("message image not found")
)

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
	return r.create(conversationID, senderID, body, nil)
}

func (r *MessageRepository) CreateWithImage(conversationID, senderID int64, body string, image model.MessageImage) (model.Message, error) {
	return r.create(conversationID, senderID, body, &image)
}

func (r *MessageRepository) create(conversationID, senderID int64, body string, image *model.MessageImage) (model.Message, error) {
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
	if image != nil {
		if _, err := tx.Exec(`
			INSERT INTO message_images (message_id, mime_type, image_data, byte_size)
			VALUES (?, ?, ?, ?)
		`, messageID, image.MIMEType, image.Data, len(image.Data)); err != nil {
			return model.Message{}, err
		}
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
		ImageMIME:      imageMIME(image),
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
		SELECT m.id, m.conversation_id, m.sender_id, u.username, m.body, COALESCE(mi.mime_type, ''), m.created_at
		FROM (
			SELECT id, conversation_id, sender_id, body, created_at
			FROM messages
			WHERE conversation_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		) m
		JOIN users u ON u.id = m.sender_id
		LEFT JOIN message_images mi ON mi.message_id = m.id
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
		SELECT m.id, m.conversation_id, m.sender_id, u.username, m.body, COALESCE(mi.mime_type, ''), m.created_at
		FROM (
			SELECT id, conversation_id, sender_id, body, created_at
			FROM messages
			WHERE conversation_id = ?
			  AND (created_at < ? OR (created_at = ? AND id < ?))
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		) m
		JOIN users u ON u.id = m.sender_id
		LEFT JOIN message_images mi ON mi.message_id = m.id
		ORDER BY m.created_at ASC, m.id ASC
	`, conversationID, createdAt, createdAt, before.ID, MessagePageSize)
}

func imageMIME(image *model.MessageImage) string {
	if image == nil {
		return ""
	}
	return image.MIMEType
}

// ImageForUser returns attachment bytes only when userID participates in the
// message's conversation; missing messages and nonparticipants are identical.
func (r *MessageRepository) ImageForUser(messageID, userID int64) (model.MessageImage, error) {
	if messageID <= 0 || userID <= 0 {
		return model.MessageImage{}, ErrMessageImageNotFound
	}
	var image model.MessageImage
	err := r.db.QueryRow(`
		SELECT mi.mime_type, mi.image_data
		FROM message_images mi
		JOIN messages m ON m.id = mi.message_id
		JOIN conversations c ON c.id = m.conversation_id
		WHERE mi.message_id = ? AND (c.user_low_id = ? OR c.user_high_id = ?)
	`, messageID, userID, userID).Scan(&image.MIMEType, &image.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MessageImage{}, ErrMessageImageNotFound
	}
	if err != nil {
		return model.MessageImage{}, err
	}
	return image, nil
}

// MarkReadForUser records read receipts for all messages sent by the other
// conversation participant. Repeating the request is safe.
func (r *MessageRepository) MarkReadForUser(conversationID, userID int64) error {
	if err := validateConversationUser(conversationID, userID); err != nil {
		return err
	}
	if err := participantExists(r.db, conversationID, userID); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT OR IGNORE INTO message_reads (message_id, user_id, read_at)
		SELECT id, ?, ?
		FROM messages
		WHERE conversation_id = ? AND sender_id != ?
	`, userID, time.Now().UTC().Format(time.RFC3339), conversationID, userID)
	return err
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
			&message.ImageMIME,
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
