package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Port              int
	Host              string
	RepoRoot          string
	DBPath            string
	JWTSecret         string
	JWTExpireMin      int
	RefreshExpireDays int
	InternalToken     string
	GitBinary         string
	TLSCert           string
	TLSKey            string
	BaseDir           string
	// BackupKey is the encryption key for backup archives (AES-256-GCM).
	// Falls back to a key derived from JWTSecret when unset.
	BackupKey string
	// BackupUploadURL is the base URL (S3-compatible PUT or plain HTTP server)
	// that backup archives are uploaded to when a schedule has upload=1.
	BackupUploadURL string
	// TrustProxy enables trusting X-Forwarded-For (rate limiting) and
	// X-Forwarded-Proto (Secure cookies, HSTS). Only enable when a trusted
	// reverse proxy always overwrites both headers.
	TrustProxy bool
	// CustomReposRoot bounds the superuser-only custom_disk_path feature:
	// custom repo directories must live inside it (default: BaseDir).
	CustomReposRoot string
}

func Default() *Config {
	base := os.Getenv("MYGIT_BASE_DIR")
	if base == "" {
		base, _ = os.Getwd()
	}
	jwtSecret := os.Getenv("MYGIT_JWT_SECRET")
	if jwtSecret == "" {
		return &Config{
			Port:              8060,
			Host:              "0.0.0.0",
			RepoRoot:          envOr("MYGIT_REPOS_ROOT", filepath.Join(base, "repos")),
			DBPath:            envOr("MYGIT_DB_PATH", filepath.Join(base, "mygit.db")),
			JWTSecret:         "",
			JWTExpireMin:      15,
			RefreshExpireDays: 30,
			InternalToken:     os.Getenv("MYGIT_INTERNAL_API_TOKEN"),
			GitBinary:         envOr("MYGIT_GIT_BINARY", "git"),
			TLSCert:           os.Getenv("MYGIT_TLS_CERT"),
			TLSKey:            os.Getenv("MYGIT_TLS_KEY"),
			BaseDir:           base,
			BackupKey:         os.Getenv("MYGIT_BACKUP_KEY"),
			BackupUploadURL:   os.Getenv("MYGIT_BACKUP_UPLOAD_URL"),
			TrustProxy:        os.Getenv("MYGIT_TRUST_PROXY") == "1" || os.Getenv("MYGIT_TRUST_PROXY") == "true",
			CustomReposRoot:   envOr("MYGIT_CUSTOM_REPOS_ROOT", base),
		}
	}
	return &Config{
		Port:              8060,
		Host:              "0.0.0.0",
		RepoRoot:          envOr("MYGIT_REPOS_ROOT", filepath.Join(base, "repos")),
		DBPath:            envOr("MYGIT_DB_PATH", filepath.Join(base, "mygit.db")),
		JWTSecret:         jwtSecret,
		JWTExpireMin:      15,
		RefreshExpireDays: 30,
		InternalToken:     os.Getenv("MYGIT_INTERNAL_API_TOKEN"),
		GitBinary:         envOr("MYGIT_GIT_BINARY", "git"),
		TLSCert:           os.Getenv("MYGIT_TLS_CERT"),
		TLSKey:            os.Getenv("MYGIT_TLS_KEY"),
		BaseDir:           base,
		BackupKey:         os.Getenv("MYGIT_BACKUP_KEY"),
		BackupUploadURL:   os.Getenv("MYGIT_BACKUP_UPLOAD_URL"),
		TrustProxy:        os.Getenv("MYGIT_TRUST_PROXY") == "1" || os.Getenv("MYGIT_TRUST_PROXY") == "true",
		CustomReposRoot:   envOr("MYGIT_CUSTOM_REPOS_ROOT", base),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// EnsureJWTSecret is no longer auto-generated. The JWT secret MUST be set
// via the MYGIT_JWT_SECRET environment variable in production. Omitting it
// will cause the server to fail to start, forcing explicit configuration.

// EnsureInternalToken returns the internal machine token (for git hooks / CI).
// The token MUST be set explicitly via the MYGIT_INTERNAL_API_TOKEN environment
// variable. Omitting it will cause the server to fail to start, forcing explicit
// configuration. The old behaviour of auto-generating and printing to stderr
// has been removed to prevent secret leakage in production logs.
func (c *Config) EnsureInternalToken() (string, error) {
	if c.InternalToken != "" {
		return c.InternalToken, nil
	}
	return "", fmt.Errorf("MYGIT_INTERNAL_API_TOKEN must be set explicitly via environment variable")
}

func (c *Config) RepoPath(owner, name string) string {
	return filepath.Join(c.RepoRoot, owner, name+".git")
}
