package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"biodays/server/internal/config"
	"biodays/server/internal/seed"
	"biodays/server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if cfg.DataDir == "" {
		fmt.Fprintln(os.Stderr, "config: BIODAYS_DATA_DIR is required for seeding")
		os.Exit(1)
	}
	level, _ := config.ParseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open database", "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer st.Close()

	report, err := seed.Run(context.Background(), st, cfg.DataDir, logger)
	if err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("imported=%d skipped=%d files=%d\n", report.Imported, report.Skipped, report.Files)
}
