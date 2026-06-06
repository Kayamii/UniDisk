package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// APIKey is a programmatic credential owned by a user. The plaintext is never
// stored — only KeyHash. Privileges are a subset of file privileges.
type APIKey struct {
	ID         int64       `json:"id"`
	UserID     int64       `json:"-"`
	Name       string      `json:"name"`
	KeyPrefix  string      `json:"key_prefix"`
	Privileges []Privilege `json:"privileges"`
	CreatedAt  time.Time   `json:"created_at"`
	ExpiresAt  *time.Time  `json:"expires_at"`
	LastUsedAt *time.Time  `json:"last_used_at"`
}

// CreateAPIKey stores a new key (by hash) and returns the row.
func (s *Store) CreateAPIKey(ctx context.Context, userID int64, name, keyHash, keyPrefix string, privs []Privilege, expiresAt *time.Time) (*APIKey, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (user_id, name, key_hash, key_prefix, privileges, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		userID, name, keyHash, keyPrefix, joinPrivs(privs), expiresAt)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.APIKeyByID(ctx, userID, id)
}

// APIKeyByID fetches one key scoped to its owner.
func (s *Store) APIKeyByID(ctx context.Context, userID, id int64) (*APIKey, error) {
	return s.scanAPIKey(s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, key_prefix, privileges, created_at, expires_at, last_used_at
		 FROM api_keys WHERE id = ? AND user_id = ?`, id, userID))
}

// ListAPIKeys returns a user's keys, newest first.
func (s *Store) ListAPIKeys(ctx context.Context, userID int64) ([]*APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, key_prefix, privileges, created_at, expires_at, last_used_at
		 FROM api_keys WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*APIKey
	for rows.Next() {
		k, err := s.scanAPIKeyRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// AuthByKeyHash resolves an API key by its hash, returning the key and its
// owning user. It rejects expired keys and bumps last_used_at.
func (s *Store) AuthByKeyHash(ctx context.Context, keyHash string) (*APIKey, *User, error) {
	var (
		k        APIKey
		privs    string
		expires  sql.NullTime
		lastUsed sql.NullTime
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, key_prefix, privileges, created_at, expires_at, last_used_at
		 FROM api_keys WHERE key_hash = ?`, keyHash).
		Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &privs, &k.CreatedAt, &expires, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	if expires.Valid {
		if time.Now().After(expires.Time) {
			return nil, nil, ErrNotFound // treat expired as invalid
		}
		k.ExpiresAt = &expires.Time
	}
	k.Privileges = splitPrivs(privs)

	user, err := s.UserByID(ctx, k.UserID)
	if err != nil {
		return nil, nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, time.Now(), k.ID)
	return &k, user, nil
}

// DeleteAPIKey revokes a key owned by the user.
func (s *Store) DeleteAPIKey(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE id = ? AND user_id = ?`, id, userID)
	return affected(res, err)
}

func (s *Store) scanAPIKey(row *sql.Row) (*APIKey, error) {
	var (
		k        APIKey
		privs    string
		expires  sql.NullTime
		lastUsed sql.NullTime
	)
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &privs,
		&k.CreatedAt, &expires, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	k.Privileges = splitPrivs(privs)
	if expires.Valid {
		k.ExpiresAt = &expires.Time
	}
	if lastUsed.Valid {
		k.LastUsedAt = &lastUsed.Time
	}
	return &k, nil
}

func (s *Store) scanAPIKeyRows(rows *sql.Rows) (*APIKey, error) {
	var (
		k        APIKey
		privs    string
		expires  sql.NullTime
		lastUsed sql.NullTime
	)
	err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &privs,
		&k.CreatedAt, &expires, &lastUsed)
	if err != nil {
		return nil, err
	}
	k.Privileges = splitPrivs(privs)
	if expires.Valid {
		k.ExpiresAt = &expires.Time
	}
	if lastUsed.Valid {
		k.LastUsedAt = &lastUsed.Time
	}
	return &k, nil
}

func joinPrivs(privs []Privilege) string {
	s := make([]string, len(privs))
	for i, p := range privs {
		s[i] = string(p)
	}
	return strings.Join(s, ",")
}

func splitPrivs(s string) []Privilege {
	if s == "" {
		return []Privilege{}
	}
	parts := strings.Split(s, ",")
	out := make([]Privilege, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, Privilege(p))
		}
	}
	return out
}
