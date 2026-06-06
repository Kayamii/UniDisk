package store

import "time"

// User is an account holder on this UniDisk instance. Roles govern what the
// user may do; the resolved Role/Privileges are attached when loaded.
type User struct {
	ID                 int64       `json:"id"`
	Email              string      `json:"email"`
	PasswordHash       string      `json:"-"` // never serialized
	RoleID             *int64      `json:"role_id"`
	RoleName           string      `json:"role_name"`
	MustChangePassword bool        `json:"must_change_password"`
	Privileges         []Privilege `json:"privileges"`
	CreatedAt          time.Time   `json:"created_at"`
}

// Has reports whether the user's role grants the privilege.
func (u *User) Has(p Privilege) bool {
	for _, up := range u.Privileges {
		if up == p {
			return true
		}
	}
	return false
}

// Account is a connected storage provider login (e.g. one Google Drive
// account). CredentialsJSON holds provider tokens/keys, encrypted at rest;
// it is never exposed through the API.
type Account struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"-"`
	Provider        string     `json:"provider"`
	DisplayName     string     `json:"display_name"`
	CredentialsJSON string     `json:"-"`
	Status          string     `json:"status"`
	QuotaBytes      int64      `json:"quota_bytes"`
	UsedBytes       int64      `json:"used_bytes"`
	Priority        int        `json:"priority"`
	CreatedAt       time.Time  `json:"created_at"`
	LastCheckedAt   *time.Time `json:"last_checked_at,omitempty"`
}

// File is a node in the unified pool — either a folder (IsDir) or a file
// whose bytes live on AccountID under the provider's RemoteID.
type File struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"-"`
	ParentID   *int64    `json:"parent_id"`
	Name       string    `json:"name"`
	IsDir      bool      `json:"is_dir"`
	SizeBytes  int64     `json:"size_bytes"`
	MimeType   string    `json:"mime_type"`
	AccountID  *int64    `json:"account_id,omitempty"`
	RemoteID   *string   `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
