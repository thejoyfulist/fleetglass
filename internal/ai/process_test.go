package ai

import (
	"github.com/thejoyfulist/fleetglass/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectAndProcessTree(t *testing.T) {
	base := t.TempDir()
	orca := filepath.Join(base, "workspaces")
	cwd := filepath.Join(orca, "helmline", "tree", "src")
	os.MkdirAll(cwd, 0700)
	project, worktree := Project(cwd, orca)
	if project != "helmline" || worktree != "tree" {
		t.Fatalf("%q %q", project, worktree)
	}
	repo := filepath.Join(base, "project")
	os.MkdirAll(filepath.Join(repo, "sub"), 0700)
	os.Mkdir(filepath.Join(repo, ".git"), 0700)
	project, _ = Project(filepath.Join(repo, "sub"), "")
	if project != "project" {
		t.Fatal(project)
	}
	proc := filepath.Join(base, "proc")
	os.Mkdir(proc, 0700)
	for _, v := range []struct{ pid, ppid, ticks, rss string }{{"10", "1", "100", "2"}, {"11", "10", "50", "1"}} {
		d := filepath.Join(proc, v.pid)
		os.Mkdir(d, 0700)
		fields := []string{"S", v.ppid}
		for len(fields) < 11 {
			fields = append(fields, "0")
		}
		fields = append(fields, v.ticks, "0")
		for len(fields) < 21 {
			fields = append(fields, "0")
		}
		fields = append(fields, v.rss)
		os.WriteFile(filepath.Join(d, "stat"), []byte(v.pid+" (agent) "+strings.Join(fields, " ")), 0600)
	}
	tracker := Tracker{}
	now := time.Now()
	root := []Process{{PID: 10}}
	first := tracker.Measure(proc, root, now)
	if first[10].RSS != 3*int64(os.Getpagesize()) {
		t.Fatalf("rss %+v", first)
	}
	second := tracker.Measure(proc, root, now.Add(time.Second))
	if second[10].CPU != 0 {
		t.Fatalf("cpu %+v", second)
	}
}

func TestOutsideProjectRootKeepsWorkingDirectoryLocation(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspaces")
	cwd := filepath.Join(base, "coordination")
	os.MkdirAll(root, 0700)
	os.MkdirAll(cwd, 0700)
	project, location := projectForRoots(cwd, []string{root})
	if project != "unmatched" || location != "coordination" {
		t.Fatalf("outside root = %q, %q", project, location)
	}
	sessions := []model.AgentSession{{ID: "session", Kind: "claude", Project: "unknown", At: time.Now()}}
	Enrich(sessions, []Process{{PID: 42, Comm: "claude", CWD: cwd}}, []string{root}, base, time.Now())
	if sessions[0].Project != "unmatched" || sessions[0].Worktree != "coordination" {
		t.Fatalf("session location = %q, %q", sessions[0].Project, sessions[0].Worktree)
	}
}

func TestProcessMatchRequiresRecency(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "uptime"), []byte("1000.0 0.0\n"), 0600)
	cwd := filepath.Join(d, "project")
	os.Mkdir(cwd, 0700)
	now := time.Now()
	sessions := []model.AgentSession{{Kind: "claude", Project: "project", At: now.Add(-2 * time.Minute)}}
	processes := []Process{{PID: 12, Comm: "claude", CWD: cwd, StartTime: 95000}}
	Enrich(sessions, processes, nil, d, now)
	if sessions[0].PID != 0 {
		t.Fatal("later process matched older session")
	}
	processes[0].StartTime = 80000
	Enrich(sessions, processes, nil, d, now)
	if sessions[0].PID != 12 {
		t.Fatal("eligible process unmatched")
	}
}

func TestProcessAssignedOnceAndUnmatchedProcessesVisible(t *testing.T) {
	d := t.TempDir()
	now := time.Now().UTC()
	os.WriteFile(filepath.Join(d, "uptime"), []byte("1000.0 0.0\n"), 0600)
	root := filepath.Join(d, "workspaces")
	cwd := filepath.Join(root, "demo", "tree")
	os.MkdirAll(cwd, 0700)
	sessions := []model.AgentSession{
		{ID: "older", Kind: "claude", Project: "demo", At: now.Add(-10 * time.Minute)},
		{ID: "newer", Kind: "claude", Project: "demo", At: now.Add(-time.Minute)},
	}
	processes := []Process{
		{PID: 12, UID: uint32(os.Getuid()), Comm: "claude", CWD: cwd, StartTime: 10000},
		{PID: 13, UID: uint32(os.Getuid()), Comm: "codex", CWD: cwd, StartTime: 90000},
	}
	Enrich(sessions, processes, []string{root}, d, now)
	if sessions[0].PID != 0 || sessions[1].PID != 12 {
		t.Fatalf("process assigned incorrectly: %+v", sessions)
	}
	sessions = ProcessSessions(sessions, processes, []string{root}, d, now)
	if len(sessions) != 3 || sessions[2].Source != "process" || sessions[2].Kind != "codex" || sessions[2].PID != 13 || sessions[2].Project != "demo" || sessions[2].Worktree != "tree" || sessions[2].User == "" || sessions[2].At.IsZero() {
		t.Fatalf("missing process session: %+v", sessions)
	}
	if sessions[0].CPU != 0 || sessions[0].RSS != 0 || sessions[0].PID != 0 {
		t.Fatal("older session retained process metrics")
	}
}

func TestMergeSessionsKeepsOTLPAndNewerTurnState(t *testing.T) {
	now := time.Now().UTC()
	got := MergeSessions([]model.AgentSession{
		{ID: "same", Kind: "codex", Source: "otlp", At: now.Add(-time.Minute)},
		{ID: "same", Kind: "codex", Source: "log", Model: "gpt-6-sol", Project: "demo", At: now, LastTurnEnded: true},
	})
	if len(got) != 1 || got[0].Source != "otlp" || got[0].Model != "gpt-6-sol" || got[0].Project != "demo" || !got[0].LastTurnEnded || !got[0].At.Equal(now) {
		t.Fatalf("merged session: %+v", got)
	}
}
