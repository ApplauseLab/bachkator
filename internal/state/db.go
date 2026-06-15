package state

import (
	"database/sql"
	"embed"
	"errors"
	"net/url"
	"os"

	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func openDB(path string) (*sql.DB, error) {
	resolved, err := evidence.PrepareStatePath(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := enableConnectionPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func openReadOnlyDB(path string) (*sql.DB, error) {
	abs, err := evidence.ResolveStatePath("", path)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: abs}).String() + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := enableConnectionPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func enableConnectionPragmas(db *sql.DB) error {
	_, err := db.Exec(`
		PRAGMA busy_timeout = 30000;
		PRAGMA foreign_keys = ON;
	`)
	return err
}

func initSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
	`); err != nil {
		return err
	}
	return runMigrations(db)
}

func runMigrations(db *sql.DB) error {
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}
	driver, err := migratesqlite.WithInstance(
		db,
		&migratesqlite.Config{MigrationsTable: "bach_schema_migrations"},
	)
	if err != nil {
		return err
	}
	migrator, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return err
	}
	if err := migrator.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err
	}
	return nil
}
