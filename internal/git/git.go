package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Backend struct {
	Binary string
	Root   string
}

func New(binary, root string) *Backend {
	if binary == "" {
		binary = "git"
	}
	return &Backend{Binary: binary, Root: root}
}

func (b *Backend) RepoPath(owner, name string) string {
	return filepath.Join(b.Root, owner, name+".git")
}

// cmd builds an exec.Cmd for git, adding git's own exec-path to PATH so the
// git-* sub-commands (git-upload-pack, git-receive-pack) resolve even when the
// parent process PATH is minimal (e.g. from a service or test harness).
// The returned cancel must be called (typically via defer) once the command has
// been started so the timeout's resources are released.
func (b *Backend) cmd(dir string, args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	c := exec.CommandContext(ctx, b.Binary, args...)
	c.Dir = dir
	c.Stdin = strings.NewReader("")
	if ep, err := exec.Command(b.Binary, "--exec-path").Output(); err == nil {
		execPath := strings.TrimSpace(string(ep))
		if execPath != "" {
			env := os.Environ()
			c.Env = append(env,
				"GIT_EXEC_PATH="+execPath,
				"PATH="+execPath+string(os.PathListSeparator)+os.Getenv("PATH"),
			)
		}
	}
	return c, cancel
}

func (b *Backend) Exists(owner, name string) bool {
	dir := b.RepoPath(owner, name)
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		return false
	}
	return true
}

func (b *Backend) InitBare(owner, name, defaultBranch string) error {
	dir := b.RepoPath(owner, name)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	cmd := exec.Command(b.Binary, "init", "--bare", "--initial-branch="+defaultBranch, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init: %v: %s", err, out)
	}
	return nil
}

func (b *Backend) Fork(owner, name, srcDir string) error {
	dir := b.RepoPath(owner, name)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(b.Binary, "clone", "--bare", srcDir, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %v: %s", err, out)
	}
	return nil
}

func (b *Backend) Remove(owner, name string) error {
	return os.RemoveAll(b.RepoPath(owner, name))
}

// --- smart HTTP ---

// InfoRefs advertises refs for the given service (git-upload-pack|git-receive-pack).
func (b *Backend) InfoRefs(dir, service string) ([]byte, error) {
	sub := strings.TrimPrefix(service, "git-")
	args := []string{sub, "--stateless-rpc", "--advertise-refs", dir}
	cmd, cancel := b.cmd(dir, args...)
	defer cancel()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %v: %v: %s", args, err, out)
	}
	header := fmt.Sprintf("# service=%s\n", service)
	headerPkt := pktLine(header) + "0000"
	return append([]byte(headerPkt), out...), nil
}

// RPC streams the request body to git upload-pack/receive-pack and returns stdout.
func (b *Backend) RPC(dir, service string, input io.Reader) ([]byte, error) {
	sub := strings.TrimPrefix(service, "git-")
	args := []string{sub, "--stateless-rpc", dir}
	cmd, cancel := b.cmd(dir, args...)
	defer cancel()
	cmd.Stdin = input
	return cmd.CombinedOutput()
}

func pktLine(s string) string {
	length := len(s) + 4
	return fmt.Sprintf("%04x%s", length, s)
}

// --- metadata ---

func run(binary, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %v: %s", args, err, out)
	}
	return string(out), nil
}

