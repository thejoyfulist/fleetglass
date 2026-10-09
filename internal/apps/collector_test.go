package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func put(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}
func fakeStat(pid, ppid, utime, rss int) string {
	f := []any{"S", ppid, 0, 0, 0, 0, 0, 0, 0, 0, 0, utime, 0, 0, 0, 0, 0, 0, 0, 0, 0, rss}
	return fmt.Sprintf("%d (fixture) %s\n", pid, fmt.Sprint(f)[1:len(fmt.Sprint(f))-1])
}
func TestCgroupAppAccountingAndOther(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")
	put(t, filepath.Join(proc, "100/stat"), fakeStat(100, 1, 10, 2))
	put(t, filepath.Join(proc, "100/cgroup"), "0::/system.slice/example.service\n")
	put(t, filepath.Join(proc, "100/io"), "read_bytes: 1000\nwrite_bytes: 500\n")
	put(t, filepath.Join(proc, "200/stat"), fakeStat(200, 1, 20, 1))
	put(t, filepath.Join(proc, "200/cgroup"), "0::/\n")
	cg := filepath.Join(sys, "fs/cgroup/system.slice/example.service")
	put(t, filepath.Join(cg, "cpu.stat"), "usage_usec 100000\n")
	put(t, filepath.Join(cg, "memory.current"), "4096\n")
	put(t, filepath.Join(cg, "memory.stat"), "file 1024\n")
	put(t, filepath.Join(cg, "io.stat"), "8:0 rbytes=1000 wbytes=500\n")
	var collector Collector
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	collector.Measure(proc, sys, nil, nil, 20, at)
	put(t, filepath.Join(cg, "cpu.stat"), "usage_usec 200000\n")
	put(t, filepath.Join(cg, "io.stat"), "8:0 rbytes=3000 wbytes=1500\n")
	put(t, filepath.Join(proc, "200/stat"), fakeStat(200, 1, 25, 1))
	got := collector.Measure(proc, sys, nil, nil, 20, at.Add(time.Second))
	if len(got) != 2 {
		t.Fatalf("apps: %+v", got)
	}
	energy := float64(0)
	for _, app := range got {
		if app.EnergyWatts != nil {
			energy += *app.EnergyWatts
		}
		if app.Kind == "unit" {
			if len(app.Processes) != 1 || app.Processes[0].Name != "fixture" || app.Processes[0].PID != 100 || app.Name != "example.service" || app.CPUPercent != 10 || app.MemoryBytes != 4096 || app.PageCacheBytes != 1024 || app.DiskReadRate != 2000 || app.DiskWriteBytes != 1000 || app.EnergyWatts == nil {
				t.Fatalf("unit: %+v", app)
			}
		} else if app.Kind != "other" || app.Name != "Other / not attributed" {
			t.Fatalf("other: %+v", app)
		}
	}
	if energy < 19.999 || energy > 20.001 {
		t.Fatalf("estimated package energy shares: %v", energy)
	}
}

func TestContainerNetworkNamespaceCounters(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path := "/system.slice/docker-" + id + ".scope"
	put(t, filepath.Join(proc, "300/stat"), fakeStat(300, 1, 10, 1))
	put(t, filepath.Join(proc, "300/cgroup"), "0::"+path+"\n")
	put(t, filepath.Join(proc, "300/net/dev"), "eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0\n")
	if err := os.MkdirAll(filepath.Join(proc, "1/ns"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proc, "300/ns"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("net:[1]", filepath.Join(proc, "1/ns/net")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("net:[2]", filepath.Join(proc, "300/ns/net")); err != nil {
		t.Fatal(err)
	}
	cg := filepath.Join(sys, "fs/cgroup", path)
	put(t, filepath.Join(cg, "cpu.stat"), "usage_usec 1000\n")
	put(t, filepath.Join(cg, "memory.current"), "1024\n")
	var collector Collector
	at := time.Now()
	collector.Measure(proc, sys, nil, nil, 0, at)
	put(t, filepath.Join(proc, "300/net/dev"), "eth0: 1500 0 0 0 0 0 0 0 2600 0 0 0 0 0 0 0\n")
	got := collector.Measure(proc, sys, nil, nil, 0, at.Add(time.Second))
	if len(got) != 1 || got[0].Kind != "container" || got[0].NetworkRXRate == nil || *got[0].NetworkRXRate != 500 || *got[0].NetworkTXRate != 600 {
		t.Fatalf("container network: %+v", got)
	}
}
