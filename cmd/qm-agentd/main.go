package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/dscof/qm-agent/internal/config"
	"github.com/dscof/qm-agent/internal/daemon"
	"github.com/dscof/qm-agent/internal/identity"
)

func main() {
	configPath := flag.String("config", envOr("QM_AGENT_CONFIG", "config.yaml"), "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	id, err := identity.New(ctx, cfg)
	if err != nil {
		logger.Error("identity source", "error", err)
		os.Exit(1)
	}

	d, err := daemon.New(cfg, id, logger)
	if err != nil {
		logger.Error("daemon init", "error", err)
		os.Exit(1)
	}

	logger.Info("starting qm-agentd",
		"quartermaster", cfg.Quartermaster.URL,
		"identity", cfg.Identity.Type,
		"output_dir", cfg.Output.Dir,
		"billets", cfg.Exchange.Billets,
	)

	if err := d.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("daemon stopped", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
