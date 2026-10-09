package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHostTokenStoredHashedAndLiveness(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "fleet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.AddHost("test-host", nil, "one-time-token"); err != nil {
		t.Fatal(err)
	}
	var hash []byte
	if err := s.DB.QueryRow(`SELECT token_hash FROM host_tokens WHERE host_id='test-host'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if string(hash) == "one-time-token" || len(hash) != 32 {
		t.Fatal("token was not stored as SHA-256 hash")
	}
	if ok, err := s.AuthorizeHost("test-host", "one-time-token"); err != nil || !ok {
		t.Fatalf("auth failed: %v", err)
	}
	now := time.Now()
	for _, c := range []struct {
		age  time.Duration
		want string
	}{{44 * time.Second, "live"}, {45 * time.Second, "silent"}, {23 * time.Hour, "silent"}, {24 * time.Hour, "offline"}} {
		if got := HostLiveness(now.Add(-c.age), now); got != c.want {
			t.Fatalf("age %s: %s", c.age, got)
		}
	}
}
