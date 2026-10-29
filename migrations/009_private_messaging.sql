-- Store one private conversation for each unordered user pair. Sorting user
-- IDs before insertion gives every pair one canonical representation.
CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_low_id INTEGER NOT NULL,
    user_high_id INTEGER NOT NULL,
    last_message_at TEXT,
    created_at TEXT NOT NULL,

    CHECK (user_low_id < user_high_id),
    UNIQUE (user_low_id, user_high_id),

    FOREIGN KEY (user_low_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY (user_high_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_conversations_activity
ON conversations (last_message_at DESC, id DESC);

-- The unique pair index starts with user_low_id; this index supports contact
-- lookups when the current user is the high side of the canonical pair.
CREATE INDEX idx_conversations_user_high
ON conversations (user_high_id, user_low_id);

CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    sender_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,

    FOREIGN KEY (conversation_id)
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    FOREIGN KEY (sender_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

-- id is the tie-breaker when messages share a timestamp, making history
-- ordering and (created_at, id) pagination cursors deterministic.
CREATE INDEX idx_messages_conversation_history
ON messages (conversation_id, created_at, id);
