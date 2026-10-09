package dockerapi

import (
	"encoding/json"
	"testing"
)

func TestHealthFormats(t *testing.T) {
	cases := map[string]Health{
		`{"Health":{"Status":"healthy","FailingStreak":0}}`:   "healthy",
		`{"Health":{"Status":"unhealthy","FailingStreak":3}}`: "unhealthy",
		`{"Health":"starting"}`:                               "starting",
		`{"Health":null}`:                                     "",
		`{}`:                                                  "",
		`{"Health":{"Status":"secret-token","Log":[{"Output":"private"}]}}`: "",
		`{"Health":"private"}`: "",
	}
	for in, want := range cases {
		var v struct {
			Health Health `json:"Health"`
		}
		if err := json.Unmarshal([]byte(in), &v); err != nil || v.Health != want {
			t.Fatalf("%s: got %q, %v", in, v.Health, err)
		}
	}
	var v struct {
		Health Health `json:"Health"`
	}
	if err := json.Unmarshal([]byte(`{"Health":42}`), &v); err == nil {
		t.Fatal("number accepted as health")
	}
}
