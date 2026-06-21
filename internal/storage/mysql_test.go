package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenMySQL_Integration(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := OpenMySQL(ctx, MySQLConfig{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenMySQL: %v", err)
	}
	defer db.Close()

	// проверка: пул реально работает.
	var one int
	if err := db.GetContext(ctx, &one, "SELECT 1"); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if one != 1 {
		t.Errorf("got %d, want 1", one)
	}
}

func TestOpenMySQL_BadDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := OpenMySQL(ctx, MySQLConfig{DSN: "not-a-real-dsn"})
	if err == nil {
		t.Fatal("expected error for bad DSN, got nil")
	}
}
