package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestCachedWorktreeChangeCreditsShellSession(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name          string
		before, after model.WorktreeEntry
		wantRepo      bool
	}{
		{name: "unchanged", before: model.WorktreeEntry{Head: "a", DirtyFiles: 1, StatusComplete: true}, after: model.WorktreeEntry{Head: "a", DirtyFiles: 1, StatusComplete: true}},
		{name: "dirty count", before: model.WorktreeEntry{Head: "a", StatusComplete: true}, after: model.WorktreeEntry{Head: "a", DirtyFiles: 1, StatusComplete: true}, wantRepo: true},
		{name: "incomplete status", before: model.WorktreeEntry{Head: "a", DirtyFiles: 1, StatusComplete: true}, after: model.WorktreeEntry{Head: "a"}},
		{name: "head moved", before: model.WorktreeEntry{Head: "a"}, after: model.WorktreeEntry{Head: "b"}, wantRepo: true},
		{name: "index changed", before: model.WorktreeEntry{Head: "a", IndexMTime: now.Add(-time.Minute)}, after: model.WorktreeEntry{Head: "a", IndexMTime: now}, wantRepo: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			root := filepath.Join(t.TempDir(), "trees")
			path := filepath.Join(root, "demo", "feature")
			base := now.Add(-time.Minute)
			id := strings.Repeat("a", 64)
			session := model.AgentSession{ID: id, Kind: "codex", Project: "demo", Worktree: "feature", At: now, StartedAt: now.Add(-time.Hour), FirstToolAt: now.Add(-time.Hour), LastToolAt: now, State: "working", Source: "log"}
			tree := tc.before
			tree.Root, tree.Path, tree.Project, tree.Name = root, path, "demo", "feature"
			if err := s.Ingest(model.Batch{HostID: "host", At: base, AI: &model.AI{Sessions: []model.AgentSession{session}}, Worktrees: []model.WorktreeEntry{tree}}); err != nil {
				t.Fatal(err)
			}
			tree = tc.after
			tree.Root, tree.Path, tree.Project, tree.Name = root, path, "demo", "feature"
			if err := s.Ingest(model.Batch{HostID: "host", At: now, Worktrees: []model.WorktreeEntry{tree}}); err != nil {
				t.Fatal(err)
			}
			got, err := s.AgentsWithSummary("host", nil)
			if err != nil || len(got) != 1 {
				t.Fatalf("sessions: %+v %v", got, err)
			}
			if (got[0].Provenance.FileEdit == "repo") != tc.wantRepo {
				t.Fatalf("repo change: %+v", got[0])
			}
			if tc.wantRepo && !got[0].LastFileEditAt.Equal(now) {
				t.Fatalf("repo timestamp: %+v", got[0])
			}
		})
	}
}

func TestFirstToolObservationPersistsAcrossSessionUpdates(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "first-tool.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	first := now.Add(-time.Minute)
	for _, session := range []model.AgentSession{
		{ID: id, Kind: "codex", At: first, StartedAt: now.Add(-time.Hour), FirstToolAt: first, LastToolAt: first, State: "working", Source: "log"},
		{ID: id, Kind: "codex", At: now, StartedAt: now.Add(-time.Hour), LastToolAt: now, State: "working", Source: "log"},
	} {
		if err := s.Ingest(model.Batch{HostID: "host", At: session.At, AI: &model.AI{Sessions: []model.AgentSession{session}}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Agents("host")
	if err != nil || len(got) != 1 || !got[0].FirstToolAt.Equal(first) {
		t.Fatalf("first observed tool lost: %+v %v", got, err)
	}
}
