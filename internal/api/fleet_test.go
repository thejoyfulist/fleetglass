package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestHostTokensAndFleet(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "fleet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.AddHost("remote", map[string]string{"rack": "north"}, "remote-secret"); err != nil {
		t.Fatal(err)
	}
	a := New(s, "legacy-secret", "local")
	h := a.Handler()
	send := func(host, token string) int {
		b, _ := json.Marshal(model.Batch{HostID: host, At: time.Now().UTC(), Results: []model.Result{{Name: "cpu", State: model.OK, Values: map[string]float64{"busy_percent": 20}}}})
		r := localRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if n := send("remote", "remote-secret"); n != 204 {
		t.Fatalf("remote ingest %d", n)
	}
	for _, x := range []struct{ host, token string }{{"other", "remote-secret"}, {"remote", "legacy-secret"}, {"other", "legacy-secret"}} {
		if n := send(x.host, x.token); n != 401 {
			t.Fatalf("accepted %s with wrong token: %d", x.host, n)
		}
	}
	if n := send("local", "legacy-secret"); n != 204 {
		t.Fatalf("legacy host ingest %d", n)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, localRequest("GET", "/api/v1/fleet", nil))
	if w.Code != 200 {
		t.Fatalf("fleet HTTP %d", w.Code)
	}
	var fleet []fleetHost
	if err := json.Unmarshal(w.Body.Bytes(), &fleet); err != nil {
		t.Fatal(err)
	}
	if len(fleet) != 2 || fleet[1].HostID != "remote" || fleet[1].Labels["rack"] != "north" || fleet[1].Liveness != "live" || fleet[1].CPU == nil || *fleet[1].CPU != 20 {
		t.Fatalf("unexpected fleet: %+v", fleet)
	}
	if err := s.RevokeHost("remote"); err != nil {
		t.Fatal(err)
	}
	if n := send("remote", "remote-secret"); n != 401 {
		t.Fatalf("revoked token accepted: %d", n)
	}
	if err := s.RemoveHost("remote", true); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM samples WHERE host_id='remote'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("purge left %d samples: %v", count, err)
	}
}
