package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const defaultDSN = "MYSQL_DSN"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: migrate <up|down|version|force|steps> [N|V]")
	}

	dsn := os.Getenv(defaultDSN)
	if dsn == "" {
		return errors.New("MYSQL_DSN is not set")
	}

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Fprintf(os.Stderr, "close source: %v\n", srcErr)
		}
		if dbErr != nil {
			fmt.Fprintf(os.Stderr, "close db: %v\n", dbErr)
		}
	}()

	cmd := os.Args[1]
	switch cmd {
	case "up":

		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("up: %w", err)
		}
		printVersion(m)

	case "down":
		// Down() — откатить одну. Для массового отката — steps.
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("down: %w", err)
		}
		printVersion(m)

	case "steps":
		if len(os.Args) < 3 {
			return errors.New("usage: migrate steps N")
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			return fmt.Errorf("invalid N: %w", err)
		}
		if err := m.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("steps: %w", err)
		}
		printVersion(m)

	case "version":
		printVersion(m)

	case "force":
		if len(os.Args) < 3 {
			return errors.New("usage: migrate force V")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			return fmt.Errorf("invalid V: %w", err)
		}
		if err := m.Force(v); err != nil {
			return fmt.Errorf("force: %w", err)
		}
		printVersion(m)

	default:
		return fmt.Errorf("unknown command: %q", cmd)
	}
	return nil
}

func printVersion(m *migrate.Migrate) {
	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("version: <none> (no migrations applied yet)")
			return
		}
		fmt.Fprintf(os.Stderr, "version: %v\n", err)
		return
	}
	fmt.Printf("version: %d, dirty: %v\n", v, dirty)
}
