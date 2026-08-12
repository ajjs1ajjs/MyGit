package storage

import (
	"database/sql"
	"strings"
)

// --- users ---

type User struct {
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	Email              string `json:"email"`
	PasswordHash       string `json:"-"`
	FullName           string `json:"full_name"`
	Bio                string `json:"bio"`
	IsActive           int    `json:"is_active"`
	IsSuperuser        int    `json:"is_superuser"`
	MustChangePassword int    `json:"must_change_password"`
	TokenVersion       int    `json:"-"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

const userCols = `id, username, email, password_hash, full_name, bio, is_active, is_superuser, must_change_password, token_version, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.FullName, &u.Bio,
		&u.IsActive, &u.IsSuperuser, &u.MustChangePassword, &u.TokenVersion, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (st *Store) CreateUser(u *User) (int64, error) {
	now := Now()
	res, err := st.DB.Exec(`INSERT INTO users (username, email, password_hash, full_name, bio, is_active, is_superuser, must_change_password, token_version, created_at, updated_at)
	  VALUES (?,?,?,?,?,?,?,?,?,?,?)`, u.Username, u.Email, u.PasswordHash, u.FullName, u.Bio,
		u.IsActive, u.IsSuperuser, u.MustChangePassword, 0, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RegisterUser inserts a user and atomically decides superuser bootstrap.
// BEGIN IMMEDIATE takes the SQLite write lock up front, so two concurrent
// first registrations on an empty database cannot both observe COUNT(*)=0 and
// both become superuser.
func (st *Store) RegisterUser(u *User) (id int64, isSuperuser bool, err error) {
	if _, err = st.DB.Exec(`BEGIN IMMEDIATE`); err != nil {
		return 0, false, err
	}
	defer func() {
		// ROLLBACK is a harmless no-op after a successful COMMIT.
		_, _ = st.DB.Exec(`ROLLBACK`)
	}()
	var n int
	if err = st.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, false, err
	}
	isSuperuser = n == 0
	now := Now()
	res, err := st.DB.Exec(`INSERT INTO users (username, email, password_hash, full_name, bio, is_active, is_superuser, must_change_password, token_version, created_at, updated_at)
	  VALUES (?,?,?,?,?,?,?,?,?,?,?)`, u.Username, u.Email, u.PasswordHash, u.FullName, u.Bio,
		u.IsActive, boolToInt(isSuperuser), u.MustChangePassword, 0, now, now)
	if err != nil {
		return 0, false, err
	}
	if _, err = st.DB.Exec(`COMMIT`); err != nil {
		return 0, false, err
	}
	id, _ = res.LastInsertId()
	return id, isSuperuser, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (st *Store) GetUserByUsername(username string) (*User, error) {
	return scanUser(st.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

func (st *Store) GetUserByEmail(email string) (*User, error) {
	return scanUser(st.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE email = ?`, email))
}

func (st *Store) GetUserByID(id int64) (*User, error) {
	return scanUser(st.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (st *Store) CountUsers() (int, error) {
	var n int
	err := st.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (st *Store) ListUsers() ([]User, error) {
	rows, err := st.DB.Query(`SELECT ` + userCols + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (st *Store) UpdateUser(id int64, fields map[string]any) error {
	return genericUpdate(st.DB, "users", id, fields)
}

// --- personal access tokens ---

type Token struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	TokenHash  string `json:"-"`
	Scopes     string `json:"scopes"`
	LastUsedAt string `json:"last_used_at"`
	ExpiresAt  string `json:"expires_at"`
	CreatedAt  string `json:"created_at"`
}

func (st *Store) CreateToken(userID int64, name, hash, scopes string) (int64, error) {
	return st.CreateTokenWithExpiry(userID, name, hash, scopes, "")
}

func (st *Store) CreateTokenWithExpiry(userID int64, name, hash, scopes, expiresAt string) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO tokens (user_id, name, token_hash, scopes, expires_at, created_at) VALUES (?,?,?,?,?,?)`,
		userID, name, hash, scopes, expiresAt, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) GetTokenByHash(hash string) (*Token, error) {
	var t Token
	err := st.DB.QueryRow(`SELECT id, user_id, name, token_hash, scopes, COALESCE(last_used_at,''), COALESCE(expires_at,''), created_at
	  FROM tokens WHERE token_hash = ?`, hash).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.Scopes, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

func (st *Store) ListTokens(userID int64) ([]Token, error) {
	rows, err := st.DB.Query(`SELECT id, user_id, name, token_hash, scopes, COALESCE(last_used_at,''), COALESCE(expires_at,''), created_at FROM tokens WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.Scopes, &t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (st *Store) DeleteToken(userID, id int64) error {
	_, err := st.DB.Exec(`DELETE FROM tokens WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

func (st *Store) TouchToken(id int64) {
	_, _ = st.DB.Exec(`UPDATE tokens SET last_used_at = ? WHERE id = ?`, Now(), id)
}

// --- ssh keys ---

type SSHKey struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Title       string `json:"title"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
	IsActive    int    `json:"is_active"`
	CreatedAt   string `json:"created_at"`
}

func (st *Store) CreateSSHKey(userID int64, title, key, fingerprint string) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO ssh_keys (user_id, title, public_key, fingerprint, is_active, created_at) VALUES (?,?,?,?,1,?)`,
		userID, title, key, fingerprint, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) ListSSHKeys(userID int64) ([]SSHKey, error) {
	rows, err := st.DB.Query(`SELECT id, user_id, title, public_key, fingerprint, is_active, created_at FROM ssh_keys WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SSHKey
	for rows.Next() {
		var k SSHKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Title, &k.PublicKey, &k.Fingerprint, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (st *Store) GetSSHKeyByID(id int64) (*SSHKey, error) {
	var k SSHKey
	err := st.DB.QueryRow(`SELECT id, user_id, title, public_key, fingerprint, is_active, created_at FROM ssh_keys WHERE id = ?`, id).
		Scan(&k.ID, &k.UserID, &k.Title, &k.PublicKey, &k.Fingerprint, &k.IsActive, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &k, err
}

func (st *Store) DeleteSSHKey(userID, id int64) error {
	_, err := st.DB.Exec(`DELETE FROM ssh_keys WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// --- repositories ---

type Repository struct {
	ID            int64  `json:"id"`
	OwnerType     string `json:"owner_type"`
	OwnerID       int64  `json:"owner_id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Description   string `json:"description"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
	IsArchived    int    `json:"is_archived"`
	IsFork        int    `json:"is_fork"`
	ForkedFrom    *int64 `json:"forked_from"`
	SizeKB        int    `json:"size_kb"`
	StoragePath   string `json:"-"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

const repoCols = `id, owner_type, owner_id, name, path, description, visibility, default_branch, is_archived, is_fork, forked_from, size_kb, storage_path, created_at, updated_at`

func scanRepo(row interface{ Scan(...any) error }) (*Repository, error) {
	var r Repository
	var forked sql.NullInt64
	err := row.Scan(&r.ID, &r.OwnerType, &r.OwnerID, &r.Name, &r.Path, &r.Description,
		&r.Visibility, &r.DefaultBranch, &r.IsArchived, &r.IsFork, &forked, &r.SizeKB, &r.StoragePath, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if forked.Valid {
		v := forked.Int64
		r.ForkedFrom = &v
	}
	return &r, err
}

func (st *Store) CreateRepo(r *Repository) (int64, error) {
	now := Now()
	res, err := st.DB.Exec(`INSERT INTO repositories (owner_type, owner_id, name, path, description, visibility, default_branch, is_archived, is_fork, forked_from, size_kb, storage_path, created_at, updated_at)
	  VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.OwnerType, r.OwnerID, r.Name, r.Path, r.Description,
		r.Visibility, r.DefaultBranch, r.IsArchived, r.IsFork, r.ForkedFrom, r.SizeKB, r.StoragePath, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) GetRepoByID(id int64) (*Repository, error) {
	return scanRepo(st.DB.QueryRow(`SELECT `+repoCols+` FROM repositories WHERE id = ?`, id))
}

func (st *Store) GetRepoByPath(path string) (*Repository, error) {
	return scanRepo(st.DB.QueryRow(`SELECT `+repoCols+` FROM repositories WHERE path = ?`, path))
}

// ListAccessibleRepos returns repos the user can see (owner, superuser, explicit
// access, or public).
func (st *Store) ListAccessibleRepos(userID int64, isSuperuser bool) ([]Repository, error) {
	var query string
	var args []any
	if isSuperuser {
		query = `SELECT ` + repoCols + ` FROM repositories ORDER BY id DESC`
	} else {
		query = `SELECT DISTINCT r.` + strings.ReplaceAll(repoCols, ", ", ", r.") +
			` FROM repositories r WHERE r.visibility = 'public'
			   OR r.owner_id = ?
			   OR EXISTS (SELECT 1 FROM access a WHERE a.repository_id = r.id AND a.user_id = ? AND a.role >= 10)
			   ORDER BY r.id DESC`
		args = []any{userID, userID}
	}
	rows, err := st.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRepos(rows)
}

// SearchRepos returns accessible repos whose name or path contains q
// (case-insensitive), pushed down to SQL instead of scanning in application
// memory.
func (st *Store) SearchRepos(userID int64, isSuperuser bool, q string) ([]Repository, error) {
	like := "%" + strings.ToLower(q) + "%"
	var query string
	var args []any
	if isSuperuser {
		query = `SELECT ` + repoCols + ` FROM repositories WHERE LOWER(name) LIKE ? OR LOWER(path) LIKE ? ORDER BY id DESC`
		args = []any{like, like}
	} else {
		query = `SELECT DISTINCT r.` + strings.ReplaceAll(repoCols, ", ", ", r.") +
			` FROM repositories r WHERE (LOWER(r.name) LIKE ? OR LOWER(r.path) LIKE ?)
			   AND (r.visibility = 'public'
			     OR r.owner_id = ?
			     OR EXISTS (SELECT 1 FROM access a WHERE a.repository_id = r.id AND a.user_id = ? AND a.role >= 10))
			   ORDER BY r.id DESC`
		args = []any{like, like, userID, userID}
	}
	rows, err := st.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRepos(rows)
}

func scanRepos(rows *sql.Rows) ([]Repository, error) {
	var out []Repository
	for rows.Next() {
		var r Repository
		var forked sql.NullInt64
		if err := rows.Scan(&r.ID, &r.OwnerType, &r.OwnerID, &r.Name, &r.Path, &r.Description,
			&r.Visibility, &r.DefaultBranch, &r.IsArchived, &r.IsFork, &forked, &r.SizeKB, &r.StoragePath, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if forked.Valid {
			v := forked.Int64
			r.ForkedFrom = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (st *Store) UpdateRepo(id int64, fields map[string]any) error {
	return genericUpdate(st.DB, "repositories", id, fields)
}

func (st *Store) DeleteRepo(id int64) error {
	tx, err := st.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM access WHERE repository_id = ?`,
		`DELETE FROM protected_branches WHERE repository_id = ?`,
		`DELETE FROM issues WHERE repository_id = ?`,
		`DELETE FROM merge_requests WHERE repository_id = ?`,
		`DELETE FROM webhooks WHERE repository_id = ?`,
		`DELETE FROM wiki_pages WHERE repository_id = ?`,
		`DELETE FROM repositories WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- access / roles ---

// EffectiveRole returns the user's effective role for a repo:
// superuser=50, owner=50, explicit access, group membership for
// organization-owned repos, else 0 (guest if public read).
func (st *Store) EffectiveRole(userID, repoID int64, ownerID int64, isSuperuser bool, visibility string) int {
	if isSuperuser {
		return 50
	}
	if userID == ownerID {
		return 50
	}
	if userID == 0 {
		if visibility == "public" {
			return 10
		}
		return 0
	}
	var role int
	err := st.DB.QueryRow(`SELECT role FROM access WHERE user_id = ? AND repository_id = ?`, userID, repoID).Scan(&role)
	if err != nil {
		role = 0
	}
	if role < 10 {
		if groupRole := st.userGroupRoleForRepo(userID, repoID); groupRole > role {
			role = groupRole
		}
	}
	if role < 10 && visibility == "public" {
		return 10
	}
	return role
}

// userGroupRoleForRepo resolves a user's role on an organization-owned repo
// through their membership in the owning group.
func (st *Store) userGroupRoleForRepo(userID, repoID int64) int {
	var ownerType string
	var ownerID int64
	err := st.DB.QueryRow(`SELECT owner_type, owner_id FROM repositories WHERE id = ?`, repoID).Scan(&ownerType, &ownerID)
	if err != nil || ownerType != "organization" {
		return 0
	}
	var role int
	if err := st.DB.QueryRow(`SELECT role FROM group_members WHERE group_id = ? AND user_id = ?`, ownerID, userID).Scan(&role); err != nil {
		return 0
	}
	return role
}

func (st *Store) SetAccess(userID, repoID int64, role int) error {
	_, err := st.DB.Exec(`INSERT INTO access (user_id, repository_id, role) VALUES (?,?,?)
	  ON CONFLICT(user_id, repository_id) DO UPDATE SET role = excluded.role`, userID, repoID, role)
	return err
}

// --- generic update helper (whitelisted columns handled by callers) ---

func genericUpdate(db *sql.DB, table string, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	var sets []string
	var args []any
	for k, v := range fields {
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}
	args = append(args, id)
	_, err := db.Exec(`UPDATE `+table+` SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err
}

// --- issues ---

type Issue struct {
	ID             int64  `json:"id"`
	RepositoryID   int64  `json:"repository_id"`
	AuthorID       int64  `json:"author_id"`
	AuthorUsername string `json:"author_username"`
	AssigneeID     *int64 `json:"assignee_id"`
	MilestoneID    *int64 `json:"milestone_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	State          string `json:"state"`
	Number         int    `json:"number"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// CreateIssue inserts an issue and assigns its per-repository number atomically
// inside the INSERT (INSERT ... SELECT MAX+1), so two concurrent creations
// cannot both pick the same number and trip the UNIQUE(repository_id, number)
// constraint.
func (st *Store) CreateIssue(i *Issue) (int64, int, error) {
	now := Now()
	res, err := st.DB.Exec(`INSERT INTO issues (repository_id, author_id, assignee_id, milestone_id, title, description, state, number, created_at, updated_at)
	  SELECT ?,?,?,?,?,?,?,COALESCE(MAX(number),0)+1,?,? FROM issues WHERE repository_id = ?`,
		i.RepositoryID, i.AuthorID, i.AssigneeID, i.MilestoneID,
		i.Title, i.Description, i.State, now, now, i.RepositoryID)
	if err != nil {
		return 0, 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	var number int
	if err := st.DB.QueryRow(`SELECT number FROM issues WHERE id = ?`, id).Scan(&number); err != nil {
		return 0, 0, err
	}
	return id, number, nil
}

func scanIssue(row interface{ Scan(...any) error }) (*Issue, error) {
	var i Issue
	var assignee, milestone sql.NullInt64
	err := row.Scan(&i.ID, &i.RepositoryID, &i.AuthorID, &assignee, &milestone, &i.Title,
		&i.Description, &i.State, &i.Number, &i.CreatedAt, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if assignee.Valid {
		v := assignee.Int64
		i.AssigneeID = &v
	}
	if milestone.Valid {
		v := milestone.Int64
		i.MilestoneID = &v
	}
	return &i, err
}

func (st *Store) GetIssue(repoID int64, number int) (*Issue, error) {
	return scanIssue(st.DB.QueryRow(`SELECT id, repository_id, author_id, assignee_id, milestone_id, title, description, state, number, created_at, updated_at FROM issues WHERE repository_id = ? AND number = ?`, repoID, number))
}

func (st *Store) ListIssues(repoID int64, state string) ([]Issue, error) {
	var rows *sql.Rows
	var err error
	if state == "open" || state == "closed" {
		rows, err = st.DB.Query(`SELECT id, repository_id, author_id, assignee_id, milestone_id, title, description, state, number, created_at, updated_at FROM issues WHERE repository_id = ? AND state = ? ORDER BY number DESC`, repoID, state)
	} else {
		rows, err = st.DB.Query(`SELECT id, repository_id, author_id, assignee_id, milestone_id, title, description, state, number, created_at, updated_at FROM issues WHERE repository_id = ? ORDER BY number DESC`, repoID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		i, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

func (st *Store) UpdateIssue(repoID int64, number int, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	var sets []string
	var args []any
	for k, v := range fields {
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}
	args = append(args, repoID, number)
	_, err := st.DB.Exec(`UPDATE issues SET `+strings.Join(sets, ", ")+` WHERE repository_id = ? AND number = ?`, args...)
	return err
}

func (st *Store) AddIssueComment(issueID, authorID int64, body string) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO issue_comments (issue_id, author_id, body, created_at) VALUES (?,?,?,?)`,
		issueID, authorID, body, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) ListIssueComments(issueID int64) ([]map[string]any, error) {
	return st.mapRows(`SELECT id, issue_id, author_id, body, created_at FROM issue_comments WHERE issue_id = ? ORDER BY id`, issueID)
}

// --- merge requests ---

type MergeRequest struct {
	ID             int64  `json:"id"`
	RepositoryID   int64  `json:"repository_id"`
	AuthorID       int64  `json:"author_id"`
	AuthorUsername string `json:"author_username"`
	SourceBranch   string `json:"source_branch"`
	TargetBranch   string `json:"target_branch"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	State          string `json:"state"`
	Number         int    `json:"number"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// CreateMR mirrors CreateIssue: the per-repository number is assigned
// atomically inside the INSERT to avoid MAX+1 read-modify-write races.
func (st *Store) CreateMR(m *MergeRequest) (int64, int, error) {
	now := Now()
	res, err := st.DB.Exec(`INSERT INTO merge_requests (repository_id, author_id, source_branch, target_branch, title, description, state, number, merge_commit_sha, created_at, updated_at)
	  SELECT ?,?,?,?,?,?,?,COALESCE(MAX(number),0)+1,?,?,? FROM merge_requests WHERE repository_id = ?`,
		m.RepositoryID, m.AuthorID, m.SourceBranch, m.TargetBranch,
		m.Title, m.Description, m.State, m.MergeCommitSHA, now, now, m.RepositoryID)
	if err != nil {
		return 0, 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}
	var number int
	if err := st.DB.QueryRow(`SELECT number FROM merge_requests WHERE id = ?`, id).Scan(&number); err != nil {
		return 0, 0, err
	}
	return id, number, nil
}

func scanMR(row interface{ Scan(...any) error }) (*MergeRequest, error) {
	var m MergeRequest
	err := row.Scan(&m.ID, &m.RepositoryID, &m.AuthorID, &m.SourceBranch, &m.TargetBranch,
		&m.Title, &m.Description, &m.State, &m.Number, &m.MergeCommitSHA, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (st *Store) GetMR(repoID int64, number int) (*MergeRequest, error) {
	return scanMR(st.DB.QueryRow(`SELECT id, repository_id, author_id, source_branch, target_branch, title, description, state, number, merge_commit_sha, created_at, updated_at FROM merge_requests WHERE repository_id = ? AND number = ?`, repoID, number))
}

func (st *Store) ListMRs(repoID int64) ([]MergeRequest, error) {
	rows, err := st.DB.Query(`SELECT id, repository_id, author_id, source_branch, target_branch, title, description, state, number, merge_commit_sha, created_at, updated_at FROM merge_requests WHERE repository_id = ? ORDER BY number DESC`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MergeRequest
	for rows.Next() {
		m, err := scanMR(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (st *Store) UpdateMR(repoID int64, number int, fields map[string]any) error {
	var sets []string
	var args []any
	for k, v := range fields {
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}
	args = append(args, repoID, number)
	_, err := st.DB.Exec(`UPDATE merge_requests SET `+strings.Join(sets, ", ")+` WHERE repository_id = ? AND number = ?`, args...)
	return err
}

// --- merge request comments ---

type MRComment struct {
	ID             int64  `json:"id"`
	MergeRequestID int64  `json:"merge_request_id"`
	AuthorID       int64  `json:"author_id"`
	AuthorUsername string `json:"author_username"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
}

func (st *Store) AddMRComment(mrID, authorID int64, body string) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO mr_comments (merge_request_id, author_id, body, created_at) VALUES (?,?,?,?)`,
		mrID, authorID, body, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) ListMRComments(mrID int64) ([]MRComment, error) {
	rows, err := st.DB.Query(`SELECT id, merge_request_id, author_id, body, created_at FROM mr_comments WHERE merge_request_id = ? ORDER BY id`, mrID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MRComment
	for rows.Next() {
		var c MRComment
		if err := rows.Scan(&c.ID, &c.MergeRequestID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- wiki ---

type WikiPage struct {
	ID           int64  `json:"id"`
	RepositoryID int64  `json:"repository_id"`
	AuthorID     int64  `json:"author_id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	CreatedAt    string `json:"created_at"`
}

func (st *Store) ListWiki(repoID int64) ([]WikiPage, error) {
	rows, err := st.DB.Query(`SELECT id, repository_id, author_id, slug, title, content, created_at FROM wiki_pages WHERE repository_id = ? ORDER BY slug`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WikiPage
	for rows.Next() {
		var p WikiPage
		if err := rows.Scan(&p.ID, &p.RepositoryID, &p.AuthorID, &p.Slug, &p.Title, &p.Content, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpsertWiki inserts a page or overwrites it when the slug already exists.
func (st *Store) UpsertWiki(repoID, authorID int64, slug, title, content string) error {
	now := Now()
	if title == "" {
		title = slug
	}
	_, err := st.DB.Exec(`INSERT INTO wiki_pages (repository_id, author_id, slug, title, content, created_at) VALUES (?,?,?,?,?,?)
	  ON CONFLICT(repository_id, slug) DO UPDATE SET title = excluded.title, content = excluded.content, author_id = excluded.author_id`,
		repoID, authorID, slug, title, content, now)
	return err
}

// --- webhooks ---

type Webhook struct {
	ID           int64  `json:"id"`
	RepositoryID *int64 `json:"repository_id"`
	URL          string `json:"url"`
	Secret       string `json:"secret"`
	Events       string `json:"events"`
	IsActive     int    `json:"is_active"`
	CreatedAt    string `json:"created_at"`
}

func (st *Store) CreateWebhook(w *Webhook) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO webhooks (repository_id, url, secret, events, is_active, created_at) VALUES (?,?,?,?,?,?)`,
		w.RepositoryID, w.URL, w.Secret, w.Events, w.IsActive, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) ListWebhooks(repoID int64) ([]Webhook, error) {
	rows, err := st.DB.Query(`SELECT id, repository_id, url, secret, events, is_active, created_at FROM webhooks WHERE repository_id = ? OR repository_id IS NULL ORDER BY id`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		var rid sql.NullInt64
		if err := rows.Scan(&w.ID, &rid, &w.URL, &w.Secret, &w.Events, &w.IsActive, &w.CreatedAt); err != nil {
			return nil, err
		}
		if rid.Valid {
			v := rid.Int64
			w.RepositoryID = &v
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (st *Store) DeleteWebhook(repoID int64, id int64) error {
	_, err := st.DB.Exec(`DELETE FROM webhooks WHERE id = ? AND (repository_id = ? OR repository_id IS NULL)`, id, repoID)
	return err
}

// --- notifications ---

func (st *Store) CreateNotification(recipientID int64, typ, title, message, link string) error {
	_, err := st.DB.Exec(`INSERT INTO notifications (recipient_id, type, title, message, link, is_read, created_at) VALUES (?,?,?,?,?,0,?)`,
		recipientID, typ, title, message, link, Now())
	return err
}

func (st *Store) ListNotifications(recipientID int64, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	return st.mapRows(`SELECT id, recipient_id, type, title, message, link, is_read, created_at FROM notifications WHERE recipient_id = ? ORDER BY id DESC LIMIT ?`, recipientID, limit)
}

func (st *Store) mapRows(query string, args ...any) ([]map[string]any, error) {
	rows, err := st.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				m[c] = string(v)
			case int64:
				m[c] = v
			case float64:
				m[c] = v
			case nil:
				m[c] = nil
			default:
				m[c] = v
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- groups ---

type Group struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	ParentID    *int64 `json:"parent_id"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
}

func (st *Store) CreateGroup(name, path, description string, creatorID int64) (int64, error) {
	now := Now()
	res, err := st.DB.Exec(`INSERT INTO groups (name, path, description, created_at) VALUES (?,?,?,?)`,
		name, path, description, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	_, _ = st.DB.Exec(`INSERT INTO group_members (group_id, user_id, role) VALUES (?,?,50)`, id, creatorID)
	return id, nil
}

func (st *Store) GetGroup(id int64) (*Group, error) {
	var g Group
	err := st.DB.QueryRow(`SELECT id, name, path, COALESCE(description,''), parent_id, created_at FROM groups WHERE id = ?`, id).
		Scan(&g.ID, &g.Name, &g.Path, &g.Description, &g.ParentID, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &g, err
}

func (st *Store) ListGroups(userID int64) ([]Group, error) {
	rows, err := st.DB.Query(`SELECT g.id, g.name, g.path, COALESCE(g.description,''), g.parent_id, g.created_at,
	  (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) AS member_count
	  FROM groups g ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Path, &g.Description, &g.ParentID, &g.CreatedAt, &g.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (st *Store) GroupRole(userID, groupID int64) int {
	var role int
	if err := st.DB.QueryRow(`SELECT role FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&role); err != nil {
		return 0
	}
	return role
}

func (st *Store) CountGroupMembers(groupID int64) int {
	var n int
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM group_members WHERE group_id = ?`, groupID).Scan(&n)
	return n
}

// ListGroupProjects returns the group's repositories the user can see
// (member of the group, explicit access, or public).
func (st *Store) ListGroupProjects(groupID, userID int64) ([]Repository, error) {
	var query string
	var args []any
	base := `SELECT ` + strings.ReplaceAll(repoCols, ", ", ", r.") +
		` FROM repositories r WHERE r.owner_type = 'organization' AND r.owner_id = ?`
	if userID == 0 {
		query = base + ` AND r.visibility = 'public' ORDER BY r.id DESC`
		args = []any{groupID}
	} else {
		query = base + ` AND (r.visibility = 'public' OR r.owner_id = ?
		   OR EXISTS (SELECT 1 FROM access a WHERE a.repository_id = r.id AND a.user_id = ? AND a.role >= 10)
		   OR EXISTS (SELECT 1 FROM group_members gm WHERE gm.group_id = ? AND gm.user_id = ? AND gm.role >= 10))
		   ORDER BY r.id DESC`
		args = []any{groupID, userID, userID, groupID, userID}
	}
	rows, err := st.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRepos(rows)
}

// --- integration tokens ---

type IntegrationToken struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	Provider      string `json:"provider"`
	TokenEncrypted string `json:"-"`
	CreatedAt     string `json:"created_at"`
}

func (st *Store) UpsertIntegrationToken(userID int64, provider, encrypted string) error {
	_, err := st.DB.Exec(`INSERT INTO integration_tokens (user_id, provider, token_encrypted, created_at) VALUES (?,?,?,?)
	  ON CONFLICT(user_id, provider) DO UPDATE SET token_encrypted = excluded.token_encrypted`,
		userID, provider, encrypted, Now())
	return err
}

func (st *Store) GetIntegrationToken(userID int64, provider string) (*IntegrationToken, error) {
	var t IntegrationToken
	err := st.DB.QueryRow(`SELECT id, user_id, provider, token_encrypted, COALESCE(created_at,'') FROM integration_tokens WHERE user_id = ? AND provider = ?`, userID, provider).
		Scan(&t.ID, &t.UserID, &t.Provider, &t.TokenEncrypted, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

func (st *Store) ListIntegrationTokens(userID int64) ([]IntegrationToken, error) {
	rows, err := st.DB.Query(`SELECT id, user_id, provider, token_encrypted, COALESCE(created_at,'') FROM integration_tokens WHERE user_id = ? ORDER BY provider`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IntegrationToken
	for rows.Next() {
		var t IntegrationToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Provider, &t.TokenEncrypted, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (st *Store) DeleteIntegrationToken(userID int64, provider string) error {
	_, err := st.DB.Exec(`DELETE FROM integration_tokens WHERE user_id = ? AND provider = ?`, userID, provider)
	return err
}

// --- audit log ---

func (st *Store) AddAuditEvent(action string, actorID int64, actorUsername, targetType, targetID, message string) {
	_, _ = st.DB.Exec(`INSERT INTO audit_events (action, actor_id, actor_username, target_type, target_id, message, created_at)
	  VALUES (?,?,?,?,?,?,?)`, action, actorID, actorUsername, targetType, targetID, message, Now())
}

func (st *Store) ListAuditEvents(action string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var q string
	var args []any
	if action != "" {
		q = `SELECT id, action, actor_id, actor_username, target_type, target_id, message, created_at FROM audit_events WHERE action = ? ORDER BY id DESC LIMIT ?`
		args = []any{action, limit}
	} else {
		q = `SELECT id, action, actor_id, actor_username, target_type, target_id, message, created_at FROM audit_events ORDER BY id DESC LIMIT ?`
		args = []any{limit}
	}
	return st.mapRows(q, args...)
}

// --- backup schedules / jobs ---

type BackupSchedule struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Frequency  string `json:"frequency"`
	TimeOfDay  string `json:"time_of_day"`
	Enabled    int    `json:"enabled"`
	Encrypt    int    `json:"encrypt"`
	Upload     int    `json:"upload"`
	KeepLocal  int    `json:"keep_local"`
	LastRunAt  string `json:"last_run_at"`
	CreatedAt  string `json:"created_at"`
}

func (st *Store) ListBackupSchedules() ([]BackupSchedule, error) {
	rows, err := st.DB.Query(`SELECT id, name, COALESCE(frequency,'daily'), COALESCE(time_of_day,'02:15:00'), enabled, encrypt, upload, keep_local, COALESCE(last_run_at,''), COALESCE(created_at,'') FROM backup_schedules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupSchedule
	for rows.Next() {
		var s BackupSchedule
		if err := rows.Scan(&s.ID, &s.Name, &s.Frequency, &s.TimeOfDay, &s.Enabled, &s.Encrypt, &s.Upload, &s.KeepLocal, &s.LastRunAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (st *Store) CreateBackupSchedule(s *BackupSchedule) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO backup_schedules (name, frequency, time_of_day, enabled, encrypt, upload, keep_local, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		s.Name, s.Frequency, s.TimeOfDay, s.Enabled, s.Encrypt, s.Upload, s.KeepLocal, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) GetBackupSchedule(id int64) (*BackupSchedule, error) {
	var s BackupSchedule
	err := st.DB.QueryRow(`SELECT id, name, COALESCE(frequency,'daily'), COALESCE(time_of_day,'02:15:00'), enabled, encrypt, upload, keep_local, COALESCE(last_run_at,''), COALESCE(created_at,'') FROM backup_schedules WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Frequency, &s.TimeOfDay, &s.Enabled, &s.Encrypt, &s.Upload, &s.KeepLocal, &s.LastRunAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

func (st *Store) UpdateBackupSchedule(id int64, fields map[string]any) error {
	return genericUpdate(st.DB, "backup_schedules", id, fields)
}

type BackupJob struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	ArchivePath string `json:"archive_path"`
	Error       string `json:"error"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
}

func (st *Store) CreateBackupJob(kind string) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO backup_jobs (kind, status, started_at) VALUES (?, 'running', ?)`, kind, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) FinishBackupJob(id int64, status, archivePath, errMsg string) {
	_, _ = st.DB.Exec(`UPDATE backup_jobs SET status = ?, archive_path = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, archivePath, errMsg, Now(), id)
}

func (st *Store) ListBackupJobs() ([]BackupJob, error) {
	rows, err := st.DB.Query(`SELECT id, COALESCE(kind,'scheduled'), COALESCE(status,'running'), COALESCE(archive_path,''), COALESCE(error,''), COALESCE(started_at,''), COALESCE(finished_at,'') FROM backup_jobs ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupJob
	for rows.Next() {
		var j BackupJob
		if err := rows.Scan(&j.ID, &j.Kind, &j.Status, &j.ArchivePath, &j.Error, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// --- mirror targets ---

type MirrorTarget struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Target       string `json:"target"`
	LastStatus   string `json:"last_status"`
	LastError    string `json:"last_error"`
	LastSyncedAt string `json:"last_synced_at"`
	CreatedAt    string `json:"created_at"`
}

func (st *Store) ListMirrorTargets() ([]MirrorTarget, error) {
	rows, err := st.DB.Query(`SELECT id, name, target, COALESCE(last_status,''), COALESCE(last_error,''), COALESCE(last_synced_at,''), COALESCE(created_at,'') FROM mirror_targets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MirrorTarget
	for rows.Next() {
		var m MirrorTarget
		if err := rows.Scan(&m.ID, &m.Name, &m.Target, &m.LastStatus, &m.LastError, &m.LastSyncedAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (st *Store) GetMirrorTarget(id int64) (*MirrorTarget, error) {
	var m MirrorTarget
	err := st.DB.QueryRow(`SELECT id, name, target, COALESCE(last_status,''), COALESCE(last_error,''), COALESCE(last_synced_at,''), COALESCE(created_at,'') FROM mirror_targets WHERE id = ?`, id).
		Scan(&m.ID, &m.Name, &m.Target, &m.LastStatus, &m.LastError, &m.LastSyncedAt, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (st *Store) SetMirrorResult(id int64, status, errMsg string) {
	_, _ = st.DB.Exec(`UPDATE mirror_targets SET last_status = ?, last_error = ?, last_synced_at = ? WHERE id = ?`,
		status, errMsg, Now(), id)
}

// --- import jobs ---

type ImportJob struct {
	ID         int64  `json:"id"`
	Provider   string `json:"provider"`
	TargetPath string `json:"target_path"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

func (st *Store) CreateImportJob(provider, targetPath string) (int64, error) {
	res, err := st.DB.Exec(`INSERT INTO import_jobs (provider, target_path, status, started_at) VALUES (?,?, 'running', ?)`,
		provider, targetPath, Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (st *Store) FinishImportJob(id int64, status, errMsg string) {
	_, _ = st.DB.Exec(`UPDATE import_jobs SET status = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, errMsg, Now(), id)
}

func (st *Store) ListImportJobs() ([]ImportJob, error) {
	rows, err := st.DB.Query(`SELECT id, COALESCE(provider,'custom'), COALESCE(target_path,''), COALESCE(status,'running'), COALESCE(error,''), COALESCE(started_at,''), COALESCE(finished_at,'') FROM import_jobs ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImportJob
	for rows.Next() {
		var j ImportJob
		if err := rows.Scan(&j.ID, &j.Provider, &j.TargetPath, &j.Status, &j.Error, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
