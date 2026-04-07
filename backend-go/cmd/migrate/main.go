package main

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"landrisk/backend-go/internal/config"
	"landrisk/backend-go/internal/db"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("db connection failed")
	}
	defer pool.Close()

	migrationsDir := "/app/migrations"
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "./migrations"
	}

	if err := db.ApplyMigrations(ctx, pool, migrationsDir); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
	log.Info().Msg("migrations applied")
}
