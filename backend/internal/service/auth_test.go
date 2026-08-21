package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

// --- fakes -----------------------------------------------------------------

type fakeUsers struct {
	byEmail map[string]User
	next    int
}

func newFakeUsers() *fakeUsers { return &fakeUsers{byEmail: map[string]User{}} }

func (f *fakeUsers) Create(_ context.Context, email, passwordHash string) (User, error) {
	if _, exists := f.byEmail[email]; exists {
		return User{}, ErrEmailTaken
	}
	f.next++
	u := User{ID: fmt.Sprintf("user-%d", f.next), Email: email, PasswordHash: passwordHash, CreatedAt: time.Now()}
	f.byEmail[email] = u
	return u, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

type refreshRow struct {
	userID    string
	expiresAt time.Time
	revoked   bool
}

type fakeRefresh map[string]*refreshRow

func (f fakeRefresh) Store(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	f[tokenHash] = &refreshRow{userID: userID, expiresAt: expiresAt}
	return nil
}

func (f fakeRefresh) Valid(_ context.Context, tokenHash string) (string, error) {
	row, ok := f[tokenHash]
	if !ok || row.revoked || time.Now().After(row.expiresAt) {
		return "", ErrNotFound
	}
	return row.userID, nil
}

func (f fakeRefresh) Revoke(_ context.Context, tokenHash string) error {
	if row, ok := f[tokenHash]; ok {
		row.revoked = true
	}
	return nil
}

func newAuthService() (*Auth, *fakeUsers, fakeRefresh) {
	users := newFakeUsers()
	refresh := fakeRefresh{}
	svc := NewAuth(users, refresh, AuthConfig{
		Secret:     "test-secret",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	})
	return svc, users, refresh
}

// --- workflows -------------------------------------------------------------

func TestAuth_RegisterLoginRefresh(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAuthService()

	out, err := svc.Register(ctx, "User@Example.com ", "correct-horse-battery")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", out.Email, "email is normalized")
	assert.NotEmpty(t, out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, int64(900), out.AccessExpiresIn)

	userID, err := svc.VerifyAccess(out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, out.UserID, userID, "access token verifies back to the user")

	login, err := svc.Login(ctx, "user@example.com", "correct-horse-battery")
	require.NoError(t, err)
	assert.Equal(t, out.UserID, login.UserID)

	// Refresh rotates: new pair works, the used token dies.
	refreshed, err := svc.Refresh(ctx, login.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, out.UserID, refreshed.UserID)
	assert.NotEqual(t, login.RefreshToken, refreshed.RefreshToken)

	_, err = svc.Refresh(ctx, login.RefreshToken)
	require.Error(t, err, "a rotated refresh token must not work twice")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
}

func TestAuth_Failures(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAuthService()
	_, err := svc.Register(ctx, "a@b.co", "long-enough-password")
	require.NoError(t, err)

	t.Run("duplicate email conflicts", func(t *testing.T) {
		_, err := svc.Register(ctx, "a@b.co", "another-password!")
		require.Error(t, err)
		assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	})

	t.Run("wrong password is unauthorized with a neutral message", func(t *testing.T) {
		_, err := svc.Login(ctx, "a@b.co", "wrong-password!")
		require.Error(t, err)
		e := apperr.From(err)
		assert.Equal(t, apperr.CodeUnauthorized, e.Code)
		assert.Equal(t, "invalid email or password", e.Message)
	})

	t.Run("unknown email gets the same message", func(t *testing.T) {
		_, err := svc.Login(ctx, "nobody@b.co", "whatever-pass")
		assert.Equal(t, "invalid email or password", apperr.From(err).Message)
	})

	t.Run("weak password rejected", func(t *testing.T) {
		_, err := svc.Register(ctx, "c@d.co", "short")
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "password")
	})

	t.Run("bad email rejected", func(t *testing.T) {
		_, err := svc.Register(ctx, "not-an-email", "long-enough-password")
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "email")
	})

	t.Run("tampered access token rejected", func(t *testing.T) {
		out, err := svc.Login(ctx, "a@b.co", "long-enough-password")
		require.NoError(t, err)
		_, err = svc.VerifyAccess(out.AccessToken + "x")
		require.Error(t, err)
		assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
	})
}
