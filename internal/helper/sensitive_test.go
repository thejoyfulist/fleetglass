package helper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSensitiveFilesAllowlist(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "etc", "sudoers.d"), 0700)
	os.WriteFile(filepath.Join(root, "etc", "passwd"), []byte("safe"), 0600)
	os.WriteFile(filepath.Join(root, "etc", "sudoers.d", "site"), []byte("rule"), 0600)
	os.Symlink(filepath.Join(root, "outside"), filepath.Join(root, "etc", "shadow"))
	rows, err := SensitiveFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].SHA256 == "" || rows[1].SHA256 == "" {
		t.Fatalf("unexpected allowed files: %+v", rows)
	}
}
