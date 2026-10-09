package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestMetricsUsesReadAuthenticationAndEscapesLabels(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Ingest(model.Batch{HostID: "host", At: time.Now().UTC(), Results: []model.Result{{Name: "cpu", State: model.OK, Values: map[string]float64{"busy_percent": 23}}}, Apps: []model.AppSample{{Name: "unit\"name", Kind: "unit", CPUPercent: 42}}}); err != nil {
		t.Fatal(err)
	}
	a := New(s, "ingest", "host")
	a.ReadToken = "read-secret"
	h := a.Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 401 {
		t.Fatalf("unprotected metrics: %d", w.Code)
	}
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer read-secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `fleetglass_host_metric{host="host",metric="cpu"} 23`) || !strings.Contains(w.Body.String(), `app="unit\"name"`) {
		t.Fatalf("metrics: %d %s", w.Code, w.Body.String())
	}
}
