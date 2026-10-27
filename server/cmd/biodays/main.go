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
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"biodays/server/internal/config"
	"biodays/server/internal/mailer"
	"biodays/server/internal/model"
	"biodays/server/internal/router"
	"biodays/server/internal/security"
	"biodays/server/internal/seed"
	"biodays/server/internal/service"
	"biodays/server/internal/settings"
	"biodays/server/internal/store"

	"golang.org/x/term"
)

const usage = `usage: biodays [command]

commands:
  serve          migrate schema, auto-seed if BIODAYS_DATA_DIR is set, serve the HTTP API (default)
  seed [--strict] import stories_*.json; --strict rejects skipped/failed/empty input
  migrate        create/update the database schema and exit
  hash-password  read a password from the terminal or stdin and print a bcrypt hash
  admin set-password [--username NAME] [--stdin]
                 create or replace the administrator account (interactive;
                 --stdin reads one line for scripting)
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
	case "admin":
		runAdmin(args[1:])
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
	var (
		st  *store.Store
		err error
	)
	if cfg.UsingPostgres() {
		st, err = store.OpenPostgres(cfg.DBDSN)
	} else {
		st, err = store.Open(cfg.DBPath)
	}
	if err != nil {
		logger.Error("open database", "postgres", cfg.UsingPostgres(), "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	return st
}

// openFeedbackStore opens the feedback store on the configured backend.
func openFeedbackStore(cfg config.Config, logger *slog.Logger) *store.FeedbackStore {
	var (
		feedbackStore *store.FeedbackStore
		err           error
	)
	if cfg.UsingPostgres() {
		feedbackStore, err = store.OpenFeedbackPostgres(cfg.DBDSN)
	} else {
		feedbackStore, err = store.OpenFeedback(cfg.FeedbackDBPath)
	}
	if err != nil {
		logger.Error("open feedback database", "postgres", cfg.UsingPostgres(), "path", cfg.FeedbackDBPath, "error", err)
		os.Exit(1)
	}
	return feedbackStore
}

// autoSeed imports stories when BIODAYS_DATA_DIR points at a directory that
// actually contains stories_*.json files. Without a source directory, validate
// the database bundled in the image instead.
//
// Imports are incremental on every backend: while the stored dataset version
// matches the corpus fingerprint nothing happens, and when a new corpus ships
// only new or unchanged stories are upserted. Administrator edits and
// deletions survive (admin_edited flags and tombstones), so restarting with
// BIODAYS_DATA_DIR set never reverts live data. The explicit `biodays seed`
// command remains a full reset to the dataset canon.
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
		return checkStoryData(ctx, st, logger)
	}
	logger.Info("auto-seed starting", "dir", cfg.DataDir, "files", len(files))
	report, err := seed.RunIncremental(ctx, st, cfg.DataDir, logger)
	if err != nil {
		return err
	}
	if report.Unchanged {
		logger.Info("auto-seed: dataset unchanged; skipping import", "version", report.Version)
	} else {
		logger.Info("auto-seed finished",
			"imported", report.Imported,
			"preserved", report.Preserved,
			"skipped", report.Skipped,
			"failed_files", report.FailedFiles,
			"version", report.Version,
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

	logger.Info("biodays api listening",
		"addr", server.Addr, "postgres", cfg.UsingPostgres(), "db", cfg.DBPath, "feedback", cfg.FeedbackEnabled)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
	<-shutdownDone
}

// startFeedback opens feedback.db, starts the mail worker and returns a stop
// function that drains the worker before the process exits.
// abuseSecretSettingKey is the app_settings key holding the generated HMAC key.
const abuseSecretSettingKey = "abuse_secret"

// ensureAbuseSecret resolves the abuse HMAC key: an explicit environment value
// wins, otherwise a random key is generated once and persisted in the database
// so restarts and container replacements keep sessions and rate limits stable.
func ensureAbuseSecret(ctx context.Context, feedbackStore *store.FeedbackStore, cfg config.Config) ([]byte, string) {
	if len(cfg.AbuseKeySecret) >= 32 {
		return []byte(cfg.AbuseKeySecret), "environment"
	}
	stored, err := feedbackStore.GetAppSetting(ctx, abuseSecretSettingKey)
	if err != nil {
		logger := slog.Default()
		logger.Error("read abuse secret setting", "error", err)
		os.Exit(1)
	}
	if stored != "" {
		return []byte(stored), "database"
	}
	generated, err := security.RandomToken()
	if err != nil {
		slog.Default().Error("generate abuse secret", "error", err)
		os.Exit(1)
	}
	if err := feedbackStore.SetAppSetting(ctx, abuseSecretSettingKey, generated, time.Now()); err != nil {
		slog.Default().Error("persist abuse secret", "error", err)
		os.Exit(1)
	}
	return []byte(generated), "generated"
}

// ensureAdminBootstrap imports legacy environment credentials into the
// database on first boot and logs the resulting account state. Once a
// database row exists the environment values are ignored for good.
func ensureAdminBootstrap(ctx context.Context, feedbackStore *store.FeedbackStore, cfg config.Config, logger *slog.Logger) {
	existing, err := feedbackStore.GetAdminUser(ctx)
	if err != nil {
		logger.Error("read admin account", "error", err)
		os.Exit(1)
	}
	if existing != nil {
		if strings.TrimSpace(cfg.AdminUsername) != "" {
			logger.Info("admin account is initialized in the database; BIODAYS_ADMIN_* environment values are ignored",
				"username", existing.Username)
		}
		return
	}
	if strings.TrimSpace(cfg.AdminUsername) != "" && strings.TrimSpace(cfg.AdminPasswordHash) != "" {
		now := time.Now().UTC()
		if err := feedbackStore.UpsertAdminUser(ctx, &model.AdminUser{
			Username: strings.TrimSpace(cfg.AdminUsername), PasswordHash: cfg.AdminPasswordHash,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			logger.Error("import admin account from environment", "error", err)
			os.Exit(1)
		}
		logger.Info("admin account imported from environment", "username", strings.TrimSpace(cfg.AdminUsername))
		return
	}
	logger.Warn("admin account is not initialized; run 'biodays admin set-password' (docker compose exec app biodays admin set-password) to enable admin login")
}

func startFeedback(cfg config.Config, logger *slog.Logger) (router.Deps, func()) {
	if !cfg.FeedbackEnabled {
		if cfg.SMTPEnabled {
			logger.Warn("SMTP is enabled but feedback is disabled; mail worker not started")
		}
		return router.Deps{}, func() {}
	}

	feedbackStore := openFeedbackStore(cfg, logger)
	ctx := context.Background()
	secret, secretSource := ensureAbuseSecret(ctx, feedbackStore, cfg)
	logger.Info("abuse hmac key ready", "source", secretSource)
	ensureAdminBootstrap(ctx, feedbackStore, cfg, logger)

	adminService := service.NewAdminService(feedbackStore, service.AdminSettings{
		Username: cfg.AdminUsername, PasswordHash: cfg.AdminPasswordHash,
		SessionTTL: cfg.AdminSessionTTL, Secret: secret,
	}, time.Now)
	if err := adminService.Startup(context.Background()); err != nil {
		logger.Error("admin session startup", "error", err)
		os.Exit(1)
	}

	// Runtime settings manager: DB document wins, environment seeds first
	// boot. Derived clients (SMTP sender, Turnstile, spam judge) are rebuilt
	// on every save and travel with each snapshot.
	seed := cfg.SettingsSeed()
	if seed.Turnstile.Enabled && len(seed.Turnstile.Hostnames) == 0 {
		if parsed, err := url.Parse(cfg.SiteURL); err == nil && parsed.Hostname() != "" {
			seed.Turnstile.Hostnames = []string{parsed.Hostname()}
		}
	}
	var encKey []byte
	if strings.TrimSpace(cfg.SettingsEncKey) != "" {
		parsed, keyErr := settings.ParseEncryptionKey(cfg.SettingsEncKey)
		if keyErr != nil {
			logger.Error("parse settings encryption key", "error", keyErr)
			os.Exit(1)
		}
		encKey = parsed
	} else if cfg.FeedbackEnabled {
		logger.Warn("BIODAYS_SETTINGS_ENC_KEY is not set; secret settings (SMTP password, Turnstile secret, CF token) cannot be saved in the admin UI")
	}
	manager := settings.NewManager(feedbackStore, encKey, logger, time.Now)
	if err := manager.Load(context.Background(), seed, feedbackStore); err != nil {
		logger.Error("load runtime settings", "error", err)
		os.Exit(1)
	}
	source := service.NewManagerSettingsSource(manager, cfg.SiteURL)

	feedbackService := service.NewFeedbackService(feedbackStore, source, secret, time.Now)

	deps := router.Deps{
		Feedback:        feedbackStore,
		FeedbackService: feedbackService,
		AdminService:    adminService,
		SettingsManager: manager,
	}

	// The worker always runs and asks the settings snapshot for its sender:
	// SMTP disabled or misconfigured yields a paused sender and tasks stay
	// queued, matching the previous no-worker behaviour.
	worker := mailer.NewWorker(feedbackStore, manager.Sender,
		mailer.NewRenderer("biodays", cfg.SiteURL), logger, time.Now, secret)
	var (
		workerCtx    context.Context
		workerCancel context.CancelFunc
		workerDone   = make(chan struct{})
	)
	workerCtx, workerCancel = context.WithCancel(context.Background())
	go func() {
		defer close(workerDone)
		worker.Run(workerCtx)
	}()
	if manager.Snapshot().Document.SMTP.Enabled {
		logger.Info("mail worker started", "host", manager.Snapshot().Document.SMTP.Host, "tls", manager.Snapshot().Document.SMTP.TLSMode)
	} else {
		logger.Warn("SMTP is disabled; feedback is saved but mail tasks stay queued (enable it in /admin/settings)")
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
	password := readPasswordInput("Password: ", "Confirm password: ", true)
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

// readPasswordInput reads a password either interactively (hidden input with
// confirmation) or as one line from a piped stdin. With confirm disabled the
// single prompt is skipped, which keeps scripted usage terse.
func readPasswordInput(prompt, confirmPrompt string, confirm bool) string {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(os.Stderr, "read password:", err)
			os.Exit(1)
		}
		return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}
	read := func(p string) string {
		fmt.Fprint(os.Stderr, p)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read password:", err)
			os.Exit(1)
		}
		return string(raw)
	}
	password := read(prompt)
	if confirm {
		if second := read(confirmPrompt); second != password {
			fmt.Fprintln(os.Stderr, "passwords do not match")
			os.Exit(1)
		}
	}
	return password
}

// runAdmin implements `biodays admin set-password`: the supported way to
// create or rotate the administrator account. The account lives in the
// database, so no environment juggling or bcrypt quoting is required:
//
//	biodays admin set-password                     # interactive
//	echo 'new-password' | biodays admin set-password --stdin
//	docker compose exec app biodays admin set-password
func runAdmin(args []string) {
	flags := flag.NewFlagSet("admin", flag.ExitOnError)
	subcommand := "set-password"
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcommand = args[0]
		rest = args[1:]
	}
	if subcommand != "set-password" {
		fmt.Fprintf(os.Stderr, "unknown admin subcommand %q; only set-password is supported\n", subcommand)
		os.Exit(2)
	}
	username := flags.String("username", envOr("BIODAYS_ADMIN_USERNAME", "admin"), "administrator account name")
	stdin := flags.Bool("stdin", false, "read the password as one line from stdin instead of prompting")
	flags.Parse(rest)
	if strings.TrimSpace(*username) == "" {
		fmt.Fprintln(os.Stderr, "admin: --username must not be empty")
		os.Exit(1)
	}

	var password string
	if *stdin {
		password = readPasswordInput("", "", false)
	} else {
		password = readPasswordInput("New password: ", "Confirm new password: ", true)
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

	feedbackStore := openFeedbackStoreForTools()
	defer feedbackStore.Close()
	now := time.Now().UTC()
	if err := feedbackStore.UpsertAdminUser(context.Background(), &model.AdminUser{
		Username: strings.TrimSpace(*username), PasswordHash: hash, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "admin set-password:", err)
		os.Exit(1)
	}
	fmt.Printf("administrator account saved: %s\n", strings.TrimSpace(*username))
	fmt.Fprintln(os.Stderr, "existing admin sessions were revoked; log in again with the new password")
}

// openFeedbackStoreForTools opens the feedback database for CLI tools without
// requiring the full serve configuration.
func openFeedbackStoreForTools() *store.FeedbackStore {
	var (
		feedbackStore *store.FeedbackStore
		err           error
	)
	if dsn := strings.TrimSpace(os.Getenv("BIODAYS_DB_DSN")); dsn != "" {
		feedbackStore, err = store.OpenFeedbackPostgres(dsn)
	} else {
		feedbackStore, err = store.OpenFeedback(envOr("BIODAYS_FEEDBACK_DB_PATH", "./feedback.db"))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "open feedback database:", err)
		os.Exit(1)
	}
	return feedbackStore
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
		feedbackStore := openFeedbackStore(cfg, logger)
		defer feedbackStore.Close()
	}
	logger.Info("schema up to date", "postgres", cfg.UsingPostgres(), "db", cfg.DBPath, "feedback_db", cfg.FeedbackDBPath)
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
	var (
		feedbackStore *store.FeedbackStore
		err           error
	)
	if dsn := strings.TrimSpace(os.Getenv("BIODAYS_DB_DSN")); dsn != "" {
		feedbackStore, err = store.OpenFeedbackPostgres(dsn)
	} else {
		feedbackPath := envOr("BIODAYS_FEEDBACK_DB_PATH", "./feedback.db")
		feedbackStore, err = store.OpenFeedback(feedbackPath)
	}
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
	if strings.TrimSpace(os.Getenv("BIODAYS_DB_DSN")) != "" {
		fmt.Fprintln(os.Stderr, "backup: not available with BIODAYS_DB_DSN; back up the PostgreSQL database with pg_dump instead")
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
	if strings.TrimSpace(os.Getenv("BIODAYS_DB_DSN")) != "" {
		fmt.Fprintln(os.Stderr, "restore: not available with BIODAYS_DB_DSN; restore the PostgreSQL database with pg_restore instead")
		os.Exit(1)
	}
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
