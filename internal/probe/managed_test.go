package probe

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestBackupStatusAndInternalFixtures(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	backups := filepath.Join(root, "backups")
	if e := os.Mkdir(backups, 0700); e != nil {
		t.Fatal(e)
	}
	set := config.BackupSet{Name: "db", Pattern: filepath.Join(backups, "*.dump"), MaxAgeSeconds: 3600, MinSizeBytes: 4}
	if got := Backup(set, now); got.State != model.Crit || got.Values["count"] != 0 {
		t.Fatal(got)
	}
	path := filepath.Join(backups, "current.dump")
	if e := os.WriteFile(path, []byte("12345"), 0600); e != nil {
		t.Fatal(e)
	}
	if got := Backup(set, now); got.State != model.OK || got.Values["size_bytes"] != 5 {
		t.Fatal(got)
	}
	old := now.Add(-2 * time.Hour)
	os.Chtimes(path, old, old)
	if got := Backup(set, now); got.State != model.Crit {
		t.Fatal(got)
	}
	status := filepath.Join(root, "status.json")
	file := config.StatusFile{Name: "guard", Path: status, FieldPath: "checks.health_down", OKValues: []string{"false"}, CritValues: []string{"true"}, MaxAgeSeconds: 3600, MissingState: model.Crit}
	if got := Status(file, now); got.State != model.Crit {
		t.Fatal(got)
	}
	if e := os.WriteFile(status, []byte(`{"checks":{"health_down":false}}`), 0600); e != nil {
		t.Fatal(e)
	}
	if got := Status(file, now); got.State != model.OK {
		t.Fatal(got)
	}
	os.Chtimes(status, old, old)
	if got := Status(file, now); got.State != model.Unknown {
		t.Fatal(got)
	}
	if e := os.WriteFile(status, []byte(`{"last_copy":"2020-01-01T00:00:00Z"}`), 0600); e != nil {
		t.Fatal(e)
	}
	file.FieldPath = "last_copy"
	if got := Status(file, now); got.State != model.Unknown {
		t.Fatal(got)
	}
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	p := config.InternalProbe{Name: "relay", Kind: "tcp", Target: ln.Addr().String(), TimeoutMS: 500}
	if got := Internal(context.Background(), p); got.State != model.OK || got.Values["latency_ms"] < 0 {
		t.Fatal(got)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://example.invalid/", http.StatusFound)
			return
		}
		w.WriteHeader(503)
	})}
	go server.Serve(ln)
	defer server.Close()
	p.Kind = "http"
	p.Target = "http://" + ln.Addr().String()
	if got := Internal(context.Background(), p); got.State != model.Warn || got.Values["status_code"] != 503 {
		t.Fatal(got)
	}
	p.Target += "/redirect"
	if got := Internal(context.Background(), p); got.State != model.Warn || got.Values["status_code"] != 302 {
		t.Fatal(got)
	}
}

func TestRepositoriesLocalRefs(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	os.Mkdir(repo, 0700)
	run := func(args ...string) {
		t.Helper()
		out, e := Exec(context.Background(), "git", append([]string{"-C", repo}, args...)...)
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, out)
		}
	}
	run("init", "-q")
	run("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "first")
	results := Repositories(context.Background(), []string{root}, nil, nil, time.Now(), nil)
	if len(results) != 2 || results[1].State != model.Warn || results[1].Values["has_upstream"] != 0 {
		t.Fatal(results)
	}
	if results[1].Detail != "no upstream" {
		t.Fatal(results[1].Detail)
	}
	local := Repositories(context.Background(), []string{root}, []string{"app"}, nil, time.Now(), nil)
	if local[1].State != model.OK || local[1].Values["local_only"] != 1 {
		t.Fatal(local)
	}
	os.WriteFile(filepath.Join(repo, "note"), []byte("dirty"), 0600)
	results = Repositories(context.Background(), []string{root}, nil, nil, time.Now(), nil)
	if results[1].Values["dirty"] != 1 {
		t.Fatal(results)
	}
	if results[1].Detail != "1 uncommitted file, no upstream" {
		t.Fatal(results[1].Detail)
	}
}

func TestMarkerReturnsOnlyMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.log")
	now := time.Now().UTC()
	line := now.Add(-time.Minute).Format(time.RFC3339) + " OK backup secret=must-not-leak\n"
	if e := os.WriteFile(path, []byte(line), 0600); e != nil {
		t.Fatal(e)
	}
	got := Marker(config.LogMarker{Name: "backup_verified", Path: path, Contains: "OK backup", MaxAgeSeconds: 3600}, now)
	if got.State != model.OK || got.Values["count"] != 1 || got.Detail == line {
		t.Fatal(got)
	}
	alert := Marker(config.LogMarker{Name: "alert", Path: path, Contains: "ALERT", MaxAgeSeconds: 3600, PresentState: model.Crit, AbsentState: model.OK}, now)
	if alert.State != model.OK || alert.Values["count"] != 0 {
		t.Fatal(alert)
	}
}
