package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func fakeLineageStat(t *testing.T, root string, pid, parent, start int) {
	t.Helper()
	fields := make([]string, 22)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[1] = fmt.Sprint(parent)
	fields[19] = fmt.Sprint(start)
	path := filepath.Join(root, fmt.Sprint(pid))
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "stat"), []byte(fmt.Sprintf("%d (worker) %s", pid, strings.Join(fields, " "))), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLineageSurvivesReparentingButRejectsPIDReuse(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	fakeLineageStat(t, root, 100, 1, 1000)
	fakeLineageStat(t, root, 101, 100, 1100)
	tracker := LineageTracker{}
	session := model.AgentSession{ID: Hash("session"), Kind: "codex", PID: 100, State: "working"}
	processes := []model.ProjectProcess{{PID: 101, Name: "node"}}
	tracker.Annotate(root, []model.AgentSession{session}, processes, now)
	if processes[0].AgentSessionID != session.ID || processes[0].LeftBehind {
		t.Fatalf("active child: %+v", processes[0])
	}
	fakeLineageStat(t, root, 101, 1, 1100)
	os.RemoveAll(filepath.Join(root, "100"))
	tracker.Annotate(root, nil, processes, now.Add(time.Second))
	if !processes[0].LeftBehind {
		t.Fatalf("reparented child: %+v", processes[0])
	}
	fakeLineageStat(t, root, 101, 1, 2100)
	tracker.Annotate(root, nil, processes, now.Add(2*time.Second))
	if processes[0].AgentSessionID == session.ID {
		t.Fatalf("reused PID attributed: %+v", processes[0])
	}
}
