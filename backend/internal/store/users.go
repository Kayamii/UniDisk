package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned when a queried row does not exist.
var ErrNotFound = errors.New("not found")

// CreateUser inserts a new user with the given role and returns it (with the
// role's privileges resolved). mustChange forces a password change at next
// login (used for admin-created accounts with a temp password).
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string, roleID int64, mustChange bool) (*User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, role_id, must_change_password)
		 VALUES (?, ?, ?, ?)`,
		email, passwordHash, roleID, mustChange)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.UserByID(ctx, id)
}

// UserByEmail looks up a user by email, returning ErrNotFound if absent.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(ctx, s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, role_id, must_change_password, created_at
		 FROM users WHERE email = ?`, email))
}

// UserByID looks up a user by id, returning ErrNotFound if absent.
func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return s.scanUser(ctx, s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, role_id, must_change_password, created_at
		 FROM users WHERE id = ?`, id))
}

// ListUsers returns all users (with roles resolved), newest last.
func (s *Store) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, email, password_hash, role_id, must_change_password, created_at
		 FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.RoleID,
			&u.MustChangePassword, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, u := range out {
		if err := s.attachRole(ctx, u); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CountUsers returns the total number of users. Used to decide whether the
// next registration should bootstrap the first admin.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CountAdmins returns how many users hold a role granting users.manage (i.e.
// can administer the instance). Used to prevent removing the last admin.
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT u.id)
		FROM users u
		JOIN role_privileges rp ON rp.role_id = u.role_id
		WHERE rp.privilege = ?`, string(PrivUsersManage)).Scan(&n)
	return n, err
}

// IsAdmin reports whether a user's role grants users.manage.
func (s *Store) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users u
		JOIN role_privileges rp ON rp.role_id = u.role_id
		WHERE u.id = ? AND rp.privilege = ?`, userID, string(PrivUsersManage)).Scan(&n)
	return n > 0, err
}

// SetUserRole assigns a role to a user.
func (s *Store) SetUserRole(ctx context.Context, userID, roleID int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET role_id = ? WHERE id = ?`, roleID, userID)
	return affected(res, err)
}

// SetPassword updates a user's password hash and clears the force-change flag.
func (s *Store) SetPassword(ctx context.Context, userID int64, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?`,
		passwordHash, userID)
	return affected(res, err)
}

// ResetPassword sets a new password hash and forces a change at next login
// (admin action).
func (s *Store) ResetPassword(ctx context.Context, userID int64, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = 1 WHERE id = ?`,
		passwordHash, userID)
	return affected(res, err)
}

// DeleteUser removes a user.
func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	return affected(res, err)
}

func (s *Store) scanUser(ctx context.Context, row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.RoleID,
		&u.MustChangePassword, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.attachRole(ctx, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// attachRole fills RoleName and Privileges from the user's role_id.
func (s *Store) attachRole(ctx context.Context, u *User) error {
	u.Privileges = []Privilege{}
	if u.RoleID == nil {
		return nil
	}
	role, err := s.RoleByID(ctx, *u.RoleID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	u.RoleName = role.Name
	u.Privileges = role.Privileges
	return nil
}
