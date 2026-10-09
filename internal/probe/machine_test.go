package probe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func fixture(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, name)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestMachineFixtures(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")
	fixture(t, proc, "stat", "cpu 10 0 10 80 0 0 0 0\n")
	fixture(t, proc, "loadavg", "1.00 2.00 3.00 1/10 1\n")
	fixture(t, proc, "meminfo", "MemTotal: 100 kB\nMemAvailable: 40 kB\nSwapTotal: 20 kB\nSwapFree: 5 kB\n")
	fixture(t, proc, "net/dev", "Inter-| Receive | Transmit\n eth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n")
	fixture(t, sys, "class/hwmon/hwmon0/name", "coretemp\n")
	fixture(t, sys, "class/hwmon/hwmon0/temp1_input", "85000\n")
	fixture(t, sys, "class/hwmon/hwmon0/temp1_label", "Package id 0\n")
	fixture(t, sys, "class/hwmon/hwmon0/temp1_max", "80000\n")
	fixture(t, sys, "class/hwmon/hwmon0/temp1_crit", "100000\n")
	fixture(t, sys, "class/hwmon/hwmon0/fan1_input", "1200\n")
	fixture(t, sys, "class/powercap/intel-rapl:0/name", "package-0\n")
	fixture(t, sys, "class/powercap/intel-rapl:0/energy_uj", "950\n")
	fixture(t, sys, "class/powercap/intel-rapl:0/max_energy_range_uj", "1000\n")
	m := NewMachine(proc, sys)
	if m.CPU().State != model.Unknown {
		t.Fatal("first CPU sample must be unknown")
	}
	fixture(t, proc, "stat", "cpu 20 0 20 160 0 0 0 0\n")
	if v := m.CPU().Values["busy_percent"]; v != 20 {
		t.Fatalf("CPU busy = %v", v)
	}
	if v := m.Memory().Values["used_bytes"]; v != 60*1024 {
		t.Fatalf("memory used = %v", v)
	}
	if m.Load().Values["5m"] != 2 {
		t.Fatal("load")
	}
	s := m.Sensors()
	if len(s) != 2 || s[0].State != model.Warn {
		t.Fatalf("sensors: %+v", s)
	}
	if s[0].Detail != "85.0 C, max 80.0 C, crit 100.0 C" || s[1].Detail != "1200 rpm" {
		t.Fatalf("sensor detail: %+v", s)
	}
	if m.Power().State != model.Unknown {
		t.Fatal("first power sample")
	}
	time.Sleep(time.Millisecond)
	fixture(t, sys, "class/powercap/intel-rapl:0/energy_uj", "50\n")
	if v := m.Power().Values["watts"]; v <= 0 {
		t.Fatalf("wrapped power = %v", v)
	}
	if m.Network()[0].State != model.Unknown {
		t.Fatal("first network sample")
	}
	m.netAt = time.Now().Add(-time.Second)
	fixture(t, proc, "net/dev", "eth0: 200 0 0 0 0 0 0 0 400 0 0 0 0 0 0 0\n")
	if v := m.Network()[0].Values["rx_bytes_per_second"]; v <= 0 {
		t.Fatalf("network rate = %v", v)
	}
}
func TestMissingSensors(t *testing.T) {
	m := NewMachine(t.TempDir(), t.TempDir())
	if r := m.Sensors(); len(r) != 1 || r[0].State != model.Unknown {
		t.Fatalf("missing sensors: %+v", r)
	}
	if r := m.Power(); r.State != model.Unknown {
		t.Fatal(r)
	}
}

func TestSubsystemCountersFromProcAndSys(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	sys := filepath.Join(root, "sys")
	fixture(t, proc, "stat", "cpu 10 0 10 80 0 0 0 0\ncpu0 10 0 10 80 0 0 0 0\n")
	fixture(t, proc, "meminfo", "MemTotal: 100 kB\nMemAvailable: 40 kB\nCached: 12 kB\nBuffers: 4 kB\nShmem: 3 kB\nSwapTotal: 20 kB\nSwapFree: 5 kB\n")
	fixture(t, proc, "pressure/memory", "some avg10=1.25 avg60=0.50 avg300=0.10 total=123\n")
	fixture(t, sys, "block/zram0/mm_stat", "8192 4096 2048 0 0\n")
	fixture(t, sys, "class/block/nvme0n1/device/model", "fixture\n")
	fixture(t, proc, "diskstats", "259 0 nvme0n1 0 0 100 0 0 0 200 0 0 0 0 0 0 0 0\n")
	fixture(t, sys, "class/drm/card0/device/gpu_busy_percent", "27\n")
	m := NewMachine(proc, sys)
	m.CPU()
	fixture(t, proc, "stat", "cpu 20 0 20 160 0 0 0 0\ncpu0 20 0 20 160 0 0 0 0\n")
	cpu := m.CPU()
	if cpu.Values["user_percent"] != 10 || cpu.Values["system_percent"] != 10 || cpu.Values["core_0_percent"] != 20 || cpu.Values["core_count"] != 1 {
		t.Fatalf("CPU details: %+v", cpu)
	}
	mem := m.Memory()
	if mem.Values["page_cache_bytes"] != 12*1024 || mem.Values["pressure_some_avg10"] != 1.25 || mem.Values["zram_memory_bytes"] != 2048 {
		t.Fatalf("memory details: %+v", mem)
	}
	if first := m.DriveIO(); first[0].State != model.Unknown {
		t.Fatalf("first drive: %+v", first)
	}
	m.diskAt = time.Now().Add(-time.Second)
	fixture(t, proc, "diskstats", "259 0 nvme0n1 0 0 110 0 0 0 220 0 0 0 0 0 0 0 0\n")
	drive := m.DriveIO()
	if drive[0].Values["write_bytes_per_second"] <= 0 || drive[0].Values["written_today_bytes"] != 20*512 {
		t.Fatalf("drive details: %+v", drive)
	}
	gpu := m.GPU()
	if len(gpu) != 1 || gpu[0].Values["busy_percent"] != 27 {
		t.Fatalf("GPU: %+v", gpu)
	}
}
func TestPortsFixture(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, "net/tcp", "  sl  local_address rem_address st\n   0: 0100007F:1F90 00000000:0000 0A 0 0\n")
	for _, f := range []string{"tcp6", "udp", "udp6"} {
		fixture(t, root, "net/"+f, "sl local_address rem_address st\n")
	}
	r := Ports(root)
	if len(r) != 5 || !strings.Contains(r[0].Name, "8080:loopback") || r[1].Name != "ports:tcp" || r[1].Values["count"] != 1 {
		t.Fatalf("ports: %+v", r)
	}
}
