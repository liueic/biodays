// @title           biodays API
// @version         1.0
// @description     实验室科学史上的今天 · 公开 REST API 与反馈/管理接口
// @BasePath        /
// @schemes         http https
//
// biodays is a single binary with subcommands:
//
//	biodays serve    migrate schema, auto-seed if configured, serve the HTTP API (default)
//	biodays seed     import stories_*.json from BIODAYS_DATA_DIR and exit
//	biodays migrate  create/update the database schema and exit
//	biodays hash-password  read a password and print a bcrypt hash
//	biodays cleanup  apply feedback retention rules
//	biodays backup   write consistent SQLite snapshots and prune old copies
//	biodays restore  replace a database from a backup (sessions revoked, mail paused)
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"biodays/server/internal/config"
	"biodays/server/internal/mailer"
	"biodays/server/internal/router"
	"biodays/server/internal/seed"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"golang.org/x/term"
)

const usage = `usage: biodays [command]

commands:
  serve          migrate schema, auto-seed if BIODAYS_DATA_DIR is set, serve the HTTP API (default)
  seed [--strict] import stories_*.json; --strict rejects skipped/failed/empty input
  migrate        create/update the database schema and exit
  hash-password  read a password from the terminal or stdin and print a bcrypt hash
  cleanup        delete expired sessions/tokens, scrub closed feedback, prune mail payloads
  backup         write consistent SQLite snapshots and keep the last 7 days
  restore        replace a database from a backup (requires --force when target exists)
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
	case "hash-password":
		hashPassword()
	case "cleanup":
		runCleanup()
	case "backup":
		runBackup(args[1:])
	case "restore":
		runRestore(args[1:])
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

	deps, stopFeedback := startFeedback(cfg, logger)
	defer stopFeedback()

	engine, err := router.NewWithDeps(cfg, st, logger, time.Now, deps)
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

	logger.Info("biodays api listening", "addr", server.Addr, "db", cfg.DBPath, "feedback", cfg.FeedbackEnabled)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
	<-shutdownDone
}

// startFeedback opens feedback.db, starts the mail worker and returns a stop
// function that drains the worker before the process exits.
func startFeedback(cfg config.Config, logger *slog.Logger) (router.Deps, func()) {
	if !cfg.FeedbackEnabled {
		if cfg.SMTPEnabled {
			logger.Warn("SMTP is enabled but feedback is disabled; mail worker not started")
		}
		return router.Deps{}, func() {}
	}

	feedbackStore, err := store.OpenFeedback(cfg.FeedbackDBPath)
	if err != nil {
		logger.Error("open feedback database", "path", cfg.FeedbackDBPath, "error", err)
		os.Exit(1)
	}
	secret := []byte(cfg.AbuseKeySecret)

	adminService := service.NewAdminService(feedbackStore, service.AdminSettings{
		Username: cfg.AdminUsername, PasswordHash: cfg.AdminPasswordHash,
		SessionTTL: cfg.AdminSessionTTL, Secret: secret,
	}, time.Now)
	if err := adminService.Startup(context.Background()); err != nil {
		logger.Error("admin session startup", "error", err)
		os.Exit(1)
	}

	feedbackService := service.NewFeedbackService(feedbackStore, service.FeedbackSettings{
		Enabled: cfg.FeedbackEnabled, SubscriptionsEnabled: cfg.SubscriptionsEnabled,
		SMTPEnabled: cfg.SMTPEnabled, SiteURL: cfg.SiteURL, NotifyTo: cfg.FeedbackNotifyTo,
		SMTPMissing:  cfg.MissingSMTPConfig(),
		SubmitHourly: cfg.SubmitHourlyLimit, SubmitDaily: cfg.SubmitDailyLimit,
		ResendHourly: cfg.ResendHourlyLimit, ResendDaily: cfg.ResendDailyLimit,
	}, secret, time.Now)

	deps := router.Deps{
		Feedback:        feedbackStore,
		FeedbackService: feedbackService,
		AdminService:    adminService,
	}

	var (
		workerCancel context.CancelFunc
		workerDone   chan struct{}
	)
	if cfg.SMTPEnabled {
		sender, err := mailer.NewSMTPSender(mailer.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, TLSMode: cfg.SMTPTLSMode,
			Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			From: cfg.SMTPFrom, Timeout: cfg.SMTPTimeout,
		})
		if err != nil {
			logger.Error("configure SMTP", "error", err)
			os.Exit(1)
		}
		worker := mailer.NewWorker(feedbackStore, sender,
			mailer.NewRenderer("biodays", cfg.SiteURL), logger, time.Now, secret, cfg.FeedbackNotifyTo)
		var ctx context.Context
		ctx, workerCancel = context.WithCancel(context.Background())
		workerDone = make(chan struct{})
		go func() {
			defer close(workerDone)
			worker.Run(ctx)
		}()
		logger.Info("mail worker started", "host", cfg.SMTPHost, "tls", cfg.SMTPTLSMode)
	} else {
		logger.Warn("SMTP is disabled; feedback is saved but mail tasks stay queued")
	}

	stop := func() {
		if workerCancel != nil {
			workerCancel()
			select {
			case <-workerDone:
			case <-time.After(10 * time.Second):
				logger.Warn("mail worker did not stop within 10s")
			}
		}
		if err := feedbackStore.Close(); err != nil {
			logger.Warn("close feedback database", "error", err)
		}
	}
	return deps, stop
}

func hashPassword() {
	reader := bufio.NewReader(os.Stdin)
	var password string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Password: ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read password:", err)
			os.Exit(1)
		}
		password = string(raw)
		fmt.Fprint(os.Stderr, "Confirm password: ")
		confirm, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read password:", err)
			os.Exit(1)
		}
		if string(confirm) != password {
			fmt.Fprintln(os.Stderr, "passwords do not match")
			os.Exit(1)
		}
	} else {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(os.Stderr, "read password:", err)
			os.Exit(1)
		}
		password = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}
	if password == "" {
		fmt.Fprintln(os.Stderr, "password is empty")
		os.Exit(1)
	}
	hash, err := service.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hash password:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
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
	fmt.Printf("imported=%d skipped=%d files=%d failed_files=%d version=%s\n", report.Imported, report.Skipped, report.Files, report.FailedFiles, report.Version)
}

func migrate() {
	cfg, logger := loadConfigAndLogger()
	st := openStore(cfg, logger)
	defer st.Close()
	if cfg.FeedbackEnabled {
		feedbackStore, err := store.OpenFeedback(cfg.FeedbackDBPath)
		if err != nil {
			logger.Error("migrate feedback database", "error", err)
			os.Exit(1)
		}
		defer feedbackStore.Close()
	}
	logger.Info("schema up to date", "db", cfg.DBPath, "feedback_db", cfg.FeedbackDBPath)
}

func toolLogger() *slog.Logger {
	level, err := config.ParseLogLevel(os.Getenv("BIODAYS_LOG_LEVEL"))
	if err != nil {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func runCleanup() {
	logger := toolLogger()
	feedbackPath := envOr("BIODAYS_FEEDBACK_DB_PATH", "./feedback.db")
	feedbackStore, err := store.OpenFeedback(feedbackPath)
	if err != nil {
		logger.Error("open feedback database", "error", err)
		os.Exit(1)
	}
	defer feedbackStore.Close()
	report, err := feedbackStore.Cleanup(context.Background(), time.Now())
	if err != nil {
		logger.Error("cleanup failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("sessions=%d confirm_tokens=%d abuse_buckets=%d sent_payloads=%d contacts=%d tokens=%d\n",
		report.Sessions, report.ConfirmTokens, report.AbuseBuckets, report.SentOutbox,
		report.ContactScrubbed, report.TokensInvalidated)
}

func runBackup(args []string) {
	flags := flag.NewFlagSet("backup", flag.ExitOnError)
	dest := flags.String("dest", "./backups", "directory for SQLite snapshots")
	keepDays := flags.Int("keep", 7, "delete snapshots older than this many days")
	flags.Parse(args)
	if *keepDays < 1 {
		fmt.Fprintln(os.Stderr, "backup: --keep must be at least 1")
		os.Exit(1)
	}

	logger := toolLogger()
	dbPath := envOr("BIODAYS_DB_PATH", "./biodays.db")
	feedbackPath := envOr("BIODAYS_FEEDBACK_DB_PATH", "./feedback.db")
	if err := os.MkdirAll(*dest, 0o750); err != nil {
		logger.Error("create backup directory", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	stamp := time.Now().UTC().Format("20060102-150405")

	st, err := store.Open(dbPath)
	if err != nil {
		logger.Error("open story database", "error", err)
		os.Exit(1)
	}
	storyPath := filepath.Join(*dest, stamp+"-biodays.db")
	removeIfExists(storyPath)
	if err := st.BackupTo(ctx, storyPath); err != nil {
		st.Close()
		logger.Error("backup story database", "error", err)
		os.Exit(1)
	}
	st.Close()
	fmt.Println(storyPath)

	if _, err := os.Stat(feedbackPath); err == nil {
		feedbackStore, err := store.OpenFeedback(feedbackPath)
		if err != nil {
			logger.Error("open feedback database", "error", err)
			os.Exit(1)
		}
		feedbackPath := filepath.Join(*dest, stamp+"-feedback.db")
		removeIfExists(feedbackPath)
		if err := feedbackStore.BackupTo(ctx, feedbackPath); err != nil {
			feedbackStore.Close()
			logger.Error("backup feedback database", "error", err)
			os.Exit(1)
		}
		feedbackStore.Close()
		fmt.Println(feedbackPath)
	}

	pruneBackups(*dest, *keepDays, logger)
}

func runRestore(args []string) {
	flags := flag.NewFlagSet("restore", flag.ExitOnError)
	source := flags.String("source", "", "backup file to restore")
	target := flags.String("target", "", "database file to replace")
	kind := flags.String("kind", "story", "story or feedback")
	force := flags.Bool("force", false, "replace the target if it already exists")
	flags.Parse(args)
	if *source == "" || *target == "" {
		fmt.Fprintln(os.Stderr, "restore: --source and --target are required")
		os.Exit(1)
	}
	if *kind != "story" && *kind != "feedback" {
		fmt.Fprintln(os.Stderr, "restore: --kind must be story or feedback")
		os.Exit(1)
	}
	logger := toolLogger()
	if _, err := os.Stat(*source); err != nil {
		logger.Error("backup not found", "source", *source, "error", err)
		os.Exit(1)
	}
	if _, err := os.Stat(*target); err == nil && !*force {
		fmt.Fprintln(os.Stderr, "restore: target exists; stop the service and pass --force to replace it")
		os.Exit(1)
	}
	tmp := *target + ".restore-tmp"
	if err := copyFile(*source, tmp); err != nil {
		logger.Error("copy backup", "error", err)
		os.Exit(1)
	}
	if err := os.Rename(tmp, *target); err != nil {
		logger.Error("replace database", "error", err)
		os.Exit(1)
	}
	removeIfExists(*target + "-wal")
	removeIfExists(*target + "-shm")

	if *kind == "feedback" {
		feedbackStore, err := store.OpenFeedback(*target)
		if err != nil {
			logger.Error("open restored feedback database", "error", err)
			os.Exit(1)
		}
		defer feedbackStore.Close()
		if err := feedbackStore.RevokeAllSessions(context.Background()); err != nil {
			logger.Error("revoke restored sessions", "error", err)
			os.Exit(1)
		}
		paused, err := feedbackStore.PauseOutboxAfterRestore(context.Background(), time.Now())
		if err != nil {
			logger.Error("pause restored outbox", "error", err)
			os.Exit(1)
		}
		logger.Info("feedback restored", "target", *target, "sessions_revoked", true, "mail_tasks_paused", paused)
		return
	}
	logger.Info("story database restored", "target", *target)
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func removeIfExists(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "remove", path, ":", err)
	}
}

func pruneBackups(dir string, keepDays int, logger *slog.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Warn("scan backup directory", "error", err)
		return
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			logger.Warn("remove old backup", "name", name, "error", err)
		}
	}
}
