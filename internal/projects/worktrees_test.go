package projects

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func gitFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
}

func TestScanWorktreesReportsMergedIdleSize(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main")
	root := filepath.Join(dir, "trees")
	tree := filepath.Join(root, "demo", "feature")
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, main, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(main, "readme.txt"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, main, "add", "readme.txt")
	gitFixture(t, main, "commit", "-m", "fixture")
	gitFixture(t, main, "update-ref", "refs/remotes/origin/main", "HEAD")
	if err := os.MkdirAll(filepath.Dir(tree), 0755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, main, "worktree", "add", "-b", "feature", tree)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items := ScanWorktrees(ctx, []string{root}, nil, nil)
	if len(items) != 1 || items[0].Branch != "feature" || items[0].Merged == nil || !*items[0].Merged || !items[0].Reclaimable || !items[0].SizeComplete || items[0].SizeBytes < 7 || items[0].LastCommitAt.IsZero() {
		t.Fatalf("worktree inventory: %+v", items)
	}
}
