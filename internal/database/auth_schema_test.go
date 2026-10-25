package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestAuthMigrationCreatesUsersAndSessions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	expectedTables := []string{
		"users",
		"sessions",
	}

	for _, table := range expectedTables {
		var count int

		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM sqlite_master
			WHERE type = 'table' AND name = ?
		`, table).Scan(&count)
		if err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}

		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", table, count)
		}
	}
}
func TestAuthSchemaRejectsDuplicateEmailUsernameAndSessionUser(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (
			email,
			username,
			password_hash,
			created_at,
			age,
			gender,
			first_name,
			last_name
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"user@example.com",
		"lefteris",
		"hash",
		"2026-08-09T10:00:00Z",
		30,
		"unspecified",
		"Test",
		"User",
	)
	if err != nil {
		t.Fatalf("insert first user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (
			email,
			username,
			password_hash,
			created_at,
			age,
			gender,
			first_name,
			last_name
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"user@example.com",
		"another-user",
		"hash",
		"2026-08-09T10:00:00Z",
		30,
		"unspecified",
		"Test",
		"User",
	)
	if err == nil {
		t.Fatal("duplicate email was accepted")
	}

	_, err = db.Exec(`
		INSERT INTO users (
			email,
			username,
			password_hash,
			created_at,
			age,
			gender,
			first_name,
			last_name
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"another@example.com",
		"lefteris",
		"hash",
		"2026-08-09T10:00:00Z",
		30,
		"unspecified",
		"Test",
		"User",
	)
	if err == nil {
		t.Fatal("duplicate username was accepted")
	}

	var userID int64

	err = db.QueryRow(`
		SELECT id
		FROM users
		WHERE email = ?
	`, "user@example.com").Scan(&userID)
	if err != nil {
		t.Fatalf("select user id: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sessions (
			id,
			user_id,
			expires_at,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		"session-1",
		userID,
		"2026-08-10T10:00:00Z",
		"2026-08-09T10:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert first session: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sessions (
			id,
			user_id,
			expires_at,
			created_at
		)
		VALUES (?, ?, ?, ?)
	`,
		"session-2",
		userID,
		"2026-08-10T10:00:00Z",
		"2026-08-09T10:00:00Z",
	)
	if err == nil {
		t.Fatal("second session for same user was accepted")
	}
}
func TestUserSchemaHasRequiredProfileFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	if err := Migrate(db, "../../migrations"); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		t.Fatalf("query users schema: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)

	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)

		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("scan users schema: %v", err)
		}

		columns[name] = true
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate users schema: %v", err)
	}

	for _, required := range []string{
		"age",
		"gender",
		"first_name",
		"last_name",
	} {
		if !columns[required] {
			t.Errorf("users table is missing column %q", required)
		}
	}
}
