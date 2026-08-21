// Command migrate applies SQL migrations from ./migrations using the
// same .env-driven configuration as the server, so credentials stay in
// one place. Usage: go run ./cmd/migrate [-dir up|down] [-steps N]
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/diyorbek/islamiccalculator/internal/config"
)

func main() {
	dir := flag.String("dir", "up", "direction: up or down")
	steps := flag.Int("steps", 0, "number of migrations (0 = all up / 1 down)")
	path := flag.String("path", "migrations", "migrations directory")
	flag.Parse()

	if err := run(*dir, *steps, *path); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
}

func run(dir string, steps int, path string) error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	m, err := migrate.New("file://"+path, cfg.DB.DSN())
	if err != nil {
		return err
	}
	defer m.Close()

	switch dir {
	case "up":
		if steps > 0 {
			err = m.Steps(steps)
		} else {
			err = m.Up()
		}
	case "down":
		if steps == 0 {
			steps = 1 // never roll back everything by accident
		}
		err = m.Steps(-steps)
	default:
		return fmt.Errorf("unknown direction %q", dir)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("no pending migrations")
		return nil
	}
	if err != nil {
		return err
	}

	version, dirty, _ := m.Version()
	slog.Info("migrations applied", "direction", dir, "version", version, "dirty", dirty)
	return nil
}
