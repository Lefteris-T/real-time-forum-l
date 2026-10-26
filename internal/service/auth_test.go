package service

import (
	"errors"
	"testing"
	"time"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/validation"
)

type fakeUserRepository struct {
	createCalled bool

	email        string
	username     string
	user         model.User
	passwordHash string

	id  int64
	err error
}

func (f *fakeUserRepository) Create(user model.User) (int64, error) {
	f.createCalled = true
	f.user = user
	f.email = user.Email
	f.username = user.Username
	f.passwordHash = user.PasswordHash

	return f.id, f.err
}

type fakePasswordManager struct {
	hashCalled bool

	gotPassword string

	hash string
	err  error
}

func (f *fakePasswordManager) Hash(password string) (string, error) {
	f.hashCalled = true
	f.gotPassword = password

	return f.hash, f.err
}

func TestRegisterValidUser(t *testing.T) {
	repo := &fakeUserRepository{
		id: 42,
	}

	passwords := &fakePasswordManager{
		hash: "hashed-password",
	}

	auth := NewAuthService(repo, passwords)

	id, err := auth.Register(validation.RegistrationInput{
		Email:    "  Lefteris@Example.COM  ",
		Username: "  lefteris  ",
		Age:      30, Gender: "test", FirstName: "Test", LastName: "User",
		Password: "strong-password-123",
	})
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	if id != 42 {
		t.Fatalf("Register() id = %d, want 42", id)
	}

	if !passwords.hashCalled {
		t.Fatal("password Hash() was not called")
	}

	if passwords.gotPassword != "strong-password-123" {
		t.Fatalf(
			"Hash() password = %q, want original password",
			passwords.gotPassword,
		)
	}

	if !repo.createCalled {
		t.Fatal("repository Create() was not called")
	}

	if repo.email != "lefteris@example.com" {
		t.Fatalf(
			"repository email = %q, want normalized email",
			repo.email,
		)
	}

	if repo.username != "lefteris" {
		t.Fatalf(
			"repository username = %q, want normalized username",
			repo.username,
		)
	}

	if repo.passwordHash != "hashed-password" {
		t.Fatalf(
			"repository password hash = %q, want hashed-password",
			repo.passwordHash,
		)
	}
	if repo.user.Age != 30 || repo.user.Gender != "test" || repo.user.FirstName != "Test" || repo.user.LastName != "User" {
		t.Fatalf("repository profile = %+v, want submitted profile", repo.user)
	}
}
func TestRegisterInvalidInputStopsEarly(t *testing.T) {
	repo := &fakeUserRepository{}

	passwords := &fakePasswordManager{
		hash: "hashed-password",
	}

	auth := NewAuthService(repo, passwords)

	_, err := auth.Register(validation.RegistrationInput{
		Email:    "not-an-email",
		Username: "lefteris",
		Password: "strong-password-123",
	})

	if err == nil {
		t.Fatal("Register() error = nil, want validation error")
	}

	if passwords.hashCalled {
		t.Fatal("password Hash() was called for invalid input")
	}

	if repo.createCalled {
		t.Fatal("repository Create() was called for invalid input")
	}
}
func TestRegisterPreservesDuplicateErrors(t *testing.T) {
	tests := []struct {
		name     string
		repoErr  error
		expected error
	}{
		{
			name:     "duplicate email",
			repoErr:  repository.ErrEmailExists,
			expected: repository.ErrEmailExists,
		},
		{
			name:     "duplicate username",
			repoErr:  repository.ErrUsernameExists,
			expected: repository.ErrUsernameExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserRepository{
				err: tt.repoErr,
			}

			passwords := &fakePasswordManager{
				hash: "hashed-password",
			}

			auth := NewAuthService(repo, passwords)

			_, err := auth.Register(validation.RegistrationInput{
				Email:    "lefteris@example.com",
				Username: "lefteris",
				Age:      30, Gender: "test", FirstName: "Test", LastName: "User",
				Password: "strong-password-123",
			})

			if err != tt.expected {
				t.Fatalf(
					"Register() error = %v, want %v",
					err,
					tt.expected,
				)
			}
		})
	}
}

type fakeUserFinder struct {
	user model.User
	err  error

	called     bool
	identifier string
}

func (f *fakeUserFinder) ByIdentifier(identifier string) (model.User, error) {
	f.called = true
	f.identifier = identifier

	return f.user, f.err
}

type fakePasswordComparer struct {
	called bool

	hash     string
	password string

	err error
}

func (f *fakePasswordComparer) Compare(
	hash string,
	password string,
) error {
	f.called = true
	f.hash = hash
	f.password = password

	return f.err
}
func TestLoginWithCorrectCredentials(t *testing.T) {
	users := &fakeUserFinder{
		user: model.User{
			ID:           42,
			Email:        "lefteris@example.com",
			PasswordHash: "stored-hash",
		},
	}

	passwords := &fakePasswordComparer{}

	auth := NewLoginService(
		users,
		passwords,
		nil,
		24*time.Hour,
	)

	user, err := auth.Login(validation.LoginInput{
		Identifier: "  Lefteris@Example.COM  ",
		Password:   "strong-password-123",
	})
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	if user.ID != 42 {
		t.Fatalf("user.ID = %d, want 42", user.ID)
	}

	if !users.called {
		t.Fatal("ByIdentifier() was not called")
	}

	if users.identifier != "lefteris@example.com" {
		t.Fatalf(
			"ByIdentifier() identifier = %q, want normalized email",
			users.identifier,
		)
	}

	if !passwords.called {
		t.Fatal("Compare() was not called")
	}

	if passwords.hash != "stored-hash" {
		t.Fatalf(
			"Compare() hash = %q, want stored-hash",
			passwords.hash,
		)
	}

	if passwords.password != "strong-password-123" {
		t.Fatal("Compare() received wrong password")
	}
}