func (b *Backend) DefaultBranch(dir string) string {
	out, err := run(b.Binary, dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(out)
}

func (b *Backend) Branches(dir string) ([]string, error) {
	out, err := run(b.Binary, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(out), nil
}

func (b *Backend) Tags(dir string) ([]string, error) {
	out, err := run(b.Binary, dir, "for-each-ref", "--format=%(refname:short)", "refs/tags/")
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(out), nil
}

// Tree lists the tree at ref/path. Non-recursive shows top-level entries
// (files + dirs); recursive shows all descendants.
func (b *Backend) Tree(dir, ref, path string, recursive bool) ([]TreeEntry, error) {
	args := []string{"ls-tree", "-l"}
	if recursive {
		args = append(args, "-r")
	}
	target := ref
	if path != "" {
		target = ref + ":" + path
	}
	args = append(args, target)
	out, err := run(b.Binary, dir, args...)
	if err != nil {
		return nil, err
	}
	return parseTree(out), nil
}

type TreeEntry struct {
	Mode string `json:"mode"`
	Type string `json:"type"`
	OID  string `json:"oid"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

func parseTree(out string) []TreeEntry {
	var entries []TreeEntry
	for _, line := range strings.Split(out, "\n") {
		// format: <mode> SP <type> SP <object> SP <size> TAB <file>
		if line == "" {
			continue
		}
		tabIdx := strings.IndexByte(line, '\t')
		if tabIdx < 0 {
			continue
		}
		meta := strings.Fields(line[:tabIdx])
		if len(meta) < 3 {
			continue
		}
		entry := TreeEntry{Mode: meta[0], Type: meta[1], OID: meta[2], Path: line[tabIdx+1:]}
		if len(meta) >= 4 {
			fmt.Sscanf(meta[3], "%d", &entry.Size)
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		// directories first
		di, dj := entries[i].Type == "tree", entries[j].Type == "tree"
		if di != dj {
			return di
		}
		return entries[i].Path < entries[j].Path
	})
	return entries
}

// Blob returns the raw content of a file at ref:path (or at a raw sha).
func (b *Backend) Blob(dir, ref, path string) ([]byte, error) {
	target := ref
	if path != "" {
		target = ref + ":" + path
	}
	cmd, cancel := b.cmd(dir, "cat-file", "blob", target)
	defer cancel()
	return cmd.Output()
}

func (b *Backend) BlobAtSHA(dir, sha string) ([]byte, error) {
	cmd, cancel := b.cmd(dir, "cat-file", "blob", sha)
	defer cancel()
	return cmd.Output()
}

type Commit struct {
	SHA       string   `json:"sha"`
	ShortSHA  string   `json:"short_sha"`
	Message   string   `json:"message"`
	Author    string   `json:"author"`
	Email     string   `json:"email"`
	Timestamp string   `json:"timestamp"`
	Parents   []string `json:"parents"`
}

func (b *Backend) Commits(dir, ref string, limit int) ([]Commit, error) {
	if ref == "" {
		ref = "HEAD"
	}
	args := []string{"log", "--format=%H|%an|%ae|%at|%s", "--parents"}
	if limit > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", limit))
	}
	args = append(args, ref)
	out, err := run(b.Binary, dir, args...)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		// with --parents, the first field is "<sha> <parent1> <parent2>..."
		head := strings.Fields(parts[0])
		parents := []string{}
		if len(head) > 1 {
			parents = head[1:]
		}
		sha := ""
		if len(head) > 0 {
			sha = head[0]
		}
		commits = append(commits, Commit{
			SHA: sha, ShortSHA: shortSHA(sha), Author: parts[1],
			Email: parts[2], Timestamp: parts[3], Message: parts[4], Parents: parents,
		})
	}
	return commits, nil
}

func (b *Backend) CommitDetail(dir, sha string) (*Commit, error) {
	out, err := run(b.Binary, dir, "log", "-1", "--format=%H|%an|%ae|%at|%B", sha)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("commit not found")
	}
	parts := strings.SplitN(lines[0], "|", 5)
	if len(parts) < 5 {
		return nil, fmt.Errorf("bad commit format")
	}
	return &Commit{
		SHA: parts[0], ShortSHA: shortSHA(parts[0]), Author: parts[1],
		Email: parts[2], Timestamp: parts[3], Message: strings.Join(lines[1:], "\n"),
	}, nil
}

func (b *Backend) Diff(dir, base, head string) (string, error) {
	return run(b.Binary, dir, "diff", base, head)
}

func (b *Backend) Blame(dir, ref, path string) (string, error) {
	return run(b.Binary, dir, "blame", ref, "--", path)
}

func (b *Backend) CountSize(dir string) int {
	out, err := run(b.Binary, dir, "count-objects", "-vH")
	if err != nil {
		return 0
	}
	var size int
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "size-pack:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "size-pack:"))
			s = strings.TrimSuffix(s, " KiB")
			fmt.Sscanf(s, "%d", &size)
		}
	}
	return size
}

func (b *Backend) RefSHA(dir, ref string) (string, error) {
	out, err := run(b.Binary, dir, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CreateBranch creates branch name pointing at src. Both names are validated so
// a user-supplied value can never be interpreted as a git option.
func (b *Backend) CreateBranch(dir, name, src string) error {
	if !b.ValidRef(name) || !b.ValidRef(src) {
		return fmt.Errorf("invalid branch name")
	}
	cmd, cancel := b.cmd(dir, "branch", name, src)
	defer cancel()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch: %v: %s", err, out)
	}
	return nil
}

// DeleteBranch deletes branch name (validated against option injection).
func (b *Backend) DeleteBranch(dir, name string) error {
	if !b.ValidRef(name) {
		return fmt.Errorf("invalid branch name")
	}
	cmd, cancel := b.cmd(dir, "branch", "-D", name)
	defer cancel()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch: %v: %s", err, out)
	}
	return nil
}

// ValidRef returns true if ref is a safe git ref name. Guarding against branch
// names that start with "-" prevents git option injection, and check-ref-format
// rejects any name that could escape the ref namespace.
func (b *Backend) ValidRef(ref string) bool {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return false
	}
	if _, err := run(b.Binary, "", "check-ref-format", "--branch", ref); err != nil {
		return false
	}
	return true
}

// MergeMR merges sourceBranch into targetBranch of the bare repo at dir and
// returns the resulting SHA of targetBranch. It performs the merge in an
// isolated temporary clone so bare repositories don't need a permanent
// worktree. Fast-forwards when possible; otherwise creates a merge commit.
func (b *Backend) MergeMR(dir, sourceBranch, targetBranch string) (string, error) {
	if !b.ValidRef(sourceBranch) || !b.ValidRef(targetBranch) {
		return "", fmt.Errorf("invalid branch name")
	}
	tmp, err := os.MkdirTemp("", "mygit-merge-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// init a scratch clone and fetch both branches from the bare repo.
	if out, err := run(b.Binary, tmp, "init", "-q"); err != nil {
		return "", fmt.Errorf("git init: %v: %s", err, out)
	}
	if out, err := run(b.Binary, tmp, "remote", "add", "origin", dir); err != nil {
		return "", fmt.Errorf("git remote add: %v: %s", err, out)
	}
	if _, err := run(b.Binary, tmp, "fetch", "-q", "origin", sourceBranch+":"+sourceBranch, targetBranch+":"+targetBranch); err != nil {
		return "", fmt.Errorf("git fetch: %w", err)
	}
	if out, err := run(b.Binary, tmp, "checkout", "-q", targetBranch); err != nil {
		return "", fmt.Errorf("git checkout: %v: %s", err, out)
	}
	// merge with a non-fast-forward merge commit, aborting on conflicts.
	if out, err := run(b.Binary, tmp, "merge", "--no-ff", "-m",
		fmt.Sprintf("Merge branch '%s' into '%s'", sourceBranch, targetBranch), sourceBranch); err != nil {
		return "", fmt.Errorf("git merge: %v: %s", err, out)
	}
	if out, err := run(b.Binary, tmp, "push", "-q", "origin", "HEAD:"+targetBranch); err != nil {
		return "", fmt.Errorf("git push: %v: %s", err, out)
	}
	sha, err := b.RefSHA(dir, targetBranch)
	if err != nil {
		return "", err
	}
	return sha, nil
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
