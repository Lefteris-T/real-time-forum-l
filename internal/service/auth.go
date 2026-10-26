// Package service enforces forum business rules independently of HTTP and SQL.
package service

import (
	"errors"
	"fmt"
	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/validation"
	"time"
)

// Validation errors let handlers distinguish bad input from internal failures.
var ErrInvalidRegistration = errors.New("invalid registration")
var ErrInvalidLogin = errors.New("invalid login")

// UserCreator is the smallest persistence boundary required for registration.
type UserCreator interface {
	Create(user model.User) (int64, error)
}

// PasswordHasher keeps bcrypt details outside registration orchestration.
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// AuthService validates identities, hashes passwords, and creates users.
type AuthService struct {
	users     UserCreator
	passwords PasswordHasher
}

// NewAuthService constructs registration behavior from replaceable boundaries.
func NewAuthService(
	users UserCreator,
	passwords PasswordHasher,
) *AuthService {
	return &AuthService{
		users:     users,
		passwords: passwords,
	}
}

// Register normalizes and validates input before any value is persisted.
func (s *AuthService) Register(
	input validation.RegistrationInput,
) (int64, error) {
	validated, err := validation.ValidateRegistration(input)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: %v",
			ErrInvalidRegistration,
			err,
		)
	}

	hash, err := s.passwords.Hash(validated.Password)
	if err != nil {
		return 0, err
	}

	return s.users.Create(model.User{
		Email:        validated.Email,
		Username:     validated.Username,
		Age:          validated.Age,
		Gender:       validated.Gender,
		FirstName:    validated.FirstName,
		LastName:     validated.LastName,
		PasswordHash: hash,
	})
}

// ErrInvalidCredentials deliberately covers both unknown users and bad
// passwords so callers cannot reveal whether an email is registered.
var ErrInvalidCredentials = errors.New("invalid credentials")

// Comparing against a real-format dummy hash reduces timing differences when
// the submitted email does not exist.
const dummyPasswordHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoO5xDBmIf6M1q3P0RHFN6GmlCaypVH7De"

type UserFinder interface {
	ByIdentifier(identifier string) (model.User, error)
}

type PasswordComparer interface {
	Compare(hash, password string) error
}

type SessionStore interface {
	Replace(
		id string,
		userID int64,
		expiresAt time.Time,
	) error

	Delete(id string) error
}

// LoginService verifies credentials and owns the session lifecycle rules.
type LoginService struct {
	users           UserFinder
	passwords       PasswordComparer
	sessions        SessionStore
	sessionDuration time.Duration
}

func NewLoginService(
	users UserFinder,
	passwords PasswordComparer,
	sessions SessionStore,
	sessionDuration time.Duration,
) *LoginService {
	return &LoginService{
		users:           users,
		passwords:       passwords,
		sessions:        sessions,
		sessionDuration: sessionDuration,
	}
}

// Login returns the same public failure for an unknown email and wrong password.
func (s *LoginService) Login(
	input validation.LoginInput,
) (model.User, error) {
	validated, err := validation.ValidateLogin(input)
	if err != nil {
		return model.User{}, fmt.Errorf(
			"%w: %v",
			ErrInvalidLogin,
			err,
		)
	}

	user, err := s.users.ByIdentifier(validated.Identifier)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			_ = s.passwords.Compare(
				dummyPasswordHash,
				validated.Password,
			)

			return model.User{}, ErrInvalidCredentials
		}

		return model.User{}, err
	}

	if user.PasswordHash == "" {
		return model.User{}, ErrInvalidCredentials
	}

	if err := s.passwords.Compare(
		user.PasswordHash,
		validated.Password,
	); err != nil {
		return model.User{}, ErrInvalidCredentials
	}

	return user, nil
}

// CreateSession calculates server-side expiry and replaces any prior session.
func (s *LoginService) CreateSession(
	id string,
	userID int64,
) error {
	expiresAt := time.Now().
		UTC().
		Add(s.sessionDuration)

	return s.sessions.Replace(
		id,
		userID,
		expiresAt,
	)
}

// Logout invalidates the server-side record for id.
func (s *LoginService) Logout(id string) error {
	return s.sessions.Delete(id)
}
