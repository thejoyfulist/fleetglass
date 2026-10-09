package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSocketPrivateAtCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "helper.sock")
	l, err := protectedListen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("socket permissions at creation: %04o", got)
	}
}
