package config

import (
	"os"
)

// Config holds runtime configuration loaded from environment variables.
// Provider credentials are NOT stored here — they are entered per-account
// through the dashboard and persisted (encrypted) in the database.
type Config struct {
	// Addr is the TCP address the HTTP server listens on.
	Addr string
	// DBPath is the SQLite database file path.
	DBPath string
	// JWTSecret signs session tokens. Generated on first boot if unset.
	JWTSecret string
	// EncryptionKey encrypts provider credentials at rest. If unset, a random
	// key is generated and persisted in the data dir (stable across restarts).
	EncryptionKey string
	// DataDir holds the database and any runtime state.
	DataDir string
	// PublicURL optionally forces the externally reachable base URL used to
	// build OAuth redirect URIs and presigned links (e.g.
	// https://unidisk.example.com). If empty, the app auto-detects it from each
	// request (Host + X-Forwarded-* headers), so it works by IP or domain with
	// no configuration.
	PublicURL string
	// GoogleClientID / GoogleClientSecret are the instance-wide OAuth app
	// credentials used by the "Connect with Google" flow. Set once by the
	// operator; users then connect Drive accounts with a single click.
	GoogleClientID     string
	GoogleClientSecret string
	// DropboxAppKey / DropboxAppSecret enable the Dropbox "Connect" flow.
	DropboxAppKey    string
	DropboxAppSecret string
	// AdminEmail / AdminPassword seed the bootstrap administrator on first
	// boot (when no users exist). Self-registration is disabled, so these are
	// the only way to create the initial admin.
	AdminEmail    string
	AdminPassword string
	// OAuth app credentials for the additional "Connect"-flow providers.
	OneDriveClientID     string
	OneDriveClientSecret string
	BoxClientID          string
	BoxClientSecret      string
	PCloudClientID       string
	PCloudClientSecret   string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads configuration from the environment, applying sensible defaults
// so the app boots with zero configuration in development.
func Load() *Config {
	dataDir := getenv("UNIDISK_DATA_DIR", "/data")
	return &Config{
		Addr:      getenv("UNIDISK_ADDR", ":8080"),
		DataDir:   dataDir,
		DBPath:    getenv("UNIDISK_DB_PATH", dataDir+"/unidisk.db"),
		JWTSecret:          os.Getenv("UNIDISK_JWT_SECRET"),
		EncryptionKey:      os.Getenv("UNIDISK_ENCRYPTION_KEY"),
		// Empty by default: the app auto-detects its base URL from the request
		// (host + proxy headers), so it works by IP or DNS with no config. Set
		// this only to force a fixed public URL.
		PublicURL:          os.Getenv("UNIDISK_PUBLIC_URL"),
		GoogleClientID:     os.Getenv("UNIDISK_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("UNIDISK_GOOGLE_CLIENT_SECRET"),
		DropboxAppKey:      os.Getenv("UNIDISK_DROPBOX_APP_KEY"),
		DropboxAppSecret:   os.Getenv("UNIDISK_DROPBOX_APP_SECRET"),
		AdminEmail:         os.Getenv("UNIDISK_ADMIN_EMAIL"),
		AdminPassword:      os.Getenv("UNIDISK_ADMIN_PASSWORD"),
		OneDriveClientID:     os.Getenv("UNIDISK_ONEDRIVE_CLIENT_ID"),
		OneDriveClientSecret: os.Getenv("UNIDISK_ONEDRIVE_CLIENT_SECRET"),
		BoxClientID:          os.Getenv("UNIDISK_BOX_CLIENT_ID"),
		BoxClientSecret:      os.Getenv("UNIDISK_BOX_CLIENT_SECRET"),
		PCloudClientID:       os.Getenv("UNIDISK_PCLOUD_CLIENT_ID"),
		PCloudClientSecret:   os.Getenv("UNIDISK_PCLOUD_CLIENT_SECRET"),
	}
}
