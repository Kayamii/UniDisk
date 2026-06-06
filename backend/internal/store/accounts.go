package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const accountCols = `id, user_id, provider, display_name, credentials_json,
	status, quota_bytes, used_bytes, priority, created_at, last_checked_at`

// CreateAccount persists a verified provider account and returns it. UserID
// records who connected it; the account itself is part of the shared pool.
func (s *Store) CreateAccount(ctx context.Context, a *Account) (*Account, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO accounts
			(user_id, provider, display_name, credentials_json, status, quota_bytes, used_bytes, priority)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.UserID, a.Provider, a.DisplayName, a.CredentialsJSON,
		a.Status, a.QuotaBytes, a.UsedBytes, a.Priority)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.AccountByID(ctx, id)
}

// AccountByID fetches one account from the shared pool.
func (s *Store) AccountByID(ctx context.Context, id int64) (*Account, error) {
	return s.scanAccount(s.db.QueryRowContext(ctx,
		`SELECT `+accountCols+` FROM accounts WHERE id = ?`, id))
}

// ListAccounts returns all pool accounts ordered by routing priority.
func (s *Store) ListAccounts(ctx context.Context) ([]*Account, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+accountCols+` FROM accounts ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Account
	for rows.Next() {
		a, err := s.scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAccountUsage refreshes the cached quota/used figures and check time.
func (s *Store) UpdateAccountUsage(ctx context.Context, id, quota, used int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET quota_bytes = ?, used_bytes = ?, status = 'active',
			last_checked_at = ? WHERE id = ?`,
		quota, used, time.Now(), id)
	return err
}

// DeleteAccount removes an account. Files referencing it have account_id set
// to NULL by the foreign-key constraint (the metadata row remains but is
// flagged orphaned by the missing account).
func (s *Store) DeleteAccount(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) scanAccount(row *sql.Row) (*Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.UserID, &a.Provider, &a.DisplayName, &a.CredentialsJSON,
		&a.Status, &a.QuotaBytes, &a.UsedBytes, &a.Priority, &a.CreatedAt, &a.LastCheckedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) scanAccountRows(rows *sql.Rows) (*Account, error) {
	var a Account
	err := rows.Scan(&a.ID, &a.UserID, &a.Provider, &a.DisplayName, &a.CredentialsJSON,
		&a.Status, &a.QuotaBytes, &a.UsedBytes, &a.Priority, &a.CreatedAt, &a.LastCheckedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
