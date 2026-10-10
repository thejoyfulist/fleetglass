package ai

import (
	"github.com/thejoyfulist/fleetglass/internal/model"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLogIncrementalDedupRotationAndSymlink(t *testing.T) {
	d := t.TempDir()
	root := filepath.Join(d, "projects")
	os.Mkdir(root, 0700)
	project := filepath.Join(root, "-home-agent-projects-demo")
	os.Mkdir(project, 0700)
	path := filepath.Join(project, "s.jsonl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	line := `{"timestamp":"` + now + `","sessionId":"private","message":{"id":"m1","usage":{"input_tokens":10}}}` + "\n"
	os.WriteFile(path, []byte(line+strings.TrimSuffix(line, "\n")), 0600)
	r := NewLogReader([]Root{{Kind: "claude", Path: root}})
	v := r.Scan(time.Now())
	if len(v.Usage) != 1 || v.Usage[0].Tokens.Input != 10 {
		t.Fatalf("first scan: %+v", v)
	}
	if len(r.Scan(time.Now()).Usage) != 0 {
		t.Fatal("incomplete line was consumed")
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.Write([]byte("\n"))
	f.Close()
	v = r.Scan(time.Now())
	if len(v.Usage) != 1 {
		t.Fatalf("completed line: %+v", v)
	}
	os.Remove(path)
	os.WriteFile(path, []byte(strings.Replace(line, `"input_tokens":10`, `"input_tokens":20`, 1)), 0600)
	v = r.Scan(time.Now())
	if len(v.Usage) != 1 || v.Usage[0].Tokens.Input != 20 {
		t.Fatalf("rotation: %+v", v)
	}
	os.Symlink(path, filepath.Join(project, "link.jsonl"))
	if len(r.Scan(time.Now()).Usage) != 0 {
		t.Fatal("symlink followed")
	}
}

func TestClaudeEndTurnFromLog(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "session.jsonl")
	now := time.Now().UTC()
	line := `{"timestamp":"` + now.Format(time.RFC3339Nano) + `","sessionId":"private","type":"assistant","message":{"id":"final","stop_reason":"end_turn","usage":{"output_tokens":7}}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	sessions := NewLogReader([]Root{{Kind: "claude", Path: d}}).Scan(now).Sessions
	if len(sessions) != 1 || !sessions[0].LastTurnEnded {
		t.Fatalf("Claude end turn missing: %+v", sessions)
	}
}
func TestQuotaPace(t *testing.T) {
	now := time.Now().UTC()
	q := model.Quota{Used: 50, ObservedAt: now, ResetsAt: now.Add(4 * time.Hour)}
	p := QuotaProjection(q, 300)
	if p.Before(now) || p.After(now.Add(2*time.Hour)) {
		t.Fatalf("projection %v", p)
	}
}

func TestCodexLogUsageAndQuota(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "2026", "10", "file.jsonl")
	os.MkdirAll(filepath.Dir(p), 0700)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	reset := time.Now().Add(4 * time.Hour).Unix()
	lines := []string{`{"timestamp":"` + now + `","type":"session_meta","payload":{"id":"private-session","cwd":"/srv/projects/demo"}}`, `{"timestamp":"` + now + `","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":20}},"rate_limits":{"limit_id":"codex","primary":{"used_percent":64,"window_minutes":10080,"resets_at":` + strconv.FormatInt(time.Now().Add(6*24*time.Hour).Unix(), 10) + `},"secondary":{"used_percent":28,"window_minutes":300,"resets_at":` + strconv.FormatInt(reset, 10) + `}}}}`}
	os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0600)
	r := NewLogReader([]Root{{Kind: "codex", Path: d}})
	got := r.Scan(time.Now())
	if len(got.Usage) != 1 || got.Usage[0].Tokens.Input != 60 || got.Usage[0].Tokens.CacheRead != 40 || len(got.Quotas) != 2 || got.Usage[0].Session != Hash("private-session") {
		t.Fatalf("codex: %+v", got)
	}
	if got.Usage[0].Project != "demo" {
		t.Fatalf("codex project = %q", got.Usage[0].Project)
	}
	windows := map[string]model.Quota{}
	for _, q := range got.Quotas {
		windows[q.Window] = q
	}
	if windows["primary"].WindowMinutes != 10080 || windows["primary"].Used != 64 || windows["secondary"].WindowMinutes != 300 || windows["secondary"].Used != 28 {
		t.Fatalf("Codex rate limit windows mismatched: %+v", got.Quotas)
	}
}

func TestCodexProjectFromConfiguredWorktreeRoot(t *testing.T) {
	d := t.TempDir()
	worktreeRoot := filepath.Join(d, "workspaces")
	logRoot := filepath.Join(d, "logs")
	if err := os.MkdirAll(logRoot, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	line := `{"timestamp":"` + now.Format(time.RFC3339Nano) + `","type":"session_meta","payload":{"id":"demo-session","cwd":"` + filepath.Join(worktreeRoot, "atlas", "feature-home", "src") + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(logRoot, "rollout.jsonl"), []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewLogReader([]Root{{Kind: "codex", Path: logRoot}})
	r.WorktreeRoots = []string{worktreeRoot}
	got := r.Scan(now)
	if len(got.Sessions) != 1 || got.Sessions[0].Project != "atlas" {
		t.Fatalf("codex worktree project: %+v", got.Sessions)
	}
}

func TestCodexOutsideConfiguredRootUsesCWDLocation(t *testing.T) {
	d := t.TempDir()
	logRoot := filepath.Join(d, "logs")
	os.MkdirAll(logRoot, 0700)
	cwd := filepath.Join(d, "coordination")
	now := time.Now().UTC()
	line := `{"timestamp":"` + now.Format(time.RFC3339Nano) + `","type":"session_meta","payload":{"id":"demo-session","cwd":"` + cwd + `"}}` + "\n"
	os.WriteFile(filepath.Join(logRoot, "rollout.jsonl"), []byte(line), 0600)
	r := NewLogReader([]Root{{Kind: "codex", Path: logRoot}})
	r.WorktreeRoots = []string{filepath.Join(d, "workspaces")}
	got := r.Scan(now)
	if len(got.Sessions) != 1 || got.Sessions[0].Project != "unmatched" || got.Sessions[0].Worktree != "coordination" {
		t.Fatalf("outside root location: %+v", got.Sessions)
	}
}

func TestCodexModelFromAnonymizedRollout(t *testing.T) {
	fixture, err := os.ReadFile("testdata/codex-rollout-anonymized.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	d := t.TempDir()
	p := filepath.Join(d, "rollout.jsonl")
	now := time.Now().UTC()
	text := strings.ReplaceAll(string(fixture), "TIMESTAMP", now.Format(time.RFC3339Nano))
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewLogReader([]Root{{Kind: "codex", Path: d}})
	got := r.Scan(now)
	if len(got.Usage) != 1 || got.Usage[0].Model != "gpt-6-sol" || len(got.Sessions) != 1 || got.Sessions[0].Model != "gpt-6-sol" || !got.Sessions[0].LastTurnEnded {
		t.Fatalf("Codex model or turn state missing: %+v", got)
	}
}

func TestOversizedLogLineSkipped(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "big.jsonl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	valid := `{"timestamp":"` + now + `","sessionId":"s","message":{"id":"m","usage":{"input_tokens":7}}}` + "\n"
	os.WriteFile(p, []byte(strings.Repeat("x", 1<<20+2)+"\n"+valid), 0600)
	r := NewLogReader([]Root{{Kind: "claude", Path: d}})
	r.ByteLimit = 2 << 20
	got := r.Scan(time.Now())
	if len(got.Usage) != 1 || got.Usage[0].Tokens.Input != 7 {
		t.Fatalf("oversized line stalled reader: %+v", got)
	}
}

func TestEncodedProjectLongestMatch(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".claude", "projects")
	encoded := strings.ReplaceAll(home, string(filepath.Separator), "-")
	path := filepath.Join(root, encoded+"-orca-workspaces-example-project-tree", "x.jsonl")
	if got := projectFromLog(root, path, []string{"the", "example-project"}); got != "example-project" {
		t.Fatalf("project=%q", got)
	}
}
