package helper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

type SensitiveFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	MtimeNS int64  `json:"mtime_ns"`
	SHA256  string `json:"sha256"`
}

var sensitiveFixed = []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/sudoers", "/etc/ssh/sshd_config", "/root/.ssh/authorized_keys"}

// SensitiveFiles reads a fixed allowlist, with no symlink traversal at the file itself.
func SensitiveFiles(root string) ([]SensitiveFile, error) {
	if root == "" {
		root = "/"
	}
	paths := append([]string{}, sensitiveFixed...)
	for _, pattern := range []string{"/etc/sudoers.d/*", "/home/*/.ssh/authorized_keys"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, err
		}
		for _, p := range matches {
			rel, err := filepath.Rel(root, p)
			if err == nil {
				paths = append(paths, "/"+rel)
			}
		}
	}
	sort.Strings(paths)
	out := []SensitiveFile{}
	for _, path := range paths {
		actual := filepath.Join(root, path)
		parent := root
		safe := true
		for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/")[:len(strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/"))-1] {
			parent = filepath.Join(parent, part)
			info, err := os.Lstat(parent)
			if err != nil {
				if os.IsNotExist(err) {
					safe = false
					break
				}
				return nil, fmt.Errorf("sensitive directory unreadable")
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				safe = false
				break
			}
		}
		if !safe {
			continue
		}
		fd, err := syscall.Open(actual, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if os.IsNotExist(err) || err == syscall.ELOOP {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("sensitive file unreadable")
		}
		f := os.NewFile(uintptr(fd), actual)
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		if !info.Mode().IsRegular() {
			f.Close()
			continue
		}
		if info.Size() > 1<<20 {
			f.Close()
			return nil, fmt.Errorf("sensitive file too large")
		}
		hash := sha256.New()
		_, err = io.Copy(hash, io.LimitReader(f, 1<<20+1))
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("sensitive file unreadable")
		}
		out = append(out, SensitiveFile{Path: path, Size: info.Size(), MtimeNS: info.ModTime().UnixNano(), SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	return out, nil
}
