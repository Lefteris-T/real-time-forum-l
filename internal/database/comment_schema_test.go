package database

import (
	"path/filepath"
	"testing"
)

func TestCommentSchemaForeignKeysAndHistoryIndex(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	result, err := db.Exec(`
		INSERT INTO users (
			email, username, password_hash, created_at,
			age, gender, first_name, last_name
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "comment-schema@example.com", "comment-schema-user", "hash", "2026-10-26T10:00:00Z", 30, "test", "Test", "User")
	if err != nil {
		t.Fatalf("insert valid user: %v", err)
	}
	authorID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get user id: %v", err)
	}

	result, err = db.Exec(`
		INSERT INTO posts (author_id, title, body, created_at)
		VALUES (?, ?, ?, ?)
	`, authorID, "Comment schema post", "Post body", "2026-10-26T10:01:00Z")
	if err != nil {
		t.Fatalf("insert valid post: %v", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get post id: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO comments (post_id, author_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`, postID, authorID, "Valid comment", "2026-10-26T10:02:00Z"); err != nil {
		t.Fatalf("insert valid comment: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO comments (post_id, author_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`, postID+1, authorID, "Missing post", "2026-10-26T10:03:00Z"); err == nil {
		t.Fatal("comment with nonexistent post was accepted")
	}

	if _, err := db.Exec(`
		INSERT INTO comments (post_id, author_id, body, created_at)
		VALUES (?, ?, ?, ?)
	`, postID, authorID+1, "Missing author", "2026-10-26T10:04:00Z"); err == nil {
		t.Fatal("comment with nonexistent author was accepted")
	}

	assertIndexExists(t, db, "comments", "idx_comments_post_history")
	assertIndexColumns(t, db, "idx_comments_post_history", []string{"post_id", "created_at", "id"})
}
