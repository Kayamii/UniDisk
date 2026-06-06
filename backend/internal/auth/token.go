package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenTTL is how long an issued session token stays valid.
const tokenTTL = 7 * 24 * time.Hour

// TokenManager issues and validates stateless JWT session tokens.
type TokenManager struct {
	secret []byte
}

// NewTokenManager builds a TokenManager. If secret is empty a random one is
// generated, meaning sessions reset on restart — fine for dev, but set
// UNIDISK_JWT_SECRET in production to keep users logged in across restarts.
func NewTokenManager(secret string) (*TokenManager, error) {
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate jwt secret: %w", err)
		}
		secret = hex.EncodeToString(b)
	}
	return &TokenManager{secret: []byte(secret)}, nil
}

// Issue creates a signed token carrying the user id as subject.
func (m *TokenManager) Issue(userID int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", userID),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

// IssueState mints a short-lived token (10 min) used as the OAuth `state`
// parameter. It binds the connecting user to the round-trip so the unauthen-
// ticated provider callback can attribute the new account correctly.
func (m *TokenManager) IssueState(userID int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", userID),
		Audience:  jwt.ClaimStrings{"oauth-state"},
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

// ValidateState parses an OAuth state token and returns its user id. It
// requires the oauth-state audience so a normal session token can't be
// replayed here.
func (m *TokenManager) ValidateState(tokenStr string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithAudience("oauth-state"))
	if err != nil {
		return 0, err
	}
	var id int64
	if _, err := fmt.Sscanf(claims.Subject, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid subject: %w", err)
	}
	return id, nil
}

// Validate parses a token and returns the user id it was issued for.
func (m *TokenManager) Validate(tokenStr string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return 0, err
	}
	var id int64
	if _, err := fmt.Sscanf(claims.Subject, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid subject: %w", err)
	}
	return id, nil
}
