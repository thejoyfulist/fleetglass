package probe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestJournalFixtures(t *testing.T) {
	errors, err := os.ReadFile("testdata/errors.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := os.ReadFile("testdata/auth.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	rows := Journal(context.Background(), func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "-p err") {
			return errors, nil
		}
		return auth, nil
	})
	lookup := func(name string) model.Result {
		for _, r := range rows {
			if r.Name == name {
				return r
			}
		}
		t.Fatalf("missing %s", name)
		return model.Result{}
	}
	if got := lookup("recent_errors").Values["count"]; got != 3 {
		t.Fatalf("errors=%v", got)
	}
	if got := lookup("recent_errors").Values["oom_count"]; got != 1 {
		t.Fatalf("OOM=%v", got)
	}
	if got := lookup("security:ssh_logins").Fields["source_class"]; got != "public" {
		t.Fatalf("class=%s", got)
	}
	if got := lookup("security:failed_source:0").Fields["source_class"]; got != "public" {
		t.Fatalf("failed class=%s", got)
	}
	if got := lookup("security:sudo_user:0").Fields["user"]; got != "agent" {
		t.Fatalf("sudo=%s", got)
	}
	for _, r := range rows {
		if strings.Contains(r.Detail, "203.0.113.42") || strings.Contains(r.Fields["message"], "pass@example") || strings.Contains(r.Fields["message"], "Bearer abc") {
			t.Fatalf("sensitive log data retained: %s", r.Name)
		}
	}
}
func TestUpdatesFromLocalState(t *testing.T) {
	root := t.TempDir()
	lists := filepath.Join(root, "lists")
	os.Mkdir(lists, 0700)
	os.WriteFile(filepath.Join(root, "status"), []byte("Package: a\nVersion: 1.0\nStatus: install ok installed\n\nPackage: b\nVersion: 2.0\nStatus: install ok installed\n"), 0600)
	os.WriteFile(filepath.Join(lists, "mirror-security_main_Packages"), []byte("Package: a\nVersion: 1.1\n\nPackage: b\nVersion: 1.9\n"), 0600)
	os.WriteFile(filepath.Join(lists, "mirror-updates_main_Packages"), []byte("Package: a\nVersion: 1.2\n"), 0600)
	r := Updates(context.Background(), lists, filepath.Join(root, "status"), filepath.Join(root, "reboot"))
	if r.State != model.Warn || r.Values["total"] != 1 || r.Values["security"] != 1 {
		t.Fatalf("unexpected updates: %+v", r)
	}
	if !newer("1.0~rc1", "1.0~beta") || newer("1.0~rc1", "1.0") || !newer("2:1.0", "1:9.0") {
		t.Fatal("Debian version ordering")
	}
}
