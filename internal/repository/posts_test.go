package repository

import (
	"errors"
	"path/filepath"
	"testing"

	"forum/internal/database"
	"forum/internal/model"
)

func TestPostRepositoryCreateWithOneCategory(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userID, err := insertTestUser(users, "lefteris@example.com", "lefteris", "password-hash")
	if err != nil {
		t.Fatalf("users.Create(): %v", err)
	}

	posts := NewPostRepository(db)

	postID, err := posts.Create(userID,
		"My first post",
		"This is the body",
		[]int64{1})
	if err != nil {
		t.Fatalf("posts.Create(): %v", err)
	}

	if postID == 0 {
		t.Fatal("postID = 0, want non-zero")
	}

	var gotTitle string
	var gotAuthorID int64

	err = db.QueryRow(`
		SELECT title, author_id
		FROM posts
		WHERE id = ?
	`, postID).Scan(
		&gotTitle,
		&gotAuthorID,
	)
	if err != nil {
		t.Fatalf("query post: %v", err)
	}

	if gotTitle != "My first post" {
		t.Fatalf(
			"title = %q, want %q",
			gotTitle,
			"My first post",
		)
	}

	if gotAuthorID != userID {
		t.Fatalf(
			"author_id = %d, want %d",
			gotAuthorID,
			userID,
		)
	}

	var categoryID int64

	err = db.QueryRow(`
		SELECT category_id
		FROM post_categories
		WHERE post_id = ?
	`, postID).Scan(&categoryID)
	if err != nil {
		t.Fatalf("query post category: %v", err)
	}

	if categoryID != 1 {
		t.Fatalf(
			"category_id = %d, want 1",
			categoryID,
		)
	}
}
func TestPostRepositoryCreateWithSeveralCategories(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userID, err := insertTestUser(users, "lefteris@example.com", "lefteris", "password-hash")
	if err != nil {
		t.Fatalf("users.Create(): %v", err)
	}

	posts := NewPostRepository(db)

	postID, err := posts.Create(userID,
		"Post with categories",
		"Body",
		[]int64{1, 2, 3})
	if err != nil {
		t.Fatalf("posts.Create(): %v", err)
	}

	rows, err := db.Query(`
		SELECT category_id
		FROM post_categories
		WHERE post_id = ?
		ORDER BY category_id
	`, postID)
	if err != nil {
		t.Fatalf("query post categories: %v", err)
	}
	defer rows.Close()

	var got []int64

	for rows.Next() {
		var id int64

		if err := rows.Scan(&id); err != nil {
			t.Fatalf("rows.Scan(): %v", err)
		}

		got = append(got, id)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(): %v", err)
	}

	want := []int64{1, 2, 3}

	if len(got) != len(want) {
		t.Fatalf(
			"category count = %d, want %d",
			len(got),
			len(want),
		)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf(
				"category[%d] = %d, want %d",
				i,
				got[i],
				want[i],
			)
		}
	}
}
func TestPostRepositoryCreateRollsBackOnUnknownCategory(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userID, err := insertTestUser(users, "lefteris@example.com", "lefteris", "password-hash")
	if err != nil {
		t.Fatalf("users.Create(): %v", err)
	}

	posts := NewPostRepository(db)

	_, err = posts.Create(userID,
		"Should rollback",
		"This post must not remain",
		[]int64{1, 999})
	if err == nil {
		t.Fatal("posts.Create() error = nil, want error")
	}

	var postCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM posts
		WHERE title = ?
	`, "Should rollback").Scan(&postCount)
	if err != nil {
		t.Fatalf("count posts: %v", err)
	}

	if postCount != 0 {
		t.Fatalf(
			"post count = %d, want 0 after rollback",
			postCount,
		)
	}

	var linkCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM post_categories
	`).Scan(&linkCount)
	if err != nil {
		t.Fatalf("count post_categories: %v", err)
	}

	if linkCount != 0 {
		t.Fatalf(
			"post_categories count = %d, want 0 after rollback",
			linkCount,
		)
	}
}
func TestPostRepositoryListReturnsEmptyList(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	posts := NewPostRepository(db)

	got, err := posts.List()
	if err != nil {
		t.Fatalf("posts.List(): %v", err)
	}

	if len(got) != 0 {
		t.Fatalf(
			"len(posts) = %d, want 0",
			len(got),
		)
	}
}
func TestPostRepositoryListReturnsNewestFirstDeterministically(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userID, err := insertTestUser(users, "lefteris@example.com", "lefteris", "password-hash")
	if err != nil {
		t.Fatalf("users.Create(): %v", err)
	}

	createdAtOld := "2026-08-20T10:00:00Z"
	createdAtNew := "2026-08-20T12:00:00Z"

	result, err := db.Exec(`
		INSERT INTO posts (
			author_id,
			title,
			body,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		userID,
		"Old post",
		"body",
		createdAtOld,
	)
	if err != nil {
		t.Fatalf("insert old post: %v", err)
	}

	oldID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId old: %v", err)
	}

	result, err = db.Exec(`
		INSERT INTO posts (
			author_id,
			title,
			body,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		userID,
		"New post A",
		"body",
		createdAtNew,
	)
	if err != nil {
		t.Fatalf("insert new post A: %v", err)
	}

	newAID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId new A: %v", err)
	}

	result, err = db.Exec(`
		INSERT INTO posts (
			author_id,
			title,
			body,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		userID,
		"New post B",
		"body",
		createdAtNew,
	)
	if err != nil {
		t.Fatalf("insert new post B: %v", err)
	}

	newBID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId new B: %v", err)
	}

	posts := NewPostRepository(db)

	got, err := posts.List()
	if err != nil {
		t.Fatalf("posts.List(): %v", err)
	}

	if len(got) != 3 {
		t.Fatalf(
			"len(posts) = %d, want 3",
			len(got),
		)
	}

	wantIDs := []int64{
		newBID,
		newAID,
		oldID,
	}

	for i, wantID := range wantIDs {
		if got[i].ID != wantID {
			t.Fatalf(
				"posts[%d].ID = %d, want %d",
				i,
				got[i].ID,
				wantID,
			)
		}
	}
}
func TestPostRepositoryListReturnsFullPostDataWithoutDuplicates(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	user2ID, err := insertTestUser(users, "user2@example.com", "user2", "password-hash")
	if err != nil {
		t.Fatalf("create user2: %v", err)
	}

	user3ID, err := insertTestUser(users, "user3@example.com", "user3", "password-hash")
	if err != nil {
		t.Fatalf("create user3: %v", err)
	}

	posts := NewPostRepository(db)

	postID, err := posts.Create(authorID,
		"Go and DevOps",
		"Testing full post data",
		[]int64{2, 4})
	if err != nil {
		t.Fatalf("posts.Create(): %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO post_reactions (
			user_id,
			post_id,
			value
		)
		VALUES (?, ?, ?)
	`,
		user2ID,
		postID,
		1,
	)
	if err != nil {
		t.Fatalf("insert like: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO post_reactions (
			user_id,
			post_id,
			value
		)
		VALUES (?, ?, ?)
	`,
		user3ID,
		postID,
		-1,
	)
	if err != nil {
		t.Fatalf("insert dislike: %v", err)
	}

	got, err := posts.List()
	if err != nil {
		t.Fatalf("posts.List(): %v", err)
	}

	if len(got) != 1 {
		t.Fatalf(
			"len(posts) = %d, want 1",
			len(got),
		)
	}

	post := got[0]

	if post.ID != postID {
		t.Fatalf(
			"post.ID = %d, want %d",
			post.ID,
			postID,
		)
	}

	if post.Author.Username != "author" {
		t.Fatalf(
			"author username = %q, want %q",
			post.Author.Username,
			"author",
		)
	}

	if len(post.Categories) != 2 {
		t.Fatalf(
			"category count = %d, want 2",
			len(post.Categories),
		)
	}

	if post.Categories[0].Name != "Go" {
		t.Fatalf(
			"first category = %q, want Go",
			post.Categories[0].Name,
		)
	}

	if post.Categories[1].Name != "DevOps" {
		t.Fatalf(
			"second category = %q, want DevOps",
			post.Categories[1].Name,
		)
	}

	if post.Likes != 1 {
		t.Fatalf(
			"likes = %d, want 1",
			post.Likes,
		)
	}

	if post.Dislikes != 1 {
		t.Fatalf(
			"dislikes = %d, want 1",
			post.Dislikes,
		)
	}
}
func TestPostRepositoryDetailReturnsPostWithOrderedComments(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	commenterID, err := insertTestUser(users, "commenter@example.com", "commenter", "password-hash")
	if err != nil {
		t.Fatalf("create commenter: %v", err)
	}

	reactorID, err := insertTestUser(users, "reactor@example.com", "reactor", "password-hash")
	if err != nil {
		t.Fatalf("create reactor: %v", err)
	}

	posts := NewPostRepository(db)

	postID, err := posts.Create(authorID,
		"Post title",
		"Post body",
		[]int64{1, 2})
	if err != nil {
		t.Fatalf("posts.Create(): %v", err)
	}

	firstCreatedAt := "2026-08-20T10:00:00Z"
	secondCreatedAt := "2026-08-20T11:00:00Z"

	result, err := db.Exec(`
		INSERT INTO comments (
			post_id,
			author_id,
			body,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		postID,
		commenterID,
		"First comment",
		firstCreatedAt,
	)
	if err != nil {
		t.Fatalf("insert first comment: %v", err)
	}

	firstCommentID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("first comment LastInsertId: %v", err)
	}

	result, err = db.Exec(`
		INSERT INTO comments (
			post_id,
			author_id,
			body,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		postID,
		commenterID,
		"Second comment",
		secondCreatedAt,
	)
	if err != nil {
		t.Fatalf("insert second comment: %v", err)
	}

	secondCommentID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("second comment LastInsertId: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO post_reactions (
			user_id,
			post_id,
			value
		)
		VALUES (?, ?, ?)
	`,
		reactorID,
		postID,
		1,
	)
	if err != nil {
		t.Fatalf("insert post reaction: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO comment_reactions (
			user_id,
			comment_id,
			value
		)
		VALUES (?, ?, ?)
	`,
		reactorID,
		firstCommentID,
		1,
	)
	if err != nil {
		t.Fatalf("insert first comment reaction: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO comment_reactions (
			user_id,
			comment_id,
			value
		)
		VALUES (?, ?, ?)
	`,
		authorID,
		secondCommentID,
		-1,
	)
	if err != nil {
		t.Fatalf("insert second comment reaction: %v", err)
	}

	got, err := posts.Detail(postID)
	if err != nil {
		t.Fatalf("posts.Detail(): %v", err)
	}

	if got.ID != postID {
		t.Fatalf(
			"post ID = %d, want %d",
			got.ID,
			postID,
		)
	}

	if got.Author.Username != "author" {
		t.Fatalf(
			"author username = %q, want author",
			got.Author.Username,
		)
	}

	if len(got.Categories) != 2 {
		t.Fatalf(
			"category count = %d, want 2",
			len(got.Categories),
		)
	}

	if got.Likes != 1 {
		t.Fatalf(
			"post likes = %d, want 1",
			got.Likes,
		)
	}

	if got.Dislikes != 0 {
		t.Fatalf(
			"post dislikes = %d, want 0",
			got.Dislikes,
		)
	}

	if len(got.Comments) != 2 {
		t.Fatalf(
			"comment count = %d, want 2",
			len(got.Comments),
		)
	}

	if got.Comments[0].ID != firstCommentID {
		t.Fatalf(
			"first comment ID = %d, want %d",
			got.Comments[0].ID,
			firstCommentID,
		)
	}

	if got.Comments[1].ID != secondCommentID {
		t.Fatalf(
			"second comment ID = %d, want %d",
			got.Comments[1].ID,
			secondCommentID,
		)
	}

	if got.Comments[0].Author.Username != "commenter" {
		t.Fatalf(
			"first comment author = %q, want commenter",
			got.Comments[0].Author.Username,
		)
	}

	if got.Comments[0].Likes != 1 {
		t.Fatalf(
			"first comment likes = %d, want 1",
			got.Comments[0].Likes,
		)
	}

	if got.Comments[0].Dislikes != 0 {
		t.Fatalf(
			"first comment dislikes = %d, want 0",
			got.Comments[0].Dislikes,
		)
	}

	if got.Comments[1].Likes != 0 {
		t.Fatalf(
			"second comment likes = %d, want 0",
			got.Comments[1].Likes,
		)
	}

	if got.Comments[1].Dislikes != 1 {
		t.Fatalf(
			"second comment dislikes = %d, want 1",
			got.Comments[1].Dislikes,
		)
	}
}
func TestPostRepositoryDetailReturnsNotFoundForMissingPost(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	posts := NewPostRepository(db)

	_, err = posts.Detail(999)

	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf(
			"Detail() error = %v, want %v",
			err,
			ErrPostNotFound,
		)
	}
}
func TestPostRepositoryListByCategoryReturnsExactPosts(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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
		t.Fatalf("users.Create(): %v", err)
	}

	posts := NewPostRepository(db)

	goPostID, err := posts.Create(authorID,
		"Go post",
		"Body",
		[]int64{2})
	if err != nil {
		t.Fatalf("create Go post: %v", err)
	}

	_, err = posts.Create(authorID,
		"DevOps post",
		"Body",
		[]int64{4})
	if err != nil {
		t.Fatalf("create DevOps post: %v", err)
	}

	multiPostID, err := posts.Create(authorID,
		"Go and DevOps",
		"Body",
		[]int64{2, 4})
	if err != nil {
		t.Fatalf("create multi-category post: %v", err)
	}

	got, err := posts.ListByCategory(2)
	if err != nil {
		t.Fatalf("ListByCategory(): %v", err)
	}

	if len(got) != 2 {
		t.Fatalf(
			"len(posts) = %d, want 2",
			len(got),
		)
	}

	gotIDs := map[int64]bool{
		got[0].ID: true,
		got[1].ID: true,
	}

	if !gotIDs[goPostID] {
		t.Fatal("Go post was not returned")
	}

	if !gotIDs[multiPostID] {
		t.Fatal("multi-category Go post was not returned")
	}
}
func TestPostRepositoryListByCategoryReturnsNotFoundForUnknownCategory(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	posts := NewPostRepository(db)

	_, err = posts.ListByCategory(999)

	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf(
			"ListByCategory() error = %v, want %v",
			err,
			ErrCategoryNotFound,
		)
	}
}
func TestPostRepositoryListByAuthorReturnsOnlyOwnPosts(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userAID, err := insertTestUser(users, "a@example.com", "userA", "password-hash")
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}

	userBID, err := insertTestUser(users, "b@example.com", "userB", "password-hash")
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}

	posts := NewPostRepository(db)

	ownPostID, err := posts.Create(userAID,
		"My post",
		"Body",
		[]int64{1, 2})
	if err != nil {
		t.Fatalf("create own post: %v", err)
	}

	_, err = posts.Create(userBID,
		"Other post",
		"Body",
		[]int64{4})
	if err != nil {
		t.Fatalf("create other post: %v", err)
	}

	got, err := posts.ListByAuthor(userAID)
	if err != nil {
		t.Fatalf("ListByAuthor(): %v", err)
	}

	if len(got) != 1 {
		t.Fatalf(
			"len(posts) = %d, want 1",
			len(got),
		)
	}

	if got[0].ID != ownPostID {
		t.Fatalf(
			"post ID = %d, want %d",
			got[0].ID,
			ownPostID,
		)
	}

	if got[0].Author.ID != userAID {
		t.Fatalf(
			"author ID = %d, want %d",
			got[0].Author.ID,
			userAID,
		)
	}

	if len(got[0].Categories) != 2 {
		t.Fatalf(
			"category count = %d, want 2",
			len(got[0].Categories),
		)
	}
}
func TestPostRepositoryListLikedByUserReturnsOnlyActiveLikes(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userAID, err := insertTestUser(users, "a@example.com", "userA", "password-hash")
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}

	userBID, err := insertTestUser(users, "b@example.com", "userB", "password-hash")
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}

	posts := NewPostRepository(db)

	likedPostID, err := posts.Create(authorID,
		"Liked post",
		"Body",
		[]int64{1, 2})
	if err != nil {
		t.Fatalf("create liked post: %v", err)
	}

	dislikedPostID, err := posts.Create(authorID,
		"Disliked post",
		"Body",
		[]int64{4})
	if err != nil {
		t.Fatalf("create disliked post: %v", err)
	}

	otherUserLikedPostID, err := posts.Create(authorID,
		"Other user's liked post",
		"Body",
		[]int64{2})
	if err != nil {
		t.Fatalf("create other user liked post: %v", err)
	}

	reactions := NewReactionRepository(db)

	err = reactions.SetPostReaction(
		userAID,
		likedPostID,
		model.ReactionLike,
	)
	if err != nil {
		t.Fatalf("user A like: %v", err)
	}

	err = reactions.SetPostReaction(
		userAID,
		dislikedPostID,
		model.ReactionDislike,
	)
	if err != nil {
		t.Fatalf("user A dislike: %v", err)
	}

	err = reactions.SetPostReaction(
		userBID,
		otherUserLikedPostID,
		model.ReactionLike,
	)
	if err != nil {
		t.Fatalf("user B like: %v", err)
	}

	got, err := posts.ListLikedByUser(userAID)
	if err != nil {
		t.Fatalf("ListLikedByUser(): %v", err)
	}

	if len(got) != 1 {
		t.Fatalf(
			"len(posts) = %d, want 1",
			len(got),
		)
	}

	if got[0].ID != likedPostID {
		t.Fatalf(
			"post ID = %d, want %d",
			got[0].ID,
			likedPostID,
		)
	}

	if got[0].ID == dislikedPostID {
		t.Fatal("disliked post was returned")
	}

	if got[0].ID == otherUserLikedPostID {
		t.Fatal("another user's liked post was returned")
	}

	if len(got[0].Categories) != 2 {
		t.Fatalf(
			"category count = %d, want 2",
			len(got[0].Categories),
		)
	}
}
func TestPostRepositoryListLikedByUserExcludesRemovedLike(t *testing.T) {
	dbPath := filepath.Join(
		t.TempDir(),
		"forum.db",
	)

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

	userID, err := insertTestUser(users, "user@example.com", "user", "password-hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	posts := NewPostRepository(db)

	postID, err := posts.Create(authorID,
		"Temporarily liked",
		"Body",
		[]int64{1})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	reactions := NewReactionRepository(db)

	err = reactions.SetPostReaction(
		userID,
		postID,
		model.ReactionLike,
	)
	if err != nil {
		t.Fatalf("set like: %v", err)
	}

	got, err := posts.ListLikedByUser(userID)
	if err != nil {
		t.Fatalf("ListLikedByUser() after like: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf(
			"liked post count = %d, want 1",
			len(got),
		)
	}

	// Same reaction again = toggle off.
	err = reactions.SetPostReaction(
		userID,
		postID,
		model.ReactionLike,
	)
	if err != nil {
		t.Fatalf("remove like: %v", err)
	}

	got, err = posts.ListLikedByUser(userID)
	if err != nil {
		t.Fatalf("ListLikedByUser() after removal: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf(
			"liked post count = %d, want 0 after removed like",
			len(got),
		)
	}
}
