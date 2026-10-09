package ai

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestHealthTimelineAndStaleWorktree(t *testing.T) {
	now := time.Now().UTC()
	sessions := []model.AgentSession{{Kind: "claude", PID: 12, LastTurnEnded: true, At: now.Add(-6 * time.Minute)}, {Kind: "codex", PID: 13, At: now.Add(-time.Minute)}}
	processes := []Process{{PID: 12, Comm: "claude"}, {PID: 13, Comm: "codex"}}
	quotas := []model.Quota{{Kind: "codex", Used: 100, ResetsAt: now.Add(time.Hour)}}
	ApplyHealth(sessions, processes, quotas, now)
	if sessions[0].State != "waiting-for-input" || sessions[0].ReasonCode != "awaiting_input" || sessions[1].State != "rate-limited" || sessions[1].ReasonCode != "quota_reached" {
		t.Fatalf("states: %+v", sessions)
	}
	root := t.TempDir()
	p := filepath.Join(root, "demo", "tree")
	os.MkdirAll(p, 0700)
	old := now.Add(-4 * 24 * time.Hour)
	os.Chtimes(p, old, old)
	stale := StaleWorktrees([]string{root}, nil, now, 72*time.Hour)
	if len(stale) != 1 || stale[0].State != "stale-worktree" || stale[0].ReasonCode != "stale_worktree" {
		t.Fatalf("stale: %+v", stale)
	}
	if len(StaleWorktrees([]string{root}, []Process{{CWD: p}}, now, 72*time.Hour)) != 0 {
		t.Fatal("active worktree flagged")
	}
	os.WriteFile(filepath.Join(p, "changed.txt"), []byte("x"), 0600)
	os.Chtimes(p, old, old)
	if len(StaleWorktrees([]string{root}, nil, now, 72*time.Hour)) != 0 {
		t.Fatal("recent file change ignored")
	}
}

func TestCodexCompletedTurnAndProcessOnlyHealth(t *testing.T) {
	now := time.Now().UTC()
	sessions := []model.AgentSession{
		{Kind: "codex", PID: 12, LastTurnEnded: true, At: now.Add(-6 * time.Minute)},
		{Kind: "codex", LastTurnEnded: true, At: now.Add(-6 * time.Minute)},
		{Kind: "codex", At: now.Add(-6 * time.Minute)},
		{Kind: "claude", PID: 13, Source: "process", At: now.Add(-time.Hour)},
	}
	ApplyHealth(sessions, []Process{{PID: 12}, {PID: 13}}, nil, now)
	want := []string{"waiting-for-input", "ended", "unknown", "running"}
	for i, s := range sessions {
		if s.State != want[i] {
			t.Fatalf("session %d: %+v", i, s)
		}
	}
}

func TestInactiveSessionWithoutProcessEndsAfterThirtyMinutes(t *testing.T) {
	now := time.Now().UTC()
	sessions := []model.AgentSession{
		{Kind: "claude", At: now.Add(-29 * time.Minute), Source: "log"},
		{Kind: "claude", At: now.Add(-31 * time.Minute), Source: "log"},
		{Kind: "codex", PID: 42, At: now.Add(-31 * time.Minute), Source: "otlp"},
		{Kind: "codex", PID: 43, At: now.Add(-31 * time.Minute), Source: "log"},
	}
	ApplyHealth(sessions, []Process{{PID: 43}}, nil, now)
	for i, want := range []string{"unknown", "ended", "ended", "unknown"} {
		if sessions[i].State != want {
			t.Fatalf("session %d: got %q, want %q", i, sessions[i].State, want)
		}
	}
}
