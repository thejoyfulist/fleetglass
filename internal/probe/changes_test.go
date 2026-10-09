package probe

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
)

func TestChangeFixturesSelectMetadataOnly(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	history := "Start-Date: 2026-10-08  10:00:00\nUpgrade: curl:amd64 (1, 2), openssl:amd64 (3, 4)\nEnd-Date: 2026-10-08  10:00:05\n\n"
	if e := os.WriteFile(filepath.Join(root, "history.log"), []byte(history), 0600); e != nil {
		t.Fatal(e)
	}
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	z.Write([]byte(history))
	z.Close()
	os.WriteFile(filepath.Join(root, "history.log.2.gz"), compressed.Bytes(), 0600)
	apt := APTChanges(root, now)
	if len(apt) != 2 || apt[0].Text != "Upgraded 2 packages: curl, openssl" {
		t.Fatal(apt)
	}
	journal := fmt.Sprintf(`{"_SYSTEMD_UNIT":"caddy.service","MESSAGE":"Started password=private service","__REALTIME_TIMESTAMP":"%d"}`+"\n", now.Add(-time.Hour).UnixMicro())
	service := JournalChanges([]byte(journal), []string{"caddy.service"}, now)
	if len(service) != 1 || service[0].Text != "caddy.service restarted" || strings.Contains(service[0].Text, "private") {
		t.Fatal(service)
	}
	manager := strings.Replace(journal, `"_SYSTEMD_UNIT":"caddy.service"`, `"UNIT":"caddy.service"`, 1)
	if got := JournalChanges([]byte(manager), []string{"caddy.service"}, now); len(got) != 1 {
		t.Fatal(got)
	}
	os.WriteFile(filepath.Join(root, "stat"), []byte(fmt.Sprintf("cpu 1 2 3\nbtime %d\n", now.Add(-2*time.Hour).Unix())), 0600)
	boot := BootChange(root, now)
	if len(boot) != 1 || boot[0].Text != "Machine restarted" {
		t.Fatal(boot)
	}
	markerPath := filepath.Join(root, "guard.log")
	os.WriteFile(markerPath, []byte(now.Add(-time.Minute).Format(time.RFC3339)+" ALERT token=must-not-leak\n"), 0600)
	marker := MarkerChanges(config.LogMarker{Name: "guard_alert", Path: markerPath, Contains: "ALERT", MaxAgeSeconds: 3600, ChangeEvent: true}, now)
	if len(marker) != 1 || marker[0].Text != "guard alert" || marker[0].ChangeCode != "marker" {
		t.Fatal(marker)
	}
	if v := MarkerChanges(config.LogMarker{Name: "guard_alert", Path: markerPath, Contains: "ALERT", MaxAgeSeconds: 3600}, now); len(v) != 0 {
		t.Fatal(v)
	}
}

func TestMergeChangesUsesDefaultBranchWithoutSubject(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	os.Mkdir(repo, 0700)
	run := func(args ...string) {
		t.Helper()
		b, e := Exec(context.Background(), "git", append([]string{"-C", repo}, args...)...)
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
	}
	run("init", "-q", "-b", "main")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "initial")
	run("checkout", "-qb", "feature")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "private token=secret")
	run("checkout", "-q", "main")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-ff", "-qm", "merge secret=private", "feature")
	out := MergeChanges(context.Background(), []string{root}, nil, time.Now(), nil)
	if len(out) != 1 || out[0].Text != "Merged into app" {
		t.Fatal(out)
	}
}
