package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// PresignedURL is a public, time-limited, read-only link to a single file.
type PresignedURL struct {
	ID        int64      `json:"id"`
	Token     string     `json:"token"`
	FileID    int64      `json:"file_id"`
	FileName  string     `json:"file_name"`
	CreatedBy int64      `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	Downloads int64      `json:"downloads"`
}

// CreatePresignedURL stores a new public link to a file.
func (s *Store) CreatePresignedURL(ctx context.Context, token string, fileID, createdBy int64, expiresAt *time.Time) (*PresignedURL, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO presigned_urls (token, file_id, created_by, expires_at)
		 VALUES (?, ?, ?, ?)`,
		token, fileID, createdBy, expiresAt)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.PresignedByID(ctx, id)
}

// PresignedByID fetches one link with its file name.
func (s *Store) PresignedByID(ctx context.Context, id int64) (*PresignedURL, error) {
	return s.scanPresigned(s.db.QueryRowContext(ctx, presignedSelect+` WHERE p.id = ?`, id))
}

// ResolvePresigned looks up a link by token for a public download. It returns
// the backing file and rejects expired links.
func (s *Store) ResolvePresigned(ctx context.Context, token string) (*PresignedURL, *File, error) {
	p, err := s.scanPresigned(s.db.QueryRowContext(ctx, presignedSelect+` WHERE p.token = ?`, token))
	if err != nil {
		return nil, nil, err
	}
	if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
		return nil, nil, ErrNotFound
	}
	file, err := s.FileByID(ctx, p.FileID)
	if err != nil {
		return nil, nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE presigned_urls SET downloads = downloads + 1 WHERE id = ?`, p.ID)
	return p, file, nil
}

// ListPresignedByUser returns links created by a user, newest first.
func (s *Store) ListPresignedByUser(ctx context.Context, userID int64) ([]*PresignedURL, error) {
	rows, err := s.db.QueryContext(ctx, presignedSelect+` WHERE p.created_by = ? ORDER BY p.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PresignedURL
	for rows.Next() {
		p, err := s.scanPresignedRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePresigned revokes a link created by the user.
func (s *Store) DeletePresigned(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM presigned_urls WHERE id = ? AND created_by = ?`, id, userID)
	return affected(res, err)
}

const presignedSelect = `SELECT p.id, p.token, p.file_id, COALESCE(f.name, ''),
	p.created_by, p.created_at, p.expires_at, p.downloads
	FROM presigned_urls p LEFT JOIN files f ON f.id = p.file_id`

func (s *Store) scanPresigned(row *sql.Row) (*PresignedURL, error) {
	var (
		p       PresignedURL
		expires sql.NullTime
	)
	err := row.Scan(&p.ID, &p.Token, &p.FileID, &p.FileName, &p.CreatedBy,
		&p.CreatedAt, &expires, &p.Downloads)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		p.ExpiresAt = &expires.Time
	}
	return &p, nil
}

func (s *Store) scanPresignedRows(rows *sql.Rows) (*PresignedURL, error) {
	var (
		p       PresignedURL
		expires sql.NullTime
	)
	err := rows.Scan(&p.ID, &p.Token, &p.FileID, &p.FileName, &p.CreatedBy,
		&p.CreatedAt, &expires, &p.Downloads)
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		p.ExpiresAt = &expires.Time
	}
	return &p, nil
}
