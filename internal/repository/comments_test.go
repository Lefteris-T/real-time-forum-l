package repository

import (
	"errors"
	"path/filepath"
	"testing"

	"forum/internal/database"
)

func TestCommentRepositoryCreateStoresAuthor(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	defer db.Close()

	err = database.Migrate(
		db,
		filepath.Join("..", "..", "migrations"),
	)
	if err != nil {
		t.Fatalf("database.Migrate(): %v", err)
	}

	users := NewUserRepository(db)

	authorID, err := insertTestUser(users, "author@example.com", "author", "password-hash")
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	posts := NewPostRepository(db)

	postID, err := posts.Create(authorID,
		"Post title",
		"Post body",
		[]int64{1})
	if err != nil {
		t.Fatalf("posts.Create(): %v", err)
	}

	comments := NewCommentRepository(db)

	commentID, err := comments.Create(
		postID,
		authorID,
		"Hello comment",
	)
	if err != nil {
		t.Fatalf("comments.Create(): %v", err)
	}

	if commentID == 0 {
		t.Fatal("commentID = 0, want non-zero")
	}

	var gotPostID int64
	var gotAuthorID int64
	var gotBody string

	err = db.QueryRow(`
		SELECT post_id, author_id, body
		FROM comments
		WHERE id = ?
	`, commentID).Scan(
		&gotPostID,
		&gotAuthorID,
		&gotBody,
	)
	if err != nil {
		t.Fatalf("query comment: %v", err)
	}

	if gotPostID != postID {
		t.Fatalf(
			"postID = %d, want %d",
			gotPostID,
			postID,
		)
	}

	if gotAuthorID != authorID {
		t.Fatalf(
			"authorID = %d, want %d",
			gotAuthorID,
			authorID,
		)
	}

	if gotBody != "Hello comment" {
		t.Fatalf(
			"body = %q, want %q",
			gotBody,
			"Hello comment",
		)
	}
}

func TestCommentRepositoryCreateReturnsUnknownPost(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	defer db.Close()

	err = database.Migrate(
		db,
		filepath.Join("..", "..", "migrations"),
	)
	if err != nil {
		t.Fatalf("database.Migrate(): %v", err)
	}

	users := NewUserRepository(db)

	authorID, err := insertTestUser(users, "author@example.com", "author", "password-hash")
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	comments := NewCommentRepository(db)

	_, err = comments.Create(
		999,
		authorID,
		"Should fail",
	)

	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrPostNotFound,
		)
	}
}

func TestCommentRepositoryCreateReturnsUnknownAuthor(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("database.Migrate(): %v", err)
	}

	users := NewUserRepository(db)
	authorID, err := insertTestUser(users, "author@example.com", "author", "password-hash")
	if err != nil {
		t.Fatalf("create author: %v", err)
	}
	postID, err := NewPostRepository(db).Create(authorID, "Post title", "Post body", []int64{1})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	_, err = NewCommentRepository(db).Create(postID, authorID+1, "Should fail")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("Create() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestCommentRepositoryListByPostReturnsOrderedComments(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("database.Migrate(): %v", err)
	}

	users := NewUserRepository(db)
	authorID, err := insertTestUser(users, "author@example.com", "author", "password-hash")
	if err != nil {
		t.Fatalf("create author: %v", err)
	}
	commenterID, err := insertTestUser(users, "commenter@example.com", "commenter", "password-hash")
	if err != nil {
		t.Fatalf("create commenter: %v", err)
	}

	posts := NewPostRepository(db)
	postID, err := posts.Create(authorID, "Post title", "Post body", []int64{1})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	emptyPostID, err := posts.Create(authorID, "Empty post", "Post body", []int64{1})
	if err != nil {
		t.Fatalf("create empty post: %v", err)
	}
	otherPostID, err := posts.Create(authorID, "Other post", "Post body", []int64{1})
	if err != nil {
		t.Fatalf("create other post: %v", err)
	}

	insertComment := func(targetPostID, targetAuthorID int64, body, createdAt string) int64 {
		t.Helper()
		result, err := db.Exec(`
			INSERT INTO comments (post_id, author_id, body, created_at)
			VALUES (?, ?, ?, ?)
		`, targetPostID, targetAuthorID, body, createdAt)
		if err != nil {
			t.Fatalf("insert comment %q: %v", body, err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("get comment %q id: %v", body, err)
		}
		return id
	}

	oldID := insertComment(postID, commenterID, "Earlier", "2026-10-26T09:00:00Z")
	tieFirstID := insertComment(postID, commenterID, "Tie first", "2026-10-26T10:00:00Z")
	tieSecondID := insertComment(postID, authorID, "Tie second", "2026-10-26T10:00:00Z")
	insertComment(otherPostID, commenterID, "Belongs elsewhere", "2026-10-26T08:00:00Z")

	repo := NewCommentRepository(db)
	got, err := repo.ListByPost(postID)
	if err != nil {
		t.Fatalf("ListByPost(): %v", err)
	}
	wantIDs := []int64{oldID, tieFirstID, tieSecondID}
	if len(got) != len(wantIDs) {
		t.Fatalf("comment count = %d, want %d", len(got), len(wantIDs))
	}
	for i, wantID := range wantIDs {
		if got[i].ID != wantID {
			t.Fatalf("comments[%d].ID = %d, want %d", i, got[i].ID, wantID)
		}
		if got[i].PostID != postID {
			t.Fatalf("comments[%d].PostID = %d, want %d", i, got[i].PostID, postID)
		}
	}
	if got[0].Author.Username != "commenter" || got[2].Author.Username != "author" {
		t.Fatalf("comment authors = %q, %q; want commenter, author", got[0].Author.Username, got[2].Author.Username)
	}

	empty, err := repo.ListByPost(emptyPostID)
	if err != nil {
		t.Fatalf("ListByPost() for empty post: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty post comment count = %d, want 0", len(empty))
	}

	if _, err := repo.ListByPost(otherPostID + 1000); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("ListByPost() for unknown post error = %v, want %v", err, ErrPostNotFound)
	}
}
