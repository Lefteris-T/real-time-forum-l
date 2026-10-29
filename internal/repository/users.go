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
	return r.byIdentity(email, "email")
}

// ByIdentifier looks up a user by email when identifier contains an @ sign,
// and by username otherwise. Callers should pass a normalized identifier.
func (r *UserRepository) ByIdentifier(identifier string) (model.User, error) {
	if strings.Contains(identifier, "@") {
		return r.byIdentity(identifier, "email")
	}

	return r.byIdentity(identifier, "username")
}

func (r *UserRepository) byIdentity(value, column string) (model.User, error) {
	var user model.User
	var createdAt string

	query := fmt.Sprintf(`
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
		WHERE %s = ?
	`, column)

	err := r.db.QueryRow(query, value).Scan(
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

		return model.User{}, fmt.Errorf("find user by %s: %w", column, err)
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

// PublicProfileByID returns only fields intended for a profile page.
func (r *UserRepository) PublicProfileByID(id int64) (model.Profile, error) {
	var profile model.Profile
	err := r.db.QueryRow(`
		SELECT id, username, first_name, last_name, age, gender
		FROM users
		WHERE id = ?
	`, id).Scan(
		&profile.ID,
		&profile.Username,
		&profile.FirstName,
		&profile.LastName,
		&profile.Age,
		&profile.Gender,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Profile{}, ErrUserNotFound
	}
	if err != nil {
		return model.Profile{}, fmt.Errorf("find public profile: %w", err)
	}
	return profile, nil
}

// UpdateProfile changes only the editable public profile fields.
func (r *UserRepository) UpdateProfile(userID int64, update model.ProfileUpdate) error {
	result, err := r.db.Exec(`
		UPDATE users
		SET first_name = ?, last_name = ?, age = ?, gender = ?
		WHERE id = ?
	`, update.FirstName, update.LastName, update.Age, update.Gender, userID)
	if err != nil {
		return fmt.Errorf("update public profile: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated profile: %w", err)
	}
	if count == 0 {
		return ErrUserNotFound
	}
	return nil
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
