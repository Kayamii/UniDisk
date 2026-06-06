package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const fileCols = `id, user_id, parent_id, name, is_dir, size_bytes, mime_type,
	account_id, remote_id, created_at, updated_at`

// CreateFile inserts a file or folder node and returns it. UserID records the
// creator; the node belongs to the shared pool.
func (s *Store) CreateFile(ctx context.Context, f *File) (*File, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO files
			(user_id, parent_id, name, is_dir, size_bytes, mime_type, account_id, remote_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.UserID, f.ParentID, f.Name, f.IsDir, f.SizeBytes, f.MimeType, f.AccountID, f.RemoteID)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.FileByID(ctx, id)
}

// FileByID fetches one node from the shared pool.
func (s *Store) FileByID(ctx context.Context, id int64) (*File, error) {
	return s.scanFile(s.db.QueryRowContext(ctx,
		`SELECT `+fileCols+` FROM files WHERE id = ?`, id))
}

// ListChildren returns the direct children of parentID (NULL parent = root),
// folders first then files, alphabetically.
func (s *Store) ListChildren(ctx context.Context, parentID *int64) ([]*File, error) {
	var (
		rows *sql.Rows
		err  error
	)
	q := `SELECT ` + fileCols + ` FROM files WHERE parent_id `
	order := ` ORDER BY is_dir DESC, name COLLATE NOCASE ASC`
	if parentID == nil {
		rows, err = s.db.QueryContext(ctx, q+`IS NULL`+order)
	} else {
		rows, err = s.db.QueryContext(ctx, q+`= ?`+order, *parentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*File
	for rows.Next() {
		f, err := s.scanFileRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SearchFiles returns up to limit non-folder nodes whose name matches the
// query (case-insensitive substring), across the whole pool, newest first.
func (s *Store) SearchFiles(ctx context.Context, query string, limit int) ([]*File, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+fileCols+` FROM files
		 WHERE is_dir = 0 AND name LIKE ? ESCAPE '\'
		 ORDER BY updated_at DESC LIMIT ?`,
		"%"+escapeLike(query)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*File
	for rows.Next() {
		f, err := s.scanFileRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// escapeLike escapes LIKE wildcards so user input is treated literally.
func escapeLike(s string) string {
	r := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '%', '_':
			r = append(r, '\\')
		}
		r = append(r, s[i])
	}
	return string(r)
}

// RenameFile updates a node's name.
func (s *Store) RenameFile(ctx context.Context, id int64, name string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE files SET name = ?, updated_at = ? WHERE id = ?`,
		name, time.Now(), id)
	return affected(res, err)
}

// DeleteFile removes a node (cascades to children for folders).
func (s *Store) DeleteFile(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id)
	return affected(res, err)
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanFile(row *sql.Row) (*File, error) {
	var f File
	err := row.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.IsDir, &f.SizeBytes,
		&f.MimeType, &f.AccountID, &f.RemoteID, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) scanFileRows(rows *sql.Rows) (*File, error) {
	var f File
	err := rows.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.IsDir, &f.SizeBytes,
		&f.MimeType, &f.AccountID, &f.RemoteID, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
