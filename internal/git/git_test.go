package git

import (
	"strings"
	"testing"
)

func TestParseDiff(t *testing.T) {
	raw := `diff --git a/README.md b/README.md
index 1111111..2222222 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,4 @@
 hello
+world
diff --git a/removed.txt b/removed.txt
deleted file mode 100644
index 3333333..0000000 100644
--- a/removed.txt
+++ /dev/null
@@ -1 +0,0 @@
-gone
diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..4444444 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+new
`
	diffs := parseDiff(raw)
	if len(diffs) != 3 {
		t.Fatalf("want 3 diffs, got %d", len(diffs))
	}
	if diffs[0].Type != "M" || diffs[0].NewPath != "README.md" || diffs[0].OldPath != "README.md" {
		t.Fatalf("diff[0] = %+v", diffs[0])
	}
	if !strings.Contains(diffs[0].Diff, "+world") {
		t.Fatalf("diff[0].Diff missing hunk body: %q", diffs[0].Diff)
	}
	if diffs[1].Type != "D" || diffs[1].OldPath != "removed.txt" {
		t.Fatalf("diff[1] = %+v", diffs[1])
	}
	if diffs[2].Type != "A" || diffs[2].NewPath != "new.txt" {
		t.Fatalf("diff[2] = %+v", diffs[2])
	}
}

func TestParseBlame(t *testing.T) {
	out := `abc1234567890000000000000000000000000001 1 1
author Alice
author-mail <alice@example.com>
author-time 1750000000
author-tz +0000
committer Alice
committer-mail <alice@example.com>
committer-time 1750000000
committer-tz +0000
summary add file
filename README.md
	hello world
abc1234567890000000000000000000000000001 1 2
author Alice
author-mail <alice@example.com>
author-time 1750000000
author-tz +0000
committer Alice
committer-mail <alice@example.com>
committer-time 1750000000
committer-tz +0000
summary add file
filename README.md
	second line
`
	lines := parseBlame(out)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	if lines[0].Author != "Alice" || lines[0].AuthorMail != "alice@example.com" {
		t.Fatalf("line[0] = %+v", lines[0])
	}
	if lines[0].Line != "hello world" {
		t.Fatalf("line[0].Line = %q", lines[0].Line)
	}
	if lines[0].ShortSHA == "" || lines[1].LineNumber != 2 {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[0].Committed == "" {
		t.Fatalf("committed_at empty: %+v", lines[0])
	}
}
