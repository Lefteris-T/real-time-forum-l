package repository

import (
	"database/sql"
	"errors"
	"time"

	"forum/internal/model"
)

var (
	ErrInvalidConversationPair = errors.New("conversation requires two different users")
	ErrConversationNotFound    = errors.New("conversation not found")
)

// ConversationContact contains only the public identity and activity data
// needed to render one row in the contact list. A zero ConversationID means
// that no conversation exists yet for this user pair.
type ConversationContact struct {
	UserID         int64
	Username       string
	ConversationID int64
	LastMessageAt  *time.Time
}

// ConversationRepository owns conversation persistence and contact ordering.
type ConversationRepository struct {
	db *sql.DB
}

func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// FindOrCreate returns the unique conversation for a pair, sorting IDs before
// persistence so calls in either user order resolve to the same row.
func (r *ConversationRepository) FindOrCreate(userA, userB int64) (model.Conversation, error) {
	if userA <= 0 || userB <= 0 || userA == userB {
		return model.Conversation{}, ErrInvalidConversationPair
	}
	lowID, highID := userA, userB
	if lowID > highID {
		lowID, highID = highID, lowID
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO conversations (user_low_id, user_high_id, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT (user_low_id, user_high_id) DO NOTHING
	`, lowID, highID, createdAt)
	if err != nil {
		// Distinguish a missing participant from an unexpected database failure.
		var count int
		lookupErr := r.db.QueryRow(`
			SELECT COUNT(*) FROM users WHERE id IN (?, ?)
		`, lowID, highID).Scan(&count)
		if lookupErr == nil && count != 2 {
			return model.Conversation{}, ErrUserNotFound
		}
		return model.Conversation{}, err
	}

	return r.byPair(lowID, highID)
}

func (r *ConversationRepository) byPair(lowID, highID int64) (model.Conversation, error) {
	var conversation model.Conversation
	var createdAt string
	var lastMessageAt sql.NullString
	err := r.db.QueryRow(`
		SELECT id, user_low_id, user_high_id, last_message_at, created_at
		FROM conversations
		WHERE user_low_id = ? AND user_high_id = ?
	`, lowID, highID).Scan(
		&conversation.ID,
		&conversation.UserLowID,
		&conversation.UserHighID,
		&lastMessageAt,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Conversation{}, ErrConversationNotFound
	}
	if err != nil {
		return model.Conversation{}, err
	}
	conversation.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return model.Conversation{}, err
	}
	if lastMessageAt.Valid {
		parsed, parseErr := time.Parse(time.RFC3339, lastMessageAt.String)
		if parseErr != nil {
			return model.Conversation{}, parseErr
		}
		conversation.LastMessageAt = &parsed
	}
	return conversation, nil
}

// ListContacts returns every other account, ordering users with message
// activity newest first and users without message activity alphabetically.
func (r *ConversationRepository) ListContacts(userID int64) ([]ConversationContact, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	var exists int
	if err := r.db.QueryRow("SELECT 1 FROM users WHERE id = ?", userID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT
			u.id,
			u.username,
			COALESCE(c.id, 0),
			c.last_message_at
		FROM users u
		LEFT JOIN conversations c
			ON (c.user_low_id = ? AND c.user_high_id = u.id)
			OR (c.user_high_id = ? AND c.user_low_id = u.id)
		WHERE u.id != ?
		ORDER BY
			CASE WHEN c.last_message_at IS NULL THEN 1 ELSE 0 END ASC,
			c.last_message_at DESC,
			LOWER(u.username) ASC,
			u.id ASC
	`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]ConversationContact, 0)
	for rows.Next() {
		var contact ConversationContact
		var lastMessageAt sql.NullString
		if err := rows.Scan(&contact.UserID, &contact.Username, &contact.ConversationID, &lastMessageAt); err != nil {
			return nil, err
		}
		if lastMessageAt.Valid {
			parsed, err := time.Parse(time.RFC3339, lastMessageAt.String)
			if err != nil {
				return nil, err
			}
			contact.LastMessageAt = &parsed
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return contacts, nil
}
