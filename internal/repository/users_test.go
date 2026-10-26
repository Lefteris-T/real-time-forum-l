package repository

import (
	"path/filepath"
	"testing"

	"forum/internal/database"
)

func TestUserRepositoryCreateAndFindByEmail(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	repo := NewUserRepository(db)

	id, err := insertTestUser(repo, "lefteris@example.com", "lefteris", "bcrypt-hash")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	user, err := repo.ByIdentifier("lefteris@example.com")
	if err != nil {
		t.Fatalf("ByIdentifier(email) error: %v", err)
	}

	if user.ID != id {
		t.Errorf("user.ID = %d, want %d", user.ID, id)
	}

	if user.Email != "lefteris@example.com" {
		t.Errorf(
			"user.Email = %q, want %q",
			user.Email,
			"lefteris@example.com",
		)
	}

	if user.Username != "lefteris" {
		t.Errorf(
			"user.Username = %q, want %q",
			user.Username,
			"lefteris",
		)
	}

	if user.Age != 30 || user.Gender != "test" || user.FirstName != "Test" || user.LastName != "User" {
		t.Fatalf("profile = %+v, want persisted profile fields", user)
	}

	if user.PasswordHash != "bcrypt-hash" {
		t.Errorf(
			"user.PasswordHash = %q, want %q",
			user.PasswordHash,
			"bcrypt-hash",
		)
	}

	byNickname, err := repo.ByIdentifier("lefteris")
	if err != nil {
		t.Fatalf("ByIdentifier(nickname) error: %v", err)
	}
	if byNickname.ID != id {
		t.Errorf("nickname lookup ID = %d, want %d", byNickname.ID, id)
	}
}

func TestUserRepositoryFindByID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	repo := NewUserRepository(db)

	id, err := insertTestUser(repo, "lefteris@example.com", "lefteris", "bcrypt-hash")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	user, err := repo.ByID(id)
	if err != nil {
		t.Fatalf("ByID() error: %v", err)
	}

	if user.ID != id {
		t.Errorf("user.ID = %d, want %d", user.ID, id)
	}
	if user.Age != 30 || user.Gender != "test" || user.FirstName != "Test" || user.LastName != "User" {
		t.Fatalf("profile = %+v, want persisted profile fields", user)
	}
}
func TestUserRepositoryReturnsNotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	repo := NewUserRepository(db)

	_, err = repo.ByEmail("missing@example.com")
	if err != ErrUserNotFound {
		t.Fatalf("ByEmail() error = %v, want ErrUserNotFound", err)
	}

	_, err = repo.ByID(9999)
	if err != ErrUserNotFound {
		t.Fatalf("ByID() error = %v, want ErrUserNotFound", err)
	}

	for _, identifier := range []string{"missing@example.com", "missing-user"} {
		if _, err := repo.ByIdentifier(identifier); err != ErrUserNotFound {
			t.Errorf(
				"ByIdentifier(%q) error = %v, want ErrUserNotFound",
				identifier,
				err,
			)
		}
	}
}

func TestUserRepositoryReturnsConflictErrors(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	repo := NewUserRepository(db)

	_, err = insertTestUser(repo, "lefteris@example.com", "lefteris", "bcrypt-hash")
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = insertTestUser(repo, "lefteris@example.com", "another-user", "bcrypt-hash")
	if err != ErrEmailExists {
		t.Fatalf("duplicate email error = %v, want ErrEmailExists", err)
	}

	_, err = insertTestUser(repo, "another@example.com", "lefteris", "bcrypt-hash")
	if err != ErrUsernameExists {
		t.Fatalf("duplicate username error = %v, want ErrUsernameExists", err)
	}
}
