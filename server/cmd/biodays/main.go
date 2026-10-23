// @title           biodays API
// @version         1.0
// @description     实验室科学史上的今天 · 公开 REST API
// @BasePath        /
// @schemes         http https
//
// biodays is a single binary with subcommands:
//
//	biodays serve    migrate schema, auto-seed if configured, serve the HTTP API (default)
//	biodays seed     import stories_*.json from BIODAYS_DATA_DIR and exit
//	biodays migrate  create/update the database schema and exit
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"biodays/server/internal/config"
	"biodays/server/internal/router"
	"biodays/server/internal/seed"
	"biodays/server/internal/store"
)

const usage = `usage: biodays [command]

commands:
  serve     migrate schema, auto-seed if BIODAYS_DATA_DIR is set, serve the HTTP API (default)
  seed [--strict]  import stories_*.json; --strict rejects skipped/failed/empty input
  migrate   create/update the database schema and exit
`

func main() {
	command := "serve"
	args := os.Args[1:]
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "serve":
		serve()
	case "seed":
		runSeed(args[1:])
	case "migrate":
		migrate()
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, usage)
		os.Exit(2)
	}
}

// loadConfigAndLogger parses the environment and installs the default logger.
func loadConfigAndLogger() (config.Config, *slog.Logger) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	level, _ := config.ParseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return cfg, logger
}

// openStore opens the database and applies schema migrations (GORM AutoMigrate).
func openStore(cfg config.Config, logger *slog.Logger) *store.Store {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open database", "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	return st
}

// autoSeed imports stories when BIODAYS_DATA_DIR points at a directory that
// actually contains stories_*.json files. The import is an idempotent upsert,
// so running it on every start is safe. Without a source directory, validate
// the database bundled in the image instead.
func autoSeed(ctx context.Context, st *store.Store, cfg config.Config, logger *slog.Logger) error {
	if cfg.DataDir == "" {
		return checkStoryData(ctx, st, logger)
	}
	files, err := filepath.Glob(filepath.Join(cfg.DataDir, "stories_*.json"))
	if err != nil {
		return fmt.Errorf("scan %s: %w", cfg.DataDir, err)
	}
	if len(files) == 0 {
		logger.Warn("auto-seed: no stories_*.json found; check data mount and permissions", "dir", cfg.DataDir)
	} else {
		logger.Info("auto-seed starting", "dir", cfg.DataDir, "files", len(files))
		report, err := seed.Run(ctx, st, cfg.DataDir, logger)
		if err != nil {
			return err
		}
		logger.Info("auto-seed finished",
			"imported", report.Imported,
			"skipped", report.Skipped,
			"files", report.Files,
		)
	}
	return checkStoryData(ctx, st, logger)
}

func checkStoryData(ctx context.Context, st *store.Store, logger *slog.Logger) error {
	total, _, err := st.Stats(ctx)
	if err != nil {
		return fmt.Errorf("check story data: %w", err)
	}
	if total == 0 {
		return errors.New("database has no stories; use an image with bundled data or import data with biodays seed")
	}
	logger.Info("story data ready", "total", total)
	return nil
}

func serve() {
	cfg, logger := loadConfigAndLogger()
	st := openStore(cfg, logger)
	defer st.Close()

	if err := autoSeed(context.Background(), st, cfg, logger); err != nil {
		logger.Error("auto-seed failed", "error", err)
		os.Exit(1)
	}

	engine, err := router.New(cfg, st, logger)
	if err != nil {
		logger.Error("build router", "error", err)
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		<-signals

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("shutting down")
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("biodays api listening", "addr", server.Addr, "db", cfg.DBPath)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
	<-shutdownDone
}

func runSeed(args []string) {
	flags := flag.NewFlagSet("seed", flag.ExitOnError)
	strict := flags.Bool("strict", false, "fail on skipped stories, unreadable/invalid files, or empty input")
	flags.Parse(args)
	cfg, logger := loadConfigAndLogger()
	if cfg.DataDir == "" {
		fmt.Fprintln(os.Stderr, "config: BIODAYS_DATA_DIR is required for seeding")
		os.Exit(1)
	}

	st := openStore(cfg, logger)
	defer st.Close()

	report, err := seed.Run(context.Background(), st, cfg.DataDir, logger)
	if err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}
	if *strict && (report.Imported == 0 || report.Skipped > 0 || report.FailedFiles > 0) {
		logger.Error("strict seed failed", "imported", report.Imported, "skipped", report.Skipped, "failed_files", report.FailedFiles)
		os.Exit(1)
	}
	fmt.Printf("imported=%d skipped=%d files=%d failed_files=%d\n", report.Imported, report.Skipped, report.Files, report.FailedFiles)
}

func migrate() {
	cfg, logger := loadConfigAndLogger()
	st := openStore(cfg, logger)
	defer st.Close()
	logger.Info("schema up to date", "db", cfg.DBPath)
}
