package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
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
	return &Config{
		Port:              8060,
		Host:              "0.0.0.0",
		RepoRoot:          envOr("MYGIT_REPOS_ROOT", filepath.Join(base, "repos")),
		DBPath:            envOr("MYGIT_DB_PATH", filepath.Join(base, "mygit.db")),
		JWTSecret:         os.Getenv("MYGIT_JWT_SECRET"),
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

// EnsureJWTSecret returns the configured secret or generates one, persisting it
// next to the DB (not CWD) so it survives systemd.
func (c *Config) EnsureJWTSecret() (string, error) {
	if c.JWTSecret != "" {
		return c.JWTSecret, nil
	}
	secretFile := filepath.Join(filepath.Dir(c.DBPath), ".mygit_jwt_secret")
	b, err := os.ReadFile(secretFile)
	if err == nil && len(b) >= 32 {
		c.JWTSecret = string(b)
		return c.JWTSecret, nil
	}
	secret, err := randomToken(32)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(c.DBPath), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(secretFile, []byte(secret), 0o600); err != nil {
		return "", fmt.Errorf("write jwt secret: %w", err)
	}
	c.JWTSecret = secret
	return c.JWTSecret, nil
}

// EnsureInternalToken returns the internal machine token (for git hooks/CI).
// The auto-generated token is only echoed to stderr on an interactive terminal;
// service/logged deployments must set MYGIT_INTERNAL_API_TOKEN explicitly to
// avoid leaking the secret into logs.
func (c *Config) EnsureInternalToken() (string, error) {
	if c.InternalToken != "" {
		return c.InternalToken, nil
	}
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	c.InternalToken = token
	if isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintf(os.Stderr, "MYGIT_INTERNAL_API_TOKEN=%s\n", token)
	}
	return c.InternalToken, nil
}

func (c *Config) RepoPath(owner, name string) string {
	return filepath.Join(c.RepoRoot, owner, name+".git")
}
