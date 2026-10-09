package store

import (
	"path/filepath"
	"testing"
)

func TestOnlineBackupIsValidAndDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB.Exec(`INSERT INTO hosts(id,labels,at_ns) VALUES('fixture','{}',1)`); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "backups", "snapshot.db")
	if err := s.Backup(destination); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(destination); err == nil {
		t.Fatal("backup overwrote existing file")
	}
	copy, err := Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	var result string
	if err := copy.DB.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil || result != "ok" {
		t.Fatalf("integrity_check: %q, %v", result, err)
	}
	var count int
	if err := copy.DB.QueryRow(`SELECT COUNT(*) FROM hosts WHERE id='fixture'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backup rows: %d, %v", count, err)
	}
}
