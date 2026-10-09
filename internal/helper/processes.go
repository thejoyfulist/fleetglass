package helper

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

type Process struct {
	PID       int    `json:"pid"`
	PPID      int    `json:"ppid"`
	UID       uint32 `json:"uid"`
	StartTime uint64 `json:"start_time"`
	CWD       string `json:"cwd"`
	Comm      string `json:"-"`
}

var commPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

func Processes(root string, allow []string) []Process {
	permitted := map[string]bool{}
	for _, v := range allow {
		if commPattern.MatchString(v) {
			permitted[v] = true
		}
	}
	out := []Process{}
	entries, e := os.ReadDir(root)
	if e != nil {
		return out
	}
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || pid <= 0 {
			continue
		}
		base := filepath.Join(root, entry.Name())
		commBytes, e := os.ReadFile(filepath.Join(base, "comm"))
		if e != nil {
			continue
		}
		comm := strings.TrimSpace(string(commBytes))
		if !permitted[comm] {
			continue
		}
		statBytes, e := os.ReadFile(filepath.Join(base, "stat"))
		if e != nil {
			continue
		}
		text := string(statBytes)
		end := strings.LastIndex(text, ")")
		if end < 0 {
			continue
		}
		fields := strings.Fields(text[end+1:])
		if len(fields) < 20 {
			continue
		}
		ppid, e := strconv.Atoi(fields[1])
		if e != nil {
			continue
		}
		start, e := strconv.ParseUint(fields[19], 10, 64)
		if e != nil {
			continue
		}
		info, e := os.Stat(base)
		if e != nil {
			continue
		}
		sys, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		cwd, e := os.Readlink(filepath.Join(base, "cwd"))
		if e != nil || !filepath.IsAbs(cwd) {
			continue
		}
		out = append(out, Process{PID: pid, PPID: ppid, UID: sys.Uid, StartTime: start, CWD: cwd, Comm: comm})
	}
	return out
}
