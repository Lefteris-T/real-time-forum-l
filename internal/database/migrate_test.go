package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateAppliesMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	migrationsDir := t.TempDir()

	err = os.WriteFile(
		filepath.Join(migrationsDir, "001_test.sql"),
		[]byte(`
			CREATE TABLE test_items (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL
			);
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	var name string

	err = db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'test_items'
	`).Scan(&name)
	if err != nil {
		t.Fatalf("test_items table was not created: %v", err)
	}

	if name != "test_items" {
		t.Fatalf("table name = %q, want %q", name, "test_items")
	}
}
func TestMigrateDoesNotReapplyCompletedMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	migrationsDir := t.TempDir()

	migrationPath := filepath.Join(migrationsDir, "001_test.sql")

	err = os.WriteFile(
		migrationPath,
		[]byte(`
			CREATE TABLE test_items (
				id INTEGER PRIMARY KEY
			);
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("first Migrate() error: %v", err)
	}

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("second Migrate() error: %v", err)
	}

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = 1
	`).Scan(&count)
	if err != nil {
		t.Fatalf("query schema_migrations error: %v", err)
	}

	if count != 1 {
		t.Fatalf("migration version count = %d, want 1", count)
	}
}
func TestMigrateAppliesOnlyNewMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	migrationsDir := t.TempDir()

	firstMigration := filepath.Join(migrationsDir, "001_first.sql")

	err = os.WriteFile(
		firstMigration,
		[]byte(`
			CREATE TABLE first_table (
				id INTEGER PRIMARY KEY
			);
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() first migration error: %v", err)
	}

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("first Migrate() error: %v", err)
	}

	secondMigration := filepath.Join(migrationsDir, "002_second.sql")

	err = os.WriteFile(
		secondMigration,
		[]byte(`
			CREATE TABLE second_table (
				id INTEGER PRIMARY KEY
			);
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() second migration error: %v", err)
	}

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("second Migrate() error: %v", err)
	}

	var count int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM schema_migrations
	`).Scan(&count)
	if err != nil {
		t.Fatalf("query schema_migrations error: %v", err)
	}

	if count != 2 {
		t.Fatalf("migration count = %d, want 2", count)
	}
}
func TestMigrateRollsBackFailedMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "forum.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	migrationsDir := t.TempDir()

	err = os.WriteFile(
		filepath.Join(migrationsDir, "001_broken.sql"),
		[]byte(`
			CREATE TABLE should_not_exist (
				id INTEGER PRIMARY KEY
			);

			THIS IS INVALID SQL;
		`),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	err = Migrate(db, migrationsDir)
	if err == nil {
		t.Fatal("Migrate() error = nil, want error")
	}

	var tableCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table'
		  AND name = 'should_not_exist'
	`).Scan(&tableCount)
	if err != nil {
		t.Fatalf("query sqlite_master error: %v", err)
	}

	if tableCount != 0 {
		t.Fatalf(
			"should_not_exist table count = %d, want 0",
			tableCount,
		)
	}

	var migrationCount int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = 1
	`).Scan(&migrationCount)
	if err != nil {
		t.Fatalf("query schema_migrations error: %v", err)
	}

	if migrationCount != 0 {
		t.Fatalf(
			"failed migration recorded %d times, want 0",
			migrationCount,
		)
	}
}
