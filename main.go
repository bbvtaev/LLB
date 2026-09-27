// LLB (Learning Language Bot): a single-user Telegram bot for learning words with spaced repetition.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // TZ works in a scratch/distroless image

	"github.com/jackc/pgx/v5/pgxpool"

	"llb/internal/bot"
	"llb/internal/store"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, dbURL, err := loadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	migrationsFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	if err := store.Migrate(ctx, pool, migrationsFS); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	app, err := bot.New(cfg, store.New(pool))
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	log.Printf("LLB started, reminders at %v (%s)", cfg.RemindAt, time.Local)
	app.Run(ctx)
	return nil
}

func loadConfig() (bot.Config, string, error) {
	cfg := bot.Config{Token: os.Getenv("TG_TOKEN")}
	if cfg.Token == "" {
		return cfg, "", fmt.Errorf("TG_TOKEN is not set")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return cfg, "", fmt.Errorf("DATABASE_URL is not set")
	}
	if s := os.Getenv("OWNER_ID"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return cfg, "", fmt.Errorf("OWNER_ID: %w", err)
		}
		cfg.OwnerID = id
	} else {
		log.Print("OWNER_ID is not set: the bot will only reply with the sender's id")
	}

	remind, ok := os.LookupEnv("REMIND_AT")
	if !ok {
		remind = "10:00,20:00"
	}
	for _, s := range strings.Split(remind, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		t, err := time.Parse("15:04", s)
		if err != nil {
			return cfg, "", fmt.Errorf("REMIND_AT %q: want HH:MM", s)
		}
		cfg.RemindAt = append(cfg.RemindAt, t.Format("15:04"))
	}
	return cfg, dbURL, nil
}
