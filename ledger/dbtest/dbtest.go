package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/pressly/goose/v3"

	"ledger/cache"
	"ledger/db"
	"ledger/migrations"
)

const defaultURL = "postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable"

var (
	once     sync.Once
	setupErr error
)

func Setup(t testing.TB) {
	t.Helper()

	once.Do(func() { setupErr = connect() })
	if setupErr != nil {
		t.Skipf("test database is not available: %v", setupErr)
	}
}

func SetupCache(t testing.TB) {
	t.Helper()

	cfg := cache.Config{Addr: os.Getenv("TEST_REDIS_ADDR"), DB: 15}
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if v := os.Getenv("TEST_REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("invalid TEST_REDIS_DB %q", v)
		}
		cfg.DB = n
	}

	client, err := cache.Connect(context.Background(), cfg)
	if err != nil {
		t.Skipf("test redis is not available: %v", err)
	}
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush test redis: %v", err)
	}

	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
		cache.Client = nil
	})
}

func connect() error {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultURL
	}

	if err := ensureDatabase(dsn); err != nil {
		return err
	}

	conn, err := db.Connect(context.Background(), dsn)
	if err != nil {
		return err
	}

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(conn, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func ensureDatabase(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return fmt.Errorf("TEST_DATABASE_URL has no database name")
	}

	admin := *u
	admin.Path = "/postgres"
	conn, err := sql.Open("pgx", admin.String())
	if err != nil {
		return err
	}
	defer conn.Close()

	var exists bool
	if err := conn.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return fmt.Errorf("check database %q: %w", name, err)
	}
	if exists {
		return nil
	}
	if _, err := conn.Exec(`CREATE DATABASE ` + quoteIdent(name)); err != nil {
		return fmt.Errorf("create database %q: %w", name, err)
	}
	return nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
