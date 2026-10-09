package helper

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestManagedEndpointsAreIndexOnlyAndDoNotReturnContents(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "backup.dump")
	status := filepath.Join(root, "guard.json")
	os.WriteFile(backup, []byte("backup-secret"), 0600)
	os.WriteFile(status, []byte(`{"health_down":false,"token":"private-value"}`), 0600)
	h := Handler{BackupSets: []config.BackupSet{{Name: "db", Pattern: backup, MaxAgeSeconds: 3600, MinSizeBytes: 1}}, StatusFiles: []config.StatusFile{{Name: "guard", Path: status, FieldPath: "health_down", OKValues: []string{"false"}, MaxAgeSeconds: 3600}}, ProcRoot: "/proc"}
	for _, endpoint := range []string{"/backup/0", "/status/0"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", endpoint, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), "private-value") || strings.Contains(w.Body.String(), "backup-secret") || strings.Contains(w.Body.String(), backup) {
			t.Fatalf("leak or failure at %s: %d %s", endpoint, w.Code, w.Body.String())
		}
		var r model.Result
		if e := json.Unmarshal(w.Body.Bytes(), &r); e != nil || r.State != model.OK {
			t.Fatalf("invalid result: %+v %v", r, e)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/status/1", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}

func TestMarkerEventEndpointReturnsOnlyConfiguredMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.log")
	os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)+" ALERT token=private-value\n"), 0600)
	h := Handler{LogMarkers: []config.LogMarker{{Name: "guard_alert", Path: path, Contains: "ALERT", MaxAgeSeconds: 3600, ChangeEvent: true}}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/log-marker-events/0", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "guard alert") || strings.Contains(w.Body.String(), "private-value") || strings.Contains(w.Body.String(), "ALERT") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, route := range []string{"/log-marker-events/1", "/log-marker-events/0?path=/etc/shadow"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", route, nil))
		if w.Code != 404 {
			t.Fatal(route, w.Code)
		}
	}
}
