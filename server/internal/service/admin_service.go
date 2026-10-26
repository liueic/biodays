package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/security"
	"biodays/server/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// bcrypt silently rejects inputs above 72 bytes; reject explicitly instead of
// truncating a user's password.
const maxPasswordBytes = 72

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrLoginRateLimited    = errors.New("too many login attempts")
	ErrAdminNotInitialized = errors.New("admin account is not initialized")
)

const (
	loginWindow   = 15 * time.Minute
	loginMaxFails = 5
	bcryptCost    = 12
)

// AdminSettings carries bootstrap values. Username and PasswordHash are only
// consulted while the database has no admin account (first boot import from
// the legacy environment variables); afterwards the database row is the
// single source of truth and `biodays admin set-password` manages it.
type AdminSettings struct {
	Username     string
	PasswordHash string
	SessionTTL   time.Duration
	Secret       []byte
}

type AdminService struct {
	store    *store.FeedbackStore
	settings AdminSettings
	now      func() time.Time
}

func NewAdminService(st *store.FeedbackStore, settings AdminSettings, now func() time.Time) *AdminService {
	if now == nil {
		now = time.Now
	}
	if settings.SessionTTL <= 0 {
		settings.SessionTTL = 8 * time.Hour
	}
	return &AdminService{store: st, settings: settings, now: now}
}

type AdminIdentity struct {
	TokenHash string
	CSRF      string
	ExpiresAt time.Time
	// Username identifies the account this session belongs to.
	Username string
	// RawToken is populated only by Login so the handler can set the cookie.
	RawToken string
}

func HashPassword(password string) (string, error) {
	if len(password) > maxPasswordBytes {
		return "", errors.New("password must be at most 72 bytes")
	}
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// adminCredentials resolves the current account: the database row wins, the
// bootstrap (environment) values only apply before one exists.
func (s *AdminService) adminCredentials(ctx context.Context) (username, passwordHash string, err error) {
	if s.store != nil {
		user, lookupErr := s.store.GetAdminUser(ctx)
		if lookupErr != nil {
			return "", "", lookupErr
		}
		if user != nil {
			return user.Username, user.PasswordHash, nil
		}
	}
	if strings.TrimSpace(s.settings.Username) != "" && s.settings.PasswordHash != "" {
		return strings.TrimSpace(s.settings.Username), s.settings.PasswordHash, nil
	}
	return "", "", ErrAdminNotInitialized
}

func (s *AdminService) credentialVersion(ctx context.Context) (string, error) {
	username, passwordHash, err := s.adminCredentials(ctx)
	if err != nil {
		return "", err
	}
	return security.CredentialVersion(username, passwordHash), nil
}

// Startup invalidates sessions created under different credentials.
func (s *AdminService) Startup(ctx context.Context) error {
	version, err := s.credentialVersion(ctx)
	if err != nil {
		if errors.Is(err, ErrAdminNotInitialized) {
			return nil // nothing to compare against; login stays disabled
		}
		return err
	}
	if err := s.store.DeleteOtherCredentialSessions(ctx, version); err != nil {
		return err
	}
	_, err = s.store.CleanupExpiredSessions(ctx, s.now().UTC())
	return err
}

func (s *AdminService) Login(ctx context.Context, username, password, clientIP string) (AdminIdentity, error) {
	now := s.now().UTC()
	windowStart := now.Truncate(loginWindow)
	ipKey := security.HMACKey(s.settings.Secret, "login:ip", clientIP)
	accountKey := security.HMACKey(s.settings.Secret, "login:account", strings.TrimSpace(username))

	for _, key := range []string{ipKey, accountKey} {
		count, err := s.store.RateLimitCount(ctx, key, windowStart)
		if err != nil {
			return AdminIdentity{}, err
		}
		if count >= loginMaxFails {
			return AdminIdentity{}, ErrLoginRateLimited
		}
	}

	storedUsername, storedHash, err := s.adminCredentials(ctx)
	if err != nil {
		return AdminIdentity{}, err
	}
	if !s.verify(username, password, storedUsername, storedHash) {
		for _, key := range []string{ipKey, accountKey} {
			if _, err := s.store.ConsumeRateLimit(ctx, key, windowStart, windowStart.Add(loginWindow), loginMaxFails); err != nil {
				return AdminIdentity{}, err
			}
		}
		return AdminIdentity{}, ErrInvalidCredentials
	}

	for _, key := range []string{ipKey, accountKey} {
		if err := s.store.ClearRateLimit(ctx, key, windowStart); err != nil {
			return AdminIdentity{}, err
		}
	}

	raw, err := security.RandomToken()
	if err != nil {
		return AdminIdentity{}, err
	}
	tokenHash := security.HashToken(raw)
	csrf := security.CSRFFor(s.settings.Secret, tokenHash)
	expires := now.Add(s.settings.SessionTTL)
	session := &model.AdminSession{
		TokenHash:         tokenHash,
		CredentialVersion: security.CredentialVersion(storedUsername, storedHash),
		CSRFHash:          security.HashToken(csrf),
		CreatedAt:         now,
		ExpiresAt:         expires,
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return AdminIdentity{}, err
	}
	_ = s.store.DeleteOtherCredentialSessions(ctx, session.CredentialVersion)
	return AdminIdentity{
		TokenHash: tokenHash, CSRF: csrf, ExpiresAt: expires,
		Username: storedUsername, RawToken: raw,
	}, nil
}

func (s *AdminService) verify(username, password, storedUsername, storedHash string) bool {
	usernameOK := security.ConstantTimeEqual(strings.TrimSpace(username), storedUsername)
	// bcrypt runs regardless of the username so response time does not reveal
	// whether the account name was correct.
	passwordOK := false
	if len(password) <= maxPasswordBytes {
		passwordOK = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	return usernameOK && passwordOK
}

func (s *AdminService) Authenticate(ctx context.Context, rawToken string) (AdminIdentity, error) {
	if rawToken == "" {
		return AdminIdentity{}, ErrUnauthenticated
	}
	tokenHash := security.HashToken(rawToken)
	session, err := s.store.GetSession(ctx, tokenHash, s.now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrFeedbackNotFound) {
			return AdminIdentity{}, ErrUnauthenticated
		}
		return AdminIdentity{}, err
	}
	username, storedHash, credErr := s.adminCredentials(ctx)
	if credErr != nil {
		_ = s.store.DeleteSession(ctx, tokenHash)
		return AdminIdentity{}, ErrUnauthenticated
	}
	if session.CredentialVersion != security.CredentialVersion(username, storedHash) {
		_ = s.store.DeleteSession(ctx, tokenHash)
		return AdminIdentity{}, ErrUnauthenticated
	}
	csrf := security.CSRFFor(s.settings.Secret, tokenHash)
	if !security.ConstantTimeEqual(session.CSRFHash, security.HashToken(csrf)) {
		_ = s.store.DeleteSession(ctx, tokenHash)
		return AdminIdentity{}, ErrUnauthenticated
	}
	return AdminIdentity{TokenHash: tokenHash, CSRF: csrf, ExpiresAt: session.ExpiresAt, Username: username}, nil
}

func (s *AdminService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, security.HashToken(rawToken))
}

func (s *AdminService) VerifyCSRF(identity AdminIdentity, provided string) bool {
	return provided != "" && security.ConstantTimeEqual(identity.CSRF, provided)
}
