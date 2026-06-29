//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/example/go-project/internal/storage"
)

func mysqlTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEST_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN/TEST_MYSQL_DSN not set, skipping integration test")
	}
	return dsn
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	return filepath.ToSlash(abs)
}

func migrationsURL(t *testing.T) string {
	t.Helper()
	_ = migrationsDir(t)
	return "file://./"
}

func TestMigrationUpDown(t *testing.T) {
	dsn := mysqlTestDSN(t)
	restore := withMigrationsCWD(t)
	defer restore()
	murl := migrationsURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := storage.OpenMySQL(ctx, storage.MySQLConfig{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenMySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := runMigrations(dsn, murl, "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	t.Cleanup(func() { runMigrations(dsn, murl, "down") })

	wantTables := []string{
		"users", "teams", "team_members",
		"tasks", "task_history", "task_comments",
	}
	for _, tbl := range wantTables {
		var n int
		err := db.GetContext(ctx, &n,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			tbl)
		if err != nil {
			t.Fatalf("check table %s: %v", tbl, err)
		}
		if n != 1 {
			t.Errorf("table %s not found (count=%d)", tbl, n)
		}
	}
}

func runMigrations(dsn, url, cmd string) error {
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	drv, err := migmysql.WithInstance(sqlDB, &migmysql.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(url, "mysql", drv)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return err
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return err
		}
	default:
		return fmt.Errorf("unknown cmd %q", cmd)
	}
	return nil
}

func withMigrationsCWD(t *testing.T) func() {
	t.Helper()
	mdir, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("abs migrations: %v", err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(mdir); err != nil {
		t.Fatalf("chdir migrations: %v", err)
	}
	return func() { _ = os.Chdir(orig) }
}

func TestSchemaInsertSmoke(t *testing.T) {
	dsn := mysqlTestDSN(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := storage.OpenMySQL(ctx, storage.MySQLConfig{DSN: dsn, MaxOpenConns: 2})
	if err != nil {
		t.Fatalf("OpenMySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`,
		email, "h", "Test User")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, _ := res.LastInsertId()

	res, err = tx.ExecContext(ctx,
		`INSERT INTO teams (name, created_by) VALUES (?, ?)`,
		"Test Team", userID)
	if err != nil {
		t.Fatalf("insert team: %v", err)
	}
	teamID, _ := res.LastInsertId()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO team_members (user_id, team_id, role) VALUES (?, ?, 'owner')`,
		userID, teamID)
	if err != nil {
		t.Fatalf("insert member: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM team_members WHERE team_id = ?`, teamID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM teams WHERE id = ?`, teamID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM users WHERE id = ?`, userID)
	})

	var gotTeam struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	err = db.GetContext(ctx, &gotTeam,
		`SELECT id, name FROM teams WHERE id = ?`, teamID)
	if err != nil {
		t.Fatalf("select team: %v", err)
	}
	if gotTeam.Name != "Test Team" {
		t.Errorf("team name = %q, want %q", gotTeam.Name, "Test Team")
	}
}
