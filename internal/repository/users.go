package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"forum/internal/model"
)

var (
	// Identity errors are stable repository outcomes that services translate
	// into user-safe registration or login responses.
	ErrUserNotFound   = errors.New("user not found")
	ErrEmailExists    = errors.New("email already exists")
	ErrUsernameExists = errors.New("username already exists")
)

// UserRepository persists account identities and password hashes.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository binds user operations to db.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

// Create stores a normalized identity and maps SQLite uniqueness failures to
// domain-specific duplicate errors.
func (r *UserRepository) Create(user model.User) (int64, error) {
	result, err := r.db.Exec(`
		INSERT INTO users (
			email,
			username,
			age,
			gender,
			first_name,
			last_name,
			password_hash,
			created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		user.Email,
		user.Username,
		user.Age,
		user.Gender,
		user.FirstName,
		user.LastName,
		user.PasswordHash,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, translateUserConstraintError(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted user id: %w", err)
	}

	return id, nil
}

// ByEmail retrieves the account used during credential verification.
func (r *UserRepository) ByEmail(email string) (model.User, error) {
	var user model.User
	var createdAt string

	err := r.db.QueryRow(`
		SELECT
			id,
			email,
			username,
			age,
			gender,
			first_name,
			last_name,
			password_hash,
			created_at
		FROM users
		WHERE email = ?
	`,
		email,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.Age,
		&user.Gender,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&createdAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}

		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}

	user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return model.User{}, fmt.Errorf("parse user created_at: %w", err)
	}

	return user, nil
}

// ByID resolves the current user referenced by a valid session.
func (r *UserRepository) ByID(id int64) (model.User, error) {
	var user model.User
	var createdAt string

	err := r.db.QueryRow(`
		SELECT
			id,
			email,
			username,
			age,
			gender,
			first_name,
			last_name,
			password_hash,
			created_at
		FROM users
		WHERE id = ?
	`,
		id,
	).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.Age,
		&user.Gender,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&createdAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}

		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}

	user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return model.User{}, fmt.Errorf("parse user created_at: %w", err)
	}

	return user, nil
}

func translateUserConstraintError(err error) error {
	message := err.Error()

	switch {
	case strings.Contains(message, "users.email"):
		return ErrEmailExists

	case strings.Contains(message, "users.username"):
		return ErrUsernameExists

	default:
		return fmt.Errorf("create user: %w", err)
	}
}
