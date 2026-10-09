package projects

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeStat(pid int, cpu, start int) string {
	fields := make([]string, 22)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[11] = fmt.Sprint(cpu)
	fields[19] = fmt.Sprint(start)
	fields[21] = "100"
	return fmt.Sprintf("%d (node) %s", pid, strings.Join(fields, " "))
}
func TestDeletedOrcaWorktreeAssignment(t *testing.T) {
	root := "/srv/orca/workspaces"
	for _, tc := range []struct{ cwd, project, worktree string }{
		{"/srv/orca/workspaces/site/feature (deleted)", "site", "deleted worktree (feature)"},
		{"/srv/orca/workspaces/site/.orca-worktree-trash/feature/src", "site", "deleted worktree (feature)"},
	} {
		project, worktree := assignment(tc.cwd, nil, []string{root})
		if project != tc.project || worktree != tc.worktree {
			t.Fatalf("%q: got %q, %q", tc.cwd, project, worktree)
		}
	}
}
func TestProjectProcessAttributionAndIdle(t *testing.T) {
	dir := t.TempDir()
	proc := filepath.Join(dir, "proc")
	root := filepath.Join(dir, "projects")
	cwd := filepath.Join(root, "site")
	for _, p := range []string{filepath.Join(proc, "123", "fd"), filepath.Join(proc, "123", "net"), filepath.Join(proc, "net"), cwd} {
		if e := os.MkdirAll(p, 0755); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.Symlink(cwd, filepath.Join(proc, "123", "cwd")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("socket:[999]", filepath.Join(proc, "123", "fd", "4")); e != nil {
		t.Fatal(e)
	}
	table := "sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode\n0: 0100007F:0BB8 00000000:0000 0A 0 0 0 0 0 999\n"
	for _, p := range []string{filepath.Join(proc, "net", "tcp"), filepath.Join(proc, "123", "net", "tcp")} {
		if e := os.WriteFile(p, []byte(table), 0644); e != nil {
			t.Fatal(e)
		}
	}
	write := func(cpu int) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(proc, "123", "stat"), []byte(fakeStat(123, cpu, 44)), 0644); e != nil {
			t.Fatal(e)
		}
	}
	write(100)
	var c Collector
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	first := c.Measure(proc, []string{root}, nil, nil, now)
	if len(first) != 1 || first[0].Kind != "dev_server" || first[0].Project != "site" || len(first[0].Ports) != 1 || first[0].Ports[0] != 3000 {
		t.Fatalf("unexpected first sample: %+v", first)
	}
	later := c.Measure(proc, []string{root}, nil, nil, now.Add(16*time.Minute))
	if len(later) != 1 || !later[0].Idle || later[0].IdleMinutes < 15 {
		t.Fatalf("expected idle server: %+v", later)
	}
	write(2000)
	busy := c.Measure(proc, []string{root}, nil, nil, now.Add(16*time.Minute+15*time.Second))
	if busy[0].Idle || busy[0].CPUPercent < 80 {
		t.Fatalf("busy sample did not clear idle state: %+v", busy)
	}
	write(2000)
	if e := os.WriteFile(filepath.Join(proc, "123", "stat"), []byte(fakeStat(123, 2000, 45)), 0644); e != nil {
		t.Fatal(e)
	}
	reused := c.Measure(proc, []string{root}, nil, nil, now.Add(32*time.Minute))
	if reused[0].Idle {
		t.Fatal("reused PID inherited idle history")
	}
}
func TestStorageScanShowsDirectoriesOnly(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "alpha", "beta", "gamma")
	if e := os.MkdirAll(nested, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(nested, "private.txt"), []byte("data"), 0644); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries := ScanIONice(ctx, root)
	if len(entries) < 3 {
		t.Fatalf("missing directory levels: %+v", entries)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Path, "private.txt") || entry.Depth > 2 || !entry.Complete {
			t.Fatalf("invalid bounded entry: %+v", entry)
		}
	}
}
