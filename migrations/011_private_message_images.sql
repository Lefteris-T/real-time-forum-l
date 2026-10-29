-- Images are stored as bounded SQLite BLOBs and fetched only through the
-- participant-authorized message image endpoint.
CREATE TABLE message_images (
    message_id INTEGER PRIMARY KEY,
    mime_type TEXT NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/gif')),
    image_data BLOB NOT NULL,
    byte_size INTEGER NOT NULL CHECK (byte_size > 0 AND byte_size <= 5242880),

    FOREIGN KEY (message_id)
        REFERENCES messages(id)
        ON DELETE CASCADE
);
