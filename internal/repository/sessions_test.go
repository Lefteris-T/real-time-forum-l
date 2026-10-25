package repository

import (
	"path/filepath"
	"testing"
	"time"

	"forum/internal/database"
)

func TestSessionRepositoryCreateAndFind(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	userResult, err := db.Exec(`
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
		"lefteris@example.com",
		"lefteris",
		30,
		"test",
		"Test",
		"User",
		"hash",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	userID, err := userResult.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error: %v", err)
	}

	repo := NewSessionRepository(db)

	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	err = repo.Replace(
		"session-1",
		userID,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("Replace() error: %v", err)
	}

	session, err := repo.ByID("session-1")
	if err != nil {
		t.Fatalf("ByID() error: %v", err)
	}

	if session.ID != "session-1" {
		t.Fatalf(
			"session.ID = %q, want %q",
			session.ID,
			"session-1",
		)
	}

	if session.UserID != userID {
		t.Fatalf(
			"session.UserID = %d, want %d",
			session.UserID,
			userID,
		)
	}
}
func TestSessionRepositoryReplaceRemovesOldSession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	userResult, err := db.Exec(`
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
		"lefteris@example.com",
		"lefteris",
		30,
		"test",
		"Test",
		"User",
		"hash",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	userID, err := userResult.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error: %v", err)
	}

	repo := NewSessionRepository(db)

	err = repo.Replace(
		"session-old",
		userID,
		time.Now().UTC().Add(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("first Replace() error: %v", err)
	}

	err = repo.Replace(
		"session-new",
		userID,
		time.Now().UTC().Add(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("second Replace() error: %v", err)
	}

	_, err = repo.ByID("session-old")
	if err != ErrSessionNotFound {
		t.Fatalf(
			"old session error = %v, want ErrSessionNotFound",
			err,
		)
	}

	session, err := repo.ByID("session-new")
	if err != nil {
		t.Fatalf("new session ByID() error: %v", err)
	}

	if session.UserID != userID {
		t.Fatalf(
			"new session userID = %d, want %d",
			session.UserID,
			userID,
		)
	}
}
func TestSessionRepositoryDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	userResult, err := db.Exec(`
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
		"lefteris@example.com",
		"lefteris",
		30,
		"test",
		"Test",
		"User",
		"hash",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	userID, err := userResult.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error: %v", err)
	}

	repo := NewSessionRepository(db)

	err = repo.Replace(
		"session-1",
		userID,
		time.Now().UTC().Add(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("Replace() error: %v", err)
	}

	if err := repo.Delete("session-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = repo.ByID("session-1")
	if err != ErrSessionNotFound {
		t.Fatalf(
			"ByID() error = %v, want ErrSessionNotFound",
			err,
		)
	}
}

func TestSessionRepositoryDeletesExpiredSessions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	userResult, err := db.Exec(`
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
		"lefteris@example.com",
		"lefteris",
		30,
		"test",
		"Test",
		"User",
		"hash",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	userID, err := userResult.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error: %v", err)
	}

	repo := NewSessionRepository(db)

	err = repo.Replace(
		"expired-session",
		userID,
		time.Now().UTC().Add(-time.Hour),
	)
	if err != nil {
		t.Fatalf("Replace() error: %v", err)
	}

	if err := repo.DeleteExpired(time.Now().UTC()); err != nil {
		t.Fatalf("DeleteExpired() error: %v", err)
	}

	_, err = repo.ByID("expired-session")
	if err != ErrSessionNotFound {
		t.Fatalf(
			"ByID() error = %v, want ErrSessionNotFound",
			err,
		)
	}
}
func TestSessionRepositoryReplaceRollsBackOnFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open() error: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("database.Migrate() error: %v", err)
	}

	userResult, err := db.Exec(`
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
		"lefteris@example.com",
		"lefteris",
		30,
		"test",
		"Test",
		"User",
		"hash",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	userID, err := userResult.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error: %v", err)
	}

	repo := NewSessionRepository(db)

	err = repo.Replace(
		"session-old",
		userID,
		time.Now().UTC().Add(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("first Replace() error: %v", err)
	}

	// Force the next insert to fail.
	_, err = db.Exec(`
		CREATE TRIGGER fail_session_insert
		BEFORE INSERT ON sessions
		BEGIN
			SELECT RAISE(ABORT, 'forced session insert failure');
		END;
	`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	err = repo.Replace(
		"session-new",
		userID,
		time.Now().UTC().Add(24*time.Hour),
	)
	if err == nil {
		t.Fatal("Replace() error = nil, want error")
	}

	oldSession, err := repo.ByID("session-old")
	if err != nil {
		t.Fatalf("old session was lost: %v", err)
	}

	if oldSession.UserID != userID {
		t.Fatalf(
			"old session userID = %d, want %d",
			oldSession.UserID,
			userID,
		)
	}

	_, err = repo.ByID("session-new")
	if err != ErrSessionNotFound {
		t.Fatalf(
			"new session error = %v, want ErrSessionNotFound",
			err,
		)
	}
}
