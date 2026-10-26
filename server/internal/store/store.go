package store

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"biodays/server/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	ErrNotFound = errors.New("story not found")
	// ErrBackupUnsupported is returned when the storage backend has no
	// file-snapshot equivalent of SQLite's VACUUM INTO.
	ErrBackupUnsupported = errors.New("file backup is only available for SQLite storage; use pg_dump for PostgreSQL")
)

type Store struct {
	db *gorm.DB
}

// Open opens the story store on SQLite at path.
func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(sqliteDSN(path)), gormConfig())
	if err != nil {
		return nil, err
	}
	// SQLite serialises writes; a single connection keeps :memory: databases
	// coherent and avoids SQLITE_BUSY under the modest read load.
	if err := setPool(db, 1, 1); err != nil {
		return nil, err
	}
	return newStoryStore(db)
}

// OpenPostgres opens the story store on the PostgreSQL database named by dsn.
// Stories and feedback share that database; table names do not overlap.
func OpenPostgres(dsn string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), gormConfig())
	if err != nil {
		return nil, err
	}
	if err := setPool(db, 10, 5); err != nil {
		return nil, err
	}
	return newStoryStore(db)
}

func newStoryStore(db *gorm.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.db.AutoMigrate(&model.Story{}, &model.Tag{}, &model.Dataset{}); err != nil {
		return nil, err
	}
	return s, nil
}

func gormConfig() *gorm.Config {
	return &gorm.Config{
		Logger: gormlogger.New(log.New(io.Discard, "", 0), gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	}
}

func setPool(db *gorm.DB, maxOpen, maxIdle int) error {
	pool, err := db.DB()
	if err != nil {
		return err
	}
	pool.SetMaxOpenConns(maxOpen)
	pool.SetMaxIdleConns(maxIdle)
	return nil
}

func sqliteDSN(path string) string {
	switch {
	case path == ":memory:":
		return "file::memory:?cache=shared&_foreign_keys=on&_busy_timeout=5000"
	case strings.HasPrefix(path, "file:"):
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		return path + separator + "_foreign_keys=on&_busy_timeout=5000"
	default:
		return path + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"
	}
}

func (s *Store) Close() error {
	pool, err := s.db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	var result int
	return s.db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error
}

// BackupTo writes a consistent snapshot with VACUUM INTO, including WAL data.
func (s *Store) BackupTo(ctx context.Context, path string) error {
	if s.db.Dialector.Name() == "postgres" {
		return ErrBackupUnsupported
	}
	return s.db.WithContext(ctx).Exec("VACUUM INTO ?", path).Error
}

type ListFilter struct {
	Month    *int
	Day      *int
	Tag      string
	Query    string
	Page     int
	PageSize int
	Summary  bool
}

func (f ListFilter) apply(db *gorm.DB) *gorm.DB {
	if f.Month != nil {
		db = db.Where("stories.month = ?", *f.Month)
	}
	if f.Day != nil {
		db = db.Where("stories.day = ?", *f.Day)
	}
	if f.Tag != "" {
		db = db.Joins("JOIN story_tags ON story_tags.story_id = stories.id").
			Joins("JOIN tags ON tags.id = story_tags.tag_id AND tags.name = ?", f.Tag)
	}
	if f.Query != "" {
		pattern := "%" + escapeLike(f.Query) + "%"
		db = db.Where(
			`(stories.title LIKE ? ESCAPE '\' OR stories.summary LIKE ? ESCAPE '\' OR stories.lab_item LIKE ? ESCAPE '\')`,
			pattern, pattern, pattern,
		)
	}
	return db
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func preloadTags(db *gorm.DB) *gorm.DB {
	return db.Preload("Tags", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("tags.id ASC")
	})
}
