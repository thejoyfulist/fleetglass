package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestUpdateCheckDisabledByDefault(t *testing.T) {
	a := New(nil, "", "fixture")
	a.Version = "1.2.3"
	a.Commit = "abcdef123456"
	w := httptest.NewRecorder()
	a.version(w, httptest.NewRequest("GET", "/api/v1/version", nil))
	var body struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Latest  string `json:"latest"`
		Enabled bool   `json:"update_check_enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "1.2.3" || body.Commit != "abcdef123456" || body.Latest != "" || body.Enabled {
		t.Fatalf("unexpected version response: %+v", body)
	}
}
