package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestForumContentSchemaConstraintsAndIndexes(t *testing.T) {
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
	`, "schema@example.com", "schema-user", "hash", "2026-10-26T10:00:00Z", 30, "test", "Test", "User")
	if err != nil {
		t.Fatalf("insert valid user: %v", err)
	}
	authorID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("get user id: %v", err)
	}

	var categoryID int64
	if err := db.QueryRow(`SELECT id FROM categories WHERE name = 'General'`).Scan(&categoryID); err != nil {
		t.Fatalf("find seeded category: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO categories (name) VALUES ('General')`); err == nil {
		t.Fatal("duplicate category name was accepted")
	}

	postResult, err := db.Exec(`
		INSERT INTO posts (author_id, title, body, created_at)
		VALUES (?, ?, ?, ?)
	`, authorID, "Valid post", "Post body", "2026-10-26T10:00:00Z")
	if err != nil {
		t.Fatalf("insert post with valid author: %v", err)
	}
	postID, err := postResult.LastInsertId()
	if err != nil {
		t.Fatalf("get post id: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO post_categories (post_id, category_id)
		VALUES (?, ?)
	`, postID, categoryID); err != nil {
		t.Fatalf("link post to valid category: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO posts (author_id, title, body, created_at)
		VALUES (?, ?, ?, ?)
	`, authorID+1, "Orphan post", "Post body", "2026-10-26T10:01:00Z"); err == nil {
		t.Fatal("post with nonexistent author was accepted")
	}

	if _, err := db.Exec(`
		INSERT INTO post_categories (post_id, category_id)
		VALUES (?, ?)
	`, postID, categoryID+1000); err == nil {
		t.Fatal("post-category link with nonexistent category was accepted")
	}

	if _, err := db.Exec(`
		INSERT INTO post_categories (post_id, category_id)
		VALUES (?, ?)
	`, postID+1000, categoryID); err == nil {
		t.Fatal("post-category link with nonexistent post was accepted")
	}

	if _, err := db.Exec(`
		INSERT INTO post_categories (post_id, category_id)
		VALUES (?, ?)
	`, postID, categoryID); err == nil {
		t.Fatal("duplicate post-category link was accepted")
	}

	assertIndexExists(t, db, "posts", "idx_posts_feed_order")
	assertIndexColumns(t, db, "idx_posts_feed_order", []string{"created_at", "id"})
	assertIndexExists(t, db, "post_categories", "idx_post_categories_category_id")
	assertIndexColumns(t, db, "idx_post_categories_category_id", []string{"category_id", "post_id"})
}

func assertIndexExists(t *testing.T, db *sql.DB, table, indexName string) {
	t.Helper()

	var got string
	if err := db.QueryRow(`
		SELECT tbl_name
		FROM sqlite_master
		WHERE type = 'index' AND name = ?
	`, indexName).Scan(&got); err != nil {
		t.Fatalf("find index %q: %v", indexName, err)
	}
	if got != table {
		t.Fatalf("index %q table = %q, want %q", indexName, got, table)
	}
}

func assertIndexColumns(t *testing.T, db *sql.DB, indexName string, want []string) {
	t.Helper()

	rows, err := db.Query("PRAGMA index_info('" + indexName + "')")
	if err != nil {
		t.Fatalf("query index %q columns: %v", indexName, err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var sequence, columnID int
		var name string
		if err := rows.Scan(&sequence, &columnID, &name); err != nil {
			t.Fatalf("scan index %q columns: %v", indexName, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read index %q columns: %v", indexName, err)
	}
	if len(got) != len(want) {
		t.Fatalf("index %q columns = %v, want %v", indexName, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %q column %d = %q, want %q", indexName, i, got[i], want[i])
		}
	}
}
