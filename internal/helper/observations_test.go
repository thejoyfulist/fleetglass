package helper

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/thejoyfulist/fleetglass/internal/projects"
)

func TestObservationRoutesUseConfiguredRoots(t *testing.T) {
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "private.txt"), []byte("fixture"), 0644); e != nil {
		t.Fatal(e)
	}
	h := Handler{ProcRoot: filepath.Join(root, "proc"), StorageRoots: []string{root}, ProjectRoots: []string{root}, ProjectCollector: &projects.Collector{}, ProjectMu: &sync.Mutex{}}
	if e := os.Mkdir(h.ProcRoot, 0755); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		path   string
		status int
	}{{"/storage/0", 200}, {"/storage/1", 404}, {"/storage/../0", 404}, {"/projects", 200}, {"/projects?root=/", 404}} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: got %d, want %d", tc.path, w.Code, tc.status)
		}
		if tc.path == "/storage/0" && w.Code == 200 && strings.Contains(w.Body.String(), "private.txt") {
			t.Fatal("storage response exposed a file name")
		}
	}
}
