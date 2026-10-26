package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"forum/internal/model"
)

// CommentRepository stores comments and resolves their parent posts.
type CommentRepository struct {
	db *sql.DB
}

// NewCommentRepository binds comment operations to db.
func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{
		db: db,
	}
}

// ErrCommentNotFound allows upper layers to map absence without exposing SQL.
var ErrCommentNotFound = errors.New("comment not found")

// CommentView contains the comment and public author/reaction data used by
// post detail and comment list responses.
type CommentView struct {
	ID        int64
	PostID    int64
	Body      string
	CreatedAt time.Time
	Author    model.User
	Likes     int
	Dislikes  int
}

// Create inserts a validated comment for an existing post and author.
func (r *CommentRepository) Create(
	postID int64,
	authorID int64,
	body string,
) (int64, error) {
	result, err := r.db.Exec(
		`
			INSERT INTO comments (
				post_id,
				author_id,
				body,
				created_at
			)
			VALUES (?, ?, ?, ?)
		`,
		postID,
		authorID,
		body,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(
			err.Error(),
			"FOREIGN KEY constraint failed",
		) {
			var exists int
			lookupErr := r.db.QueryRow(
				"SELECT 1 FROM posts WHERE id = ?",
				postID,
			).Scan(&exists)
			if errors.Is(lookupErr, sql.ErrNoRows) {
				return 0, ErrPostNotFound
			}
			if lookupErr != nil {
				return 0, lookupErr
			}

			lookupErr = r.db.QueryRow(
				"SELECT 1 FROM users WHERE id = ?",
				authorID,
			).Scan(&exists)
			if errors.Is(lookupErr, sql.ErrNoRows) {
				return 0, ErrUserNotFound
			}
			if lookupErr != nil {
				return 0, lookupErr
			}
		}

		return 0, err
	}

	commentID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return commentID, nil
}

// ListByPost returns comments for an existing post in stable chronological
// order, including the public author and reaction summary for each comment.
func (r *CommentRepository) ListByPost(postID int64) ([]CommentView, error) {
	var exists int
	if err := r.db.QueryRow(
		"SELECT 1 FROM posts WHERE id = ?",
		postID,
	).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	rows, err := r.db.Query(`
		SELECT
			c.id,
			c.post_id,
			c.body,
			c.created_at,
			u.id,
			u.email,
			u.username,
			u.created_at,
			COALESCE(SUM(
				CASE WHEN cr.value = 1 THEN 1 ELSE 0 END
			), 0) AS likes,
			COALESCE(SUM(
				CASE WHEN cr.value = -1 THEN 1 ELSE 0 END
			), 0) AS dislikes
		FROM comments c
		JOIN users u
			ON u.id = c.author_id
		LEFT JOIN comment_reactions cr
			ON cr.comment_id = c.id
		WHERE c.post_id = ?
		GROUP BY c.id
		ORDER BY c.created_at ASC, c.id ASC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []CommentView
	for rows.Next() {
		var comment CommentView
		var commentCreatedAt string
		var userCreatedAt string

		if err := rows.Scan(
			&comment.ID,
			&comment.PostID,
			&comment.Body,
			&commentCreatedAt,
			&comment.Author.ID,
			&comment.Author.Email,
			&comment.Author.Username,
			&userCreatedAt,
			&comment.Likes,
			&comment.Dislikes,
		); err != nil {
			return nil, err
		}

		comment.CreatedAt, err = time.Parse(time.RFC3339, commentCreatedAt)
		if err != nil {
			return nil, err
		}
		comment.Author.CreatedAt, err = time.Parse(time.RFC3339, userCreatedAt)
		if err != nil {
			return nil, err
		}

		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

// PostIDForComment resolves the redirect target after a comment reaction.
func (r *CommentRepository) PostIDForComment(
	commentID int64,
) (int64, error) {
	var postID int64

	err := r.db.QueryRow(`
		SELECT post_id
		FROM comments
		WHERE id = ?
	`, commentID).Scan(&postID)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrCommentNotFound
	}

	if err != nil {
		return 0, err
	}

	return postID, nil
}
