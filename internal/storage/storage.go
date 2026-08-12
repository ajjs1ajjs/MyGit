package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var Schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  full_name TEXT DEFAULT '',
  bio TEXT DEFAULT '',
  is_active INTEGER DEFAULT 1,
  is_superuser INTEGER DEFAULT 0,
  must_change_password INTEGER DEFAULT 0,
  token_version INTEGER DEFAULT 0,
  created_at TEXT, updated_at TEXT
);
CREATE TABLE IF NOT EXISTS ssh_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  title TEXT DEFAULT '',
  public_key TEXT NOT NULL,
  fingerprint TEXT,
  is_active INTEGER DEFAULT 1,
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  name TEXT DEFAULT '',
  token_hash TEXT UNIQUE NOT NULL,
  scopes TEXT DEFAULT '[]',
  last_used_at TEXT, expires_at TEXT,
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS repositories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  owner_type TEXT DEFAULT 'user',
  owner_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  path TEXT UNIQUE NOT NULL,
  description TEXT DEFAULT '',
  visibility TEXT DEFAULT 'private',
  default_branch TEXT DEFAULT 'main',
  is_archived INTEGER DEFAULT 0,
  is_fork INTEGER DEFAULT 0,
  forked_from INTEGER,
  size_kb INTEGER DEFAULT 0,
  created_at TEXT, updated_at TEXT
);
CREATE TABLE IF NOT EXISTS access (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  repository_id INTEGER NOT NULL,
  role INTEGER DEFAULT 30,
  UNIQUE(user_id, repository_id)
);
CREATE TABLE IF NOT EXISTS protected_branches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER NOT NULL,
  pattern TEXT DEFAULT 'main',
  required_approvals INTEGER DEFAULT 1,
  allow_direct_push INTEGER DEFAULT 0,
  allow_force_push INTEGER DEFAULT 0,
  allow_delete INTEGER DEFAULT 0,
  UNIQUE(repository_id, pattern)
);
CREATE TABLE IF NOT EXISTS issues (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER NOT NULL,
  author_id INTEGER NOT NULL,
  assignee_id INTEGER,
  milestone_id INTEGER,
  title TEXT NOT NULL,
  description TEXT DEFAULT '',
  state TEXT DEFAULT 'open',
  number INTEGER NOT NULL,
  created_at TEXT, updated_at TEXT,
  UNIQUE(repository_id, number)
);
CREATE TABLE IF NOT EXISTS issue_comments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  issue_id INTEGER NOT NULL,
  author_id INTEGER NOT NULL,
  body TEXT DEFAULT '',
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS labels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  color TEXT DEFAULT '#000000',
  UNIQUE(repository_id, name)
);
CREATE TABLE IF NOT EXISTS milestones (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER NOT NULL,
  title TEXT NOT NULL,
  description TEXT DEFAULT '',
  due_date TEXT, is_closed INTEGER DEFAULT 0,
  UNIQUE(repository_id, title)
);
CREATE TABLE IF NOT EXISTS issue_labels (
  issue_id INTEGER NOT NULL,
  label_id INTEGER NOT NULL,
  UNIQUE(issue_id, label_id)
);
CREATE TABLE IF NOT EXISTS merge_requests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER NOT NULL,
  author_id INTEGER NOT NULL,
  assignee_id INTEGER,
  source_branch TEXT NOT NULL,
  target_branch TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT DEFAULT '',
  state TEXT DEFAULT 'open',
  number INTEGER NOT NULL,
  merge_commit_sha TEXT,
  created_at TEXT, updated_at TEXT,
  UNIQUE(repository_id, number)
);
CREATE TABLE IF NOT EXISTS mr_comments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  merge_request_id INTEGER NOT NULL,
  author_id INTEGER NOT NULL,
  body TEXT DEFAULT '',
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS mr_reviews (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  merge_request_id INTEGER NOT NULL,
  author_id INTEGER NOT NULL,
  body TEXT DEFAULT '',
  approved INTEGER DEFAULT 0,
  UNIQUE(merge_request_id, author_id)
);
CREATE TABLE IF NOT EXISTS webhooks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER,
  url TEXT NOT NULL,
  secret TEXT DEFAULT '',
  events TEXT DEFAULT '[]',
  is_active INTEGER DEFAULT 1,
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  webhook_id INTEGER NOT NULL,
  event TEXT DEFAULT '',
  payload TEXT DEFAULT '{}',
  status TEXT DEFAULT 'pending',
  response_code INTEGER,
  response_body TEXT,
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS groups (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  path TEXT UNIQUE NOT NULL,
  description TEXT DEFAULT '',
  parent_id INTEGER,
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS group_members (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  role INTEGER DEFAULT 30,
  UNIQUE(group_id, user_id)
);
CREATE TABLE IF NOT EXISTS notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  recipient_id INTEGER NOT NULL,
  type TEXT DEFAULT 'system',
  title TEXT DEFAULT '',
  message TEXT DEFAULT '',
  link TEXT DEFAULT '',
  is_read INTEGER DEFAULT 0,
  created_at TEXT
);
CREATE TABLE IF NOT EXISTS wiki_pages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  repository_id INTEGER NOT NULL,
  author_id INTEGER NOT NULL,
  slug TEXT NOT NULL,
  title TEXT DEFAULT '',
  content TEXT DEFAULT '',
  UNIQUE(repository_id, slug)
);
`

func Open(path string) (*sql.DB, string, error) {
	if path == "" {
		path = "mygit.db"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if dir := filepath.Dir(abs); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)&_pragma=synchronous(NORMAL)", abs)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, abs, err
	}
	db.SetMaxOpenConns(10)
	if _, err := db.Exec(Schema); err != nil {
		return nil, abs, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, abs, fmt.Errorf("migrate: %w", err)
	}
	return db, abs, nil
}

// migrate applies additive schema changes to databases created by older
// versions. Each step must be idempotent.
func migrate(db *sql.DB) error {
	if err := addColumnIfMissing(db, "users", "token_version", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(db, "wiki_pages", "created_at", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func addColumnIfMissing(db *sql.DB, table, column, ddl string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl))
	return err
}

func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00")
}

type Store struct {
	DB     *sql.DB
	DBPath string
}

func NewStore(db *sql.DB, path string) *Store {
	return &Store{DB: db, DBPath: path}
}