func TestLoginWithNickname(t *testing.T) {
	users := &fakeUserFinder{
		user: model.User{
			ID:           42,
			Username:     "lefteris",
			Email:        "lefteris@example.com",
			PasswordHash: "stored-hash",
		},
	}
	passwords := &fakePasswordComparer{}
	auth := NewLoginService(users, passwords, nil, 24*time.Hour)

	user, err := auth.Login(validation.LoginInput{
		Identifier: "  Lefteris  ",
		Password:   "strong-password-123",
	})
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if user.ID != 42 {
		t.Fatalf("user.ID = %d, want 42", user.ID)
	}
	if users.identifier != "lefteris" {
		t.Fatalf("lookup identifier = %q, want %q", users.identifier, "lefteris")
	}
	if passwords.hash != "stored-hash" || passwords.password != "strong-password-123" {
		t.Fatalf("Compare() got hash %q and password %q", passwords.hash, passwords.password)
	}
}

func TestLoginInvalidCredentialsAreIndistinguishable(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		user       model.User
		userErr    error
		compareErr error
	}{
		{
			name:       "unknown email",
			identifier: "missing@example.com",
			userErr:    repository.ErrUserNotFound,
		},
		{
			name:       "unknown nickname",
			identifier: "missing-user",
			userErr:    repository.ErrUserNotFound,
		},
		{
			name:       "wrong password",
			identifier: "lefteris@example.com",
			user: model.User{
				ID:           42,
				Email:        "lefteris@example.com",
				PasswordHash: "stored-hash",
			},
			compareErr: errors.New("password mismatch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUserFinder{
				user: tt.user,
				err:  tt.userErr,
			}

			passwords := &fakePasswordComparer{
				err: tt.compareErr,
			}

			auth := NewLoginService(
				users,
				passwords,
				nil,
				24*time.Hour,
			)

			_, err := auth.Login(validation.LoginInput{
				Identifier: tt.identifier,
				Password:   "wrong-password",
			})

			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf(
					"Login() error = %v, want ErrInvalidCredentials",
					err,
				)
			}

			if !passwords.called {
				t.Fatal("Compare() was not called")
			}
		})
	}
}
func TestLoginInvalidInputStopsEarly(t *testing.T) {
	users := &fakeUserFinder{}

	passwords := &fakePasswordComparer{}

	auth := NewLoginService(
		users,
		passwords,
		nil,
		24*time.Hour,
	)

	_, err := auth.Login(validation.LoginInput{
		Identifier: "not-an@email",
		Password:   "strong-password-123",
	})

	if err == nil {
		t.Fatal("Login() error = nil, want validation error")
	}

	if users.called {
		t.Fatal("ByIdentifier() was called for invalid input")
	}

	if passwords.called {
		t.Fatal("Compare() was called for invalid input")
	}
}

type fakeSessionStore struct {
	replaceCalled bool
	deleteCalled  bool

	id        string
	userID    int64
	expiresAt time.Time

	deleteID string

	err error
}

func (f *fakeSessionStore) Replace(
	id string,
	userID int64,
	expiresAt time.Time,
) error {
	f.replaceCalled = true
	f.id = id
	f.userID = userID
	f.expiresAt = expiresAt

	return f.err
}

func (f *fakeSessionStore) Delete(id string) error {
	f.deleteCalled = true
	f.deleteID = id

	return f.err
}
func TestLoginCreatesSession(t *testing.T) {
	users := &fakeUserFinder{
		user: model.User{
			ID:           42,
			Email:        "lefteris@example.com",
			PasswordHash: "stored-hash",
		},
	}

	passwords := &fakePasswordComparer{}

	sessions := &fakeSessionStore{}

	auth := NewLoginService(
		users,
		passwords,
		sessions,
		24*time.Hour,
	)

	err := auth.CreateSession(
		"session-123",
		42,
	)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	if !sessions.replaceCalled {
		t.Fatal("session Replace() was not called")
	}

	if sessions.id != "session-123" {
		t.Fatalf(
			"session id = %q, want %q",
			sessions.id,
			"session-123",
		)
	}

	if sessions.userID != 42 {
		t.Fatalf(
			"userID = %d, want 42",
			sessions.userID,
		)
	}
}
func TestLogoutDeletesSession(t *testing.T) {
	sessions := &fakeSessionStore{}

	auth := NewLoginService(
		nil,
		nil,
		sessions,
		24*time.Hour,
	)

	err := auth.Logout("session-123")
	if err != nil {
		t.Fatalf("Logout() error: %v", err)
	}

	if !sessions.deleteCalled {
		t.Fatal("session Delete() was not called")
	}

	if sessions.deleteID != "session-123" {
		t.Fatalf(
			"deleted session id = %q, want %q",
			sessions.deleteID,
			"session-123",
		)
	}
}
