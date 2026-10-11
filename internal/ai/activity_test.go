package ai

import (
	"encoding/json"
	"github.com/thejoyfulist/fleetglass/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileToolNames(t *testing.T) {
	for _, name := range []string{"Edit", "MultiEdit", "Write", "NotebookEdit", "apply_patch", "functions.apply_patch", "mcp__codex__apply_patch", "edit_file", "write_file", "functions.edit_file", "functions.write_file"} {
		if !IsFileTool(name) {
			t.Fatalf("missed %s", name)
		}
	}
	for _, name := range []string{"Read", "Bash", "exec_command", "tool_result", ""} {
		if IsFileTool(name) {
			t.Fatalf("false edit %s", name)
		}
	}
}

func TestAssessActivity(t *testing.T) {
	now := time.Now().UTC()
	rate := 12.0
	cases := []struct {
		name    string
		session model.AgentSession
		want    string
		flag    bool
	}{
		{"editing", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-time.Hour), LastToolAt: now, LastFileEditAt: now.Add(-19 * time.Minute)}, "editing", false},
		{"real loop at threshold", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-time.Hour), LastToolAt: now, LastUsageAt: now, LastFileEditAt: now.Add(-20 * time.Minute), SpeedTokensPerSecond: &rate}, "working, no file changes for 20m (possibly looping)", true},
		{"fresh tracking of old session", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-29 * time.Hour), FirstToolAt: now.Add(-2 * time.Minute), LastToolAt: now, LastUsageAt: now, SpeedTokensPerSecond: &rate}, "working, no file changes since tracking began 2m ago", false},
		{"historical edit before tracking", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-29 * time.Hour), FirstToolAt: now.Add(-2 * time.Minute), LastToolAt: now, LastUsageAt: now, LastFileEditAt: now.Add(-time.Hour), SpeedTokensPerSecond: &rate}, "working, no file changes since tracking began 2m ago", false},
		{"shell without worktree or tokens", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-27 * time.Minute), FirstToolAt: now.Add(-27 * time.Minute), LastToolAt: now}, "working", false},
		{"shell without worktree but tokens", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-27 * time.Minute), FirstToolAt: now.Add(-27 * time.Minute), LastToolAt: now, LastUsageAt: now, SpeedTokensPerSecond: &rate}, "working, no file changes for 27m (possibly looping)", true},
		{"repo change from shell", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-time.Hour), Worktree: "tree", FirstToolAt: now.Add(-time.Hour), LastToolAt: now, LastUsageAt: now, LastFileEditAt: now.Add(-time.Minute), SpeedTokensPerSecond: &rate, Provenance: model.AgentProvenance{FileEdit: "repo"}}, "editing", false},
		{"idle", model.AgentSession{State: "idle", At: now, StartedAt: now.Add(-time.Hour), LastToolAt: now.Add(-time.Minute), LastFileEditAt: now.Add(-30 * time.Minute)}, "idle", false},
		{"unknown", model.AgentSession{State: "working", At: now, StartedAt: now.Add(-time.Hour)}, "unknown", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.session
			AssessActivity(&s, now, 20)
			if s.Activity != tc.want || s.Flag != tc.flag {
				t.Fatalf("%+v", s)
			}
		})
	}
}

func TestLogToolNamesOnly(t *testing.T) {
	claude := map[string]json.RawMessage{"message": []byte(`{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"private"}},{"type":"tool_use","name":"Read"}]}`)}
	got := logToolNames("claude", claude)
	if len(got) != 2 || got[0] != "Edit" || got[1] != "Read" {
		t.Fatal(got)
	}
	codex := map[string]json.RawMessage{"type": []byte(`"response_item"`), "payload": []byte(`{"type":"function_call","name":"apply_patch","arguments":"private"}`)}
	got = logToolNames("codex", codex)
	if len(got) != 1 || got[0] != "apply_patch" {
		t.Fatal(got)
	}
}

func TestLogReaderRecordsToolTimestamps(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct{ kind, line, tool string }{
		{"claude", `{"type":"assistant","sessionId":"synthetic","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"hidden"}}]}}`, "Edit"},
		{"codex", `{"type":"response_item","payload":{"type":"function_call","name":"apply_patch","arguments":"hidden"}}`, "apply_patch"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "synthetic.jsonl")
			line := `{"timestamp":"` + now.Format(time.RFC3339Nano) + `",` + tc.line[1:] + "\n"
			if err := os.WriteFile(path, []byte(line), 0600); err != nil {
				t.Fatal(err)
			}
			got := NewLogReader([]Root{{Kind: tc.kind, Path: root}}).Scan(now.Add(time.Second))
			if len(got.Sessions) != 1 || got.Sessions[0].LastTool != tc.tool || got.Sessions[0].LastFileEditAt.IsZero() || got.Sessions[0].Provenance.FileEdit != "log" || !got.Sessions[0].FirstToolAt.Equal(now.Add(time.Second)) {
				t.Fatalf("%+v", got.Sessions)
			}
		})
	}
}
