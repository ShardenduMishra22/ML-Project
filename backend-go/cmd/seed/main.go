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

	seedDir := "/app/seed"
	if _, err := os.Stat(seedDir); err != nil {
		seedDir = "./seed"
	}

	if err := db.ApplySeed(ctx, pool, seedDir); err != nil {
		log.Fatal().Err(err).Msg("seed failed")
	}
	log.Info().Msg("seed applied")
}
