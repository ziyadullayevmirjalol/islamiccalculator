package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

// ErrEmailTaken is returned by UserRepo.Create on a duplicate email.
var ErrEmailTaken = errors.New("email taken")

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type UserRepo interface {
	Create(ctx context.Context, email, passwordHash string) (User, error)
	ByEmail(ctx context.Context, email string) (User, error)
}

type RefreshTokenRepo interface {
	Store(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	// Valid returns the owner of an unrevoked, unexpired token hash.
	Valid(ctx context.Context, tokenHash string) (string, error)
	Revoke(ctx context.Context, tokenHash string) error
}

type AuthConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type Auth struct {
	users   UserRepo
	refresh RefreshTokenRepo
	cfg     AuthConfig
}

func NewAuth(users UserRepo, refresh RefreshTokenRepo, cfg AuthConfig) *Auth {
	return &Auth{users: users, refresh: refresh, cfg: cfg}
}

type AuthOutput struct {
	UserID          string
	Email           string
	AccessToken     string
	RefreshToken    string
	AccessExpiresIn int64 // seconds
}

func (s *Auth) Register(ctx context.Context, email, password string) (AuthOutput, error) {
	email = normalizeEmail(email)
	fields := map[string]string{}
	if !emailPattern.MatchString(email) {
		fields["email"] = "must_be_valid_email"
	}
	if len(password) < 8 {
		fields["password"] = "must_be_at_least_8_chars"
	} else if len(password) > 72 {
		fields["password"] = "too_long" // bcrypt input limit
	}
	if len(fields) > 0 {
		return AuthOutput{}, apperr.Validation("invalid registration", fields)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthOutput{}, apperr.Internal("hash password", err)
	}

	user, err := s.users.Create(ctx, email, string(hash))
	if errors.Is(err, ErrEmailTaken) {
		return AuthOutput{}, apperr.Conflict("email already registered")
	}
	if err != nil {
		return AuthOutput{}, apperr.Internal("create user", err)
	}
	return s.issue(ctx, user)
}

func (s *Auth) Login(ctx context.Context, email, password string) (AuthOutput, error) {
	user, err := s.users.ByEmail(ctx, normalizeEmail(email))
	if err != nil {
		// Same message whether the email or the password is wrong.
		return AuthOutput{}, apperr.Unauthorized("invalid email or password")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return AuthOutput{}, apperr.Unauthorized("invalid email or password")
	}
	return s.issue(ctx, user)
}

// Refresh rotates the refresh token: the presented token is revoked and a
// fresh pair is issued, so a stolen token dies on first legitimate use.
func (s *Auth) Refresh(ctx context.Context, refreshToken string) (AuthOutput, error) {
	hash := hashToken(refreshToken)
	userID, err := s.refresh.Valid(ctx, hash)
	if err != nil {
		return AuthOutput{}, apperr.Unauthorized("invalid or expired refresh token")
	}
	if err := s.refresh.Revoke(ctx, hash); err != nil {
		return AuthOutput{}, apperr.Internal("revoke refresh token", err)
	}
	return s.issue(ctx, User{ID: userID})
}

// VerifyAccess validates an access token and returns the user ID.
func (s *Auth) VerifyAccess(token string) (string, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !parsed.Valid {
		return "", apperr.Unauthorized("invalid or expired access token")
	}
	sub, err := parsed.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", apperr.Unauthorized("invalid access token")
	}
	return sub, nil
}

func (s *Auth) issue(ctx context.Context, user User) (AuthOutput, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   user.ID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return AuthOutput{}, apperr.Internal("sign access token", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return AuthOutput{}, apperr.Internal("generate refresh token", err)
	}
	refreshToken := hex.EncodeToString(raw)
	if err := s.refresh.Store(ctx, user.ID, hashToken(refreshToken), now.Add(s.cfg.RefreshTTL)); err != nil {
		return AuthOutput{}, apperr.Internal("store refresh token", err)
	}

	return AuthOutput{
		UserID:          user.ID,
		Email:           user.Email,
		AccessToken:     access,
		RefreshToken:    refreshToken,
		AccessExpiresIn: int64(s.cfg.AccessTTL.Seconds()),
	}, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	out := make([]byte, 0, len(email))
	for i := 0; i < len(email); i++ {
		c := email[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}
