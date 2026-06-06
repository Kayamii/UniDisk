package store

// Privilege is a granular capability that roles grant. Handlers check these
// to authorize actions; the frontend uses them to show/hide controls.
type Privilege string

const (
	PrivFilesView      Privilege = "files.view"
	PrivFilesUpload    Privilege = "files.upload"
	PrivFilesDownload  Privilege = "files.download"
	PrivFilesDelete    Privilege = "files.delete"
	PrivProvidersManage Privilege = "providers.manage"
	PrivUsersManage    Privilege = "users.manage"
	PrivRolesManage    Privilege = "roles.manage"
	PrivSettingsManage Privilege = "settings.manage"
)

// AllPrivileges is the full set, granted to the Admin role and offered when
// building custom roles.
func AllPrivileges() []Privilege {
	return []Privilege{
		PrivFilesView, PrivFilesUpload, PrivFilesDownload, PrivFilesDelete,
		PrivProvidersManage, PrivUsersManage, PrivRolesManage, PrivSettingsManage,
	}
}

// ViewerPrivileges is the default Viewer role: read-only + download.
func ViewerPrivileges() []Privilege {
	return []Privilege{PrivFilesView, PrivFilesDownload}
}

// IsValidPrivilege reports whether p is a known privilege (used to reject
// junk when admins create custom roles).
func IsValidPrivilege(p Privilege) bool {
	for _, known := range AllPrivileges() {
		if known == p {
			return true
		}
	}
	return false
}
