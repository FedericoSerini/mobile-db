package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/federicoserini/mobile-db/server"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg, log); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
