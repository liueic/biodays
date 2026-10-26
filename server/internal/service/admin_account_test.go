package service_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/service"
	"biodays/server/internal/store"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func adminStore(t *testing.T) *store.FeedbackStore {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	st, err := store.OpenFeedback(fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	return st
}

func hashFor(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(hash)
}

func setDBAdmin(t *testing.T, st *store.FeedbackStore, username, password string) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, st.UpsertAdminUser(context.Background(), &model.AdminUser{
		Username: username, PasswordHash: hashFor(t, password), CreatedAt: now, UpdatedAt: now,
	}))
}

// TestAdminLoginWithoutAccount verifies the fresh-deployment state: no account
// anywhere, login reports ADMIN_NOT_INITIALIZED instead of accepting anything.
func TestAdminLoginWithoutAccount(t *testing.T) {
	st := adminStore(t)
	admin := service.NewAdminService(st, service.AdminSettings{
		SessionTTL: time.Hour, Secret: []byte("0123456789abcdef0123456789abcdef"),
	}, time.Now)

	_, err := admin.Login(context.Background(), "admin", "whatever-pw", "198.51.100.1")
	require.ErrorIs(t, err, service.ErrAdminNotInitialized)

	require.NoError(t, admin.Startup(context.Background()))
}

// TestAdminDatabaseAccountWinsOverEnvironment verifies precedence: once the
// database row exists, bootstrap environment credentials are ignored.
func TestAdminDatabaseAccountWinsOverEnvironment(t *testing.T) {
	st := adminStore(t)
	ctx := context.Background()
	setDBAdmin(t, st, "dbadmin", "database-password")

	admin := service.NewAdminService(st, service.AdminSettings{
		Username: "envadmin", PasswordHash: hashFor(t, "environment-password"),
		SessionTTL: time.Hour, Secret: []byte("0123456789abcdef0123456789abcdef"),
	}, time.Now)

	_, err := admin.Login(ctx, "envadmin", "environment-password", "198.51.100.2")
	require.ErrorIs(t, err, service.ErrInvalidCredentials)

	identity, err := admin.Login(ctx, "dbadmin", "database-password", "198.51.100.2")
	require.NoError(t, err)
	require.Equal(t, "dbadmin", identity.Username)

	// Authenticate reports the database username, not the bootstrap one.
	me, err := admin.Authenticate(ctx, identity.RawToken)
	require.NoError(t, err)
	require.Equal(t, "dbadmin", me.Username)
}

// TestAdminSetPasswordRevokesSessions mirrors `biodays admin set-password`:
// writing a new hash to the database must invalidate every prior session.
func TestAdminSetPasswordRevokesSessions(t *testing.T) {
	st := adminStore(t)
	ctx := context.Background()
	secret := []byte("0123456789abcdef0123456789abcdef")
	setDBAdmin(t, st, "admin", "old-password-123")

	admin := service.NewAdminService(st, service.AdminSettings{
		SessionTTL: time.Hour, Secret: secret,
	}, time.Now)
	identity, err := admin.Login(ctx, "admin", "old-password-123", "198.51.100.3")
	require.NoError(t, err)

	// Rotate the stored hash (what the CLI does).
	now := time.Now().UTC()
	newHash, err := service.HashPassword("new-password-456")
	require.NoError(t, err)
	require.NoError(t, st.UpsertAdminUser(ctx, &model.AdminUser{
		Username: "admin", PasswordHash: newHash, CreatedAt: now, UpdatedAt: now,
	}))

	_, err = admin.Authenticate(ctx, identity.RawToken)
	require.ErrorIs(t, err, service.ErrUnauthenticated, "credential version changed; session must die")

	rotated, err := admin.Login(ctx, "admin", "new-password-456", "198.51.100.3")
	require.NoError(t, err)
	_, err = admin.Authenticate(ctx, rotated.RawToken)
	require.NoError(t, err)
}
