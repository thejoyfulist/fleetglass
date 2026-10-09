package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestPublicStatusHasNoPrivateFields(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "status.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	samples := []model.ProbeSample{{Name: "public", DisplayName: "Website", Public: true, Kind: "http", State: model.OK, IntervalSeconds: 60, At: now, LatencyMS: 124, DNSMS: 15, StatusCode: 200}, {Name: "private", DisplayName: "Private target", Public: false, Kind: "http", State: model.OK, IntervalSeconds: 60, At: now}}
	if err = s.Ingest(model.Batch{HostID: "internal-host", At: now, Probes: samples}); err != nil {
		t.Fatal(err)
	}
	a := New(s, "secret", "")
	a.ReadToken = "read-secret"
	for _, h := range []struct {
		name    string
		handler interface {
			ServeHTTP(http.ResponseWriter, *http.Request)
		}
	}{{"cockpit", a.Handler()}, {"separate", a.StatusHandler()}} {
		w := httptest.NewRecorder()
		h.handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/status", nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", h.name, w.Code)
		}
		body := w.Body.String()
		for _, secret := range []string{"internal-host", "Private target", "DNSMS", "dns_ms", "status_code", "public\""} {
			if strings.Contains(body, secret) {
				t.Fatalf("%s leaked %q: %s", h.name, secret, body)
			}
		}
	}
	for _, path := range []string{"/api/v1/hosts", "/api/v1/hosts/internal-host/probes", "/"} {
		w := httptest.NewRecorder()
		a.StatusHandler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("separate listener exposed %s: %d", path, w.Code)
		}
	}
}
