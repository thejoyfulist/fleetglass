package helper

import (
	"encoding/json"
	"github.com/thejoyfulist/fleetglass/internal/rapl"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRAPLResponseContainsDomains(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "class/powercap/intel-rapl:0")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"name": "package-0", "energy_uj": "10", "max_energy_range_uj": "100"} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	(Handler{SysRoot: root}).ServeHTTP(w, httptest.NewRequest("GET", "/rapl", nil))
	var sample rapl.Sample
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &sample) != nil || len(sample.Domains) != 1 {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
}
