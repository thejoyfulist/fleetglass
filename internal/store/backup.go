package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Backup uses SQLite's online VACUUM INTO snapshot while the live database stays open.
func (s *Store) Backup(path string) error {
	if path == "" {
		return fmt.Errorf("backup path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return fmt.Errorf("backup destination already exists")
	}
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// SQLite string literals use doubled quotes; a prepared parameter is not supported by VACUUM INTO.
	quoted := "'" + strings.ReplaceAll(path, "'", "''") + "'"
	if _, err := s.DB.Exec("VACUUM INTO " + quoted); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}
