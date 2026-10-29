package model

import "time"

// Conversation is the durable private chat shared by one canonical user pair.
type Conversation struct {
	ID            int64
	UserLowID     int64
	UserHighID    int64
	LastMessageAt *time.Time
	CreatedAt     time.Time
}
