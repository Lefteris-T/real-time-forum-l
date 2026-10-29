package model

import "time"

// Message is a persisted message in a private conversation.
type Message struct {
	ID             int64
	ConversationID int64
	SenderID       int64
	SenderUsername string
	Body           string
	ImageMIME      string
	CreatedAt      time.Time
}

// MessageImage is validated image content attached to one private message.
type MessageImage struct {
	MIMEType string
	Data     []byte
}
