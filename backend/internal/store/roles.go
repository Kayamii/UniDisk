package store

import (
	"context"
	"database/sql"
	"errors"
)

// Role is a named set of privileges.
type Role struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	IsSystem   bool        `json:"is_system"`
	Privileges []Privilege `json:"privileges"`
}

// SeedRoles ensures the built-in Admin and Viewer roles exist with their
// canonical privilege sets. Safe to call on every boot.
func (s *Store) SeedRoles(ctx context.Context) error {
	if _, err := s.ensureSystemRole(ctx, "Admin", AllPrivileges()); err != nil {
		return err
	}
	if _, err := s.ensureSystemRole(ctx, "Viewer", ViewerPrivileges()); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureSystemRole(ctx context.Context, name string, privs []Privilege) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO roles (name, is_system) VALUES (?, 1)`, name)
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	// Reset the privilege set to the canonical list (keeps built-ins correct
	// even across upgrades that add privileges).
	if err := s.setRolePrivileges(ctx, id, privs); err != nil {
		return 0, err
	}
	return id, nil
}

// RoleByName returns a role with its privileges, or ErrNotFound.
func (s *Store) RoleByName(ctx context.Context, name string) (*Role, error) {
	var r Role
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, is_system FROM roles WHERE name = ?`, name).
		Scan(&r.ID, &r.Name, &r.IsSystem)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if r.Privileges, err = s.rolePrivileges(ctx, r.ID); err != nil {
		return nil, err
	}
	return &r, nil
}

// RoleByID returns a role with its privileges, or ErrNotFound.
func (s *Store) RoleByID(ctx context.Context, id int64) (*Role, error) {
	var r Role
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, is_system FROM roles WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.IsSystem)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if r.Privileges, err = s.rolePrivileges(ctx, r.ID); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRoles returns all roles with their privileges.
func (s *Store) ListRoles(ctx context.Context) ([]*Role, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, is_system FROM roles ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.IsSystem); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Attach privileges (small N; one query each is fine).
	for _, r := range out {
		if r.Privileges, err = s.rolePrivileges(ctx, r.ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CreateRole creates a custom (non-system) role.
func (s *Store) CreateRole(ctx context.Context, name string, privs []Privilege) (*Role, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO roles (name, is_system) VALUES (?, 0)`, name)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := s.setRolePrivileges(ctx, id, privs); err != nil {
		return nil, err
	}
	return s.RoleByID(ctx, id)
}

// UpdateRolePrivileges replaces a custom role's privilege set. System roles
// are immutable and return an error.
func (s *Store) UpdateRolePrivileges(ctx context.Context, id int64, privs []Privilege) error {
	r, err := s.RoleByID(ctx, id)
	if err != nil {
		return err
	}
	if r.IsSystem {
		return errors.New("cannot modify a system role")
	}
	return s.setRolePrivileges(ctx, id, privs)
}

// DeleteRole removes a custom role. System roles and roles still assigned to
// users cannot be deleted.
func (s *Store) DeleteRole(ctx context.Context, id int64) error {
	r, err := s.RoleByID(ctx, id)
	if err != nil {
		return err
	}
	if r.IsSystem {
		return errors.New("cannot delete a system role")
	}
	var inUse int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role_id = ?`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse > 0 {
		return errors.New("role is assigned to users")
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM roles WHERE id = ?`, id)
	return err
}

func (s *Store) rolePrivileges(ctx context.Context, roleID int64) ([]Privilege, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT privilege FROM role_privileges WHERE role_id = ? ORDER BY privilege`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Privilege{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, Privilege(p))
	}
	return out, rows.Err()
}

// setRolePrivileges replaces a role's privileges in a transaction.
func (s *Store) setRolePrivileges(ctx context.Context, roleID int64, privs []Privilege) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_privileges WHERE role_id = ?`, roleID); err != nil {
		return err
	}
	for _, p := range privs {
		if !IsValidPrivilege(p) {
			continue // silently drop unknown privileges
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO role_privileges (role_id, privilege) VALUES (?, ?)`,
			roleID, string(p)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
