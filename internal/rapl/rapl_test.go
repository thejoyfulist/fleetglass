package rapl

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func put(t *testing.T, root, dir, name string, energy, max uint64) {
	t.Helper()
	path := filepath.Join(root, "class/powercap", dir)
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	for file, data := range map[string]string{"name": name, "energy_uj": strconv.FormatUint(energy, 10), "max_energy_range_uj": strconv.FormatUint(max, 10)} {
		if err := os.WriteFile(filepath.Join(path, file), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestDellDomainsAndWrap(t *testing.T) {
	root := t.TempDir()
	put(t, root, "intel-rapl:0", "package-0", 900000, 1000000)
	put(t, root, "intel-rapl:0:0", "core", 300000, 1000000)
	put(t, root, "intel-rapl:0:1", "uncore", 400000, 1000000)
	put(t, root, "intel-rapl:1", "psys", 500000, 1000000)
	put(t, root, "intel-rapl-mmio:0", "package-0", 600000, 1000000)
	put(t, root, "intel-rapl:2", "package-1", 200000, 1000000)
	first, err := Read(root)
	if err != nil || len(first.Domains) != 2 {
		t.Fatalf("sample: %+v, %v", first, err)
	}
	tracker := Tracker{}
	at := time.Now()
	if _, ok := tracker.Power(first, at); ok {
		t.Fatal("first sample has no delta")
	}
	put(t, root, "intel-rapl:0", "package-0", 100000, 1000000)
	put(t, root, "intel-rapl:2", "package-1", 400000, 1000000)
	second, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	watts, ok := tracker.Power(second, at.Add(time.Second))
	if !ok || watts["intel-rapl:0"] != 0.2 || watts["intel-rapl:2"] != 0.2 {
		t.Fatalf("power: %v %v", watts, ok)
	}
}
