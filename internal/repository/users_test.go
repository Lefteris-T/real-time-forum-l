package repository

import (
	"database/sql"
	"path/filepath"
	"testing"

	"forum/internal/database"
	"forum/internal/model"
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

func TestUserRepositoryPublicProfileAndOwnerUpdateFields(t *testing.T) {
	db, repo := newUserTestDB(t)
	id, err := insertTestUser(repo, "private@example.com", "public-name", "private-hash")
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	profile, err := repo.PublicProfileByID(id)
	if err != nil {
		t.Fatalf("PublicProfileByID(): %v", err)
	}
	if profile.Username != "public-name" || profile.FirstName != "Test" || profile.LastName != "User" || profile.Age != 30 {
		t.Fatalf("public profile = %+v", profile)
	}
	if err := repo.UpdateProfile(id, model.ProfileUpdate{FirstName: "New", LastName: "Name", Age: 40, Gender: "other"}); err != nil {
		t.Fatalf("UpdateProfile(): %v", err)
	}
	profile, err = repo.PublicProfileByID(id)
	if err != nil || profile.FirstName != "New" || profile.LastName != "Name" || profile.Age != 40 || profile.Gender != "other" {
		t.Fatalf("updated profile = %+v, error %v", profile, err)
	}
	var email, passwordHash string
	if err := db.QueryRow("SELECT email, password_hash FROM users WHERE id = ?", id).Scan(&email, &passwordHash); err != nil {
		t.Fatalf("query private fields: %v", err)
	}
	if email != "private@example.com" || passwordHash != "private-hash" {
		t.Fatalf("profile update changed private fields: %q/%q", email, passwordHash)
	}
	if err := repo.UpdateProfile(id+1000, model.ProfileUpdate{FirstName: "Test", LastName: "User", Age: 30, Gender: "test"}); err != ErrUserNotFound {
		t.Fatalf("UpdateProfile(missing) error = %v", err)
	}
}

func newUserTestDB(t *testing.T) (*sql.DB, *UserRepository) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "forum.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db, NewUserRepository(db)
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
