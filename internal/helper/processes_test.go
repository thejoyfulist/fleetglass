package helper

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentProcessEndpointAllowlist(t *testing.T) {
	root := t.TempDir()
	for pid, comm := range map[string]string{"101": "claude", "102": "bash"} {
		p := filepath.Join(root, pid)
		os.Mkdir(p, 0700)
		os.WriteFile(filepath.Join(p, "comm"), []byte(comm+"\n"), 0600)
		os.WriteFile(filepath.Join(p, "stat"), []byte(pid+" ("+comm+") S 1 "+strings.Repeat("0 ", 18)+"23 0"), 0600)
		os.Symlink("/tmp/project", filepath.Join(p, "cwd"))
	}
	h := Handler{ProcRoot: root, AgentCommands: []string{"claude"}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/agents", nil))
	var got []map[string]any
	if json.Unmarshal(w.Body.Bytes(), &got) != nil {
		t.Fatal(w.Body.String())
	}
	if len(got) != 1 || got[0]["pid"] != float64(101) {
		t.Fatalf("processes: %+v", got)
	}
	for _, forbidden := range []string{"cmdline", "environ", "comm", "secret"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("leaked %s", forbidden)
		}
	}
	if custom := Processes(root, []string{"bash"}); len(custom) != 1 || custom[0].PID != 102 {
		t.Fatalf("configured command missing: %+v", custom)
	}
}

func TestProcessRootValidation(t *testing.T) {
	if e := (Handler{ProcRoot: "relative"}).Validate(); e == nil {
		t.Fatal("relative proc root accepted")
	}
	if e := (Handler{ProcRoot: "/proc", AgentCommands: []string{"bad/name"}}).Validate(); e == nil {
		t.Fatal("invalid command name accepted")
	}
}
