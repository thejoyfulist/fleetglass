package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestDailyBackupRetention(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups := filepath.Join(dir, "backups")
	if err := os.Mkdir(backups, 0700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(backups, "fleetglass-2026-01-01.db")
	foreign := filepath.Join(backups, "other.db")
	for _, path := range []string{old, foreign} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	if err := dailyBackup(s, backups, 7, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("old Fleetglass backup retained")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("foreign file removed")
	}
	if _, err := os.Stat(filepath.Join(backups, "fleetglass-2026-02-01.db")); err != nil {
		t.Fatal("today's backup missing")
	}
	if err := dailyBackup(s, backups, 7, now); err != nil {
		t.Fatal("second run must be idempotent:", err)
	}
}
