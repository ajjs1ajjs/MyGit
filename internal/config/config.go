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
	BaseDir           string
}

func Default() *Config {
	base := os.Getenv("MYGIT_BASE_DIR")
	if base == "" {
		base, _ = os.Getwd()
	}
	return &Config{
		Port:              8080,
		Host:              "0.0.0.0",
		RepoRoot:          envOr("MYGIT_REPOS_ROOT", filepath.Join(base, "repos")),
		DBPath:            envOr("MYGIT_DB_PATH", filepath.Join(base, "mygit.db")),
		JWTSecret:         os.Getenv("MYGIT_JWT_SECRET"),
		JWTExpireMin:      15,
		RefreshExpireDays: 30,
		InternalToken:     os.Getenv("MYGIT_INTERNAL_API_TOKEN"),
		GitBinary:         envOr("MYGIT_GIT_BINARY", "git"),
		BaseDir:           base,
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
func (c *Config) EnsureInternalToken() (string, error) {
	if c.InternalToken != "" {
		return c.InternalToken, nil
	}
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	c.InternalToken = token
	fmt.Fprintf(os.Stderr, "MYGIT_INTERNAL_API_TOKEN=%s\n", token)
	return c.InternalToken, nil
}

func (c *Config) RepoPath(owner, name string) string {
	return filepath.Join(c.RepoRoot, owner, name+".git")
}
