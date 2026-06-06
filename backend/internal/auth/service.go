package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/unidisk/unidisk/internal/store"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrEmailTaken is returned when creating a user with an existing email.
	ErrEmailTaken = errors.New("email already registered")
	// ErrInvalidCredentials is returned for a bad email/password pair.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrWeakInput is returned for empty/too-short email or password.
	ErrWeakInput = errors.New("email required and password must be at least 8 characters")
)

// Service handles bootstrap registration, login, and password changes.
type Service struct {
	store  *store.Store
	tokens *TokenManager
}

// NewService wires the auth service to the store and token manager.
func NewService(s *store.Store, tm *TokenManager) *Service {
	return &Service{store: s, tokens: tm}
}

// SeedAdmin creates the bootstrap administrator from configured credentials,
// but only when the instance has no users yet. It is idempotent and safe to
// call on every boot: once any user exists it does nothing. Self-registration
// does not exist — this is the only path to the first admin.
func (s *Service) SeedAdmin(ctx context.Context, email, password string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil // not configured; skip silently
	}
	if len(password) < 8 {
		return ErrWeakInput
	}
	count, err := s.store.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // already initialized
	}
	adminRole, err := s.store.RoleByName(ctx, "Admin")
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.store.CreateUser(ctx, email, string(hash), adminRole.ID, false)
	return err
}

// Login verifies credentials and returns the user (with privileges) plus a
// session token.
func (s *Service) Login(ctx context.Context, email, password string) (*store.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		// Constant-ish time even when the email doesn't exist.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinv"), []byte(password))
		return nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, "", ErrInvalidCredentials
	}
	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// ChangePassword sets a new password for a user after verifying the current
// one. It also clears the must-change-password flag.
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	if len(next) < 8 {
		return ErrWeakInput
	}
	user, err := s.store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.SetPassword(ctx, userID, string(hash))
}

// HashPassword is a helper for admin user creation / password resets.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrWeakInput
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}
