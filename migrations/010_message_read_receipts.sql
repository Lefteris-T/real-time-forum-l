-- A private message is unread by the other participant until that user opens
-- the conversation. A receipt row records that user's read state.
CREATE TABLE message_reads (
    message_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    read_at TEXT NOT NULL,

    PRIMARY KEY (message_id, user_id),

    FOREIGN KEY (message_id)
        REFERENCES messages(id)
        ON DELETE CASCADE,

    FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_message_reads_user_message
ON message_reads (user_id, message_id);
