package probe

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/rapl"
)

type Machine struct {
	Proc, Sys         string
	cpuIdle, cpuTotal uint64
	cpuFields         map[string][]uint64
	powerTracker      rapl.Tracker
	netAt             time.Time
	netCounters       map[string][2]float64
	diskAt            time.Time
	diskCounters      map[string][2]float64
	diskToday         map[string]float64
	diskDay           string
}

func NewMachine(proc, sys string) *Machine { return &Machine{Proc: proc, Sys: sys} }
func result(name string, state model.State, detail string, v map[string]float64) model.Result {
	return model.Result{Name: name, State: state, Detail: detail, Values: v}
}
func unknown(name string, err error) model.Result {
	return result(name, model.Unknown, err.Error(), nil)
}
func readUint(path string) (uint64, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return 0, e
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

func (m *Machine) Fast(ctx context.Context) []model.Result {
	return []model.Result{m.Host(), m.CPU(), m.Load(), m.Memory()}
}
func (m *Machine) Host() model.Result {
	fields := map[string]string{"arch": runtime.GOARCH}
	values := map[string]float64{}
	if b, err := os.ReadFile(filepath.Join(m.Proc, "uptime")); err == nil {
		if parts := strings.Fields(string(b)); len(parts) > 0 {
			if seconds, err := strconv.ParseFloat(parts[0], 64); err == nil && seconds >= 0 {
				values["uptime_seconds"] = seconds
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(m.Proc, "sys/kernel/osrelease")); err == nil {
		fields["kernel_release"] = strings.TrimSpace(string(b))
	}
	if len(values) == 0 && fields["kernel_release"] == "" {
		return result("host", model.Unknown, "host facts unavailable", nil)
	}
	return model.Result{Name: "host", State: model.OK, Detail: "host facts", Values: values, Fields: fields}
}
func (m *Machine) Frequency() map[string]float64 {
	dirs, _ := filepath.Glob(filepath.Join(m.Sys, "devices/system/cpu/cpu[0-9]*/cpufreq"))
	out := map[string]float64{}
	for _, metric := range []struct{ file, key string }{{"scaling_cur_freq", "frequency_current_mhz"}, {"cpuinfo_max_freq", "frequency_max_mhz"}} {
		var sum float64
		var count int
		for _, dir := range dirs {
			if value, err := readUint(filepath.Join(dir, metric.file)); err == nil && value > 0 {
				sum += float64(value) / 1000
				count++
			}
		}
		if count > 0 {
			out[metric.key] = sum / float64(count)
		}
	}
	return out
}
func (m *Machine) CPU() model.Result {
	f, e := os.Open(filepath.Join(m.Proc, "stat"))
	if e != nil {
		return unknown("cpu", e)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return unknown("cpu", fmt.Errorf("missing CPU data"))
	}
	a := strings.Fields(s.Text())
	if len(a) < 5 || a[0] != "cpu" {
		return unknown("cpu", fmt.Errorf("invalid CPU data"))
	}
	var total, idle uint64
	fields := map[string][]uint64{}
	all := []uint64{}
	for i, x := range a[1:] {
		n, e := strconv.ParseUint(x, 10, 64)
		if e != nil {
			return unknown("cpu", e)
		}
		total += n
		all = append(all, n)
		if i == 3 || i == 4 {
			idle += n
		}
	}
	fields["cpu"] = all
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 5 || !strings.HasPrefix(parts[0], "cpu") || parts[0] == "cpu" {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(parts[0], "cpu")); err != nil {
			continue
		}
		values := []uint64{}
		for _, part := range parts[1:] {
			n, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				break
			}
			values = append(values, n)
		}
		if len(values) >= 5 {
			fields[parts[0]] = values
		}
	}
	previousFields := m.cpuFields
	m.cpuFields = fields
	prevTotal, prevIdle := m.cpuTotal, m.cpuIdle
	m.cpuTotal, m.cpuIdle = total, idle
	if prevTotal == 0 || total <= prevTotal || idle < prevIdle {
		return result("cpu", model.Unknown, "waiting for CPU delta", m.Frequency())
	}
	pct := 100 * float64(total-prevTotal-(idle-prevIdle)) / float64(total-prevTotal)
	state := model.OK
	if pct >= 90 {
		state = model.Warn
	}
	if pct >= 98 {
		state = model.Crit
	}
	cores := len(fields) - 1
	if cores < 1 {
		cores = runtime.NumCPU()
	}
	v := map[string]float64{"busy_percent": pct, "core_count": float64(cores)}
	for key, frequency := range m.Frequency() {
		v[key] = frequency
	}
	if old := previousFields["cpu"]; len(old) >= 8 && len(all) >= 8 {
		for _, metric := range []struct {
			key     string
			indexes []int
		}{{"user_percent", []int{0, 1}}, {"system_percent", []int{2, 5, 6}}, {"iowait_percent", []int{4}}, {"steal_percent", []int{7}}} {
			var delta uint64
			for _, i := range metric.indexes {
				if all[i] >= old[i] {
					delta += all[i] - old[i]
				}
			}
			v[metric.key] = 100 * float64(delta) / float64(total-prevTotal)
		}
	}
	for key, current := range fields {
		if key == "cpu" {
			continue
		}
		old := previousFields[key]
		if len(old) < 5 {
			continue
		}
		var sum, rest uint64
		for i, n := range current {
			if i >= len(old) || n < old[i] {
				sum = 0
				break
			}
			d := n - old[i]
			sum += d
			if i == 3 || i == 4 {
				rest += d
			}
		}
		if sum > 0 {
			v["core_"+strings.TrimPrefix(key, "cpu")+"_percent"] = 100 * float64(sum-rest) / float64(sum)
		}
	}
	return result("cpu", state, fmt.Sprintf("CPU busy %.1f%%", pct), v)
}
func (m *Machine) Load() model.Result {
	b, e := os.ReadFile(filepath.Join(m.Proc, "loadavg"))
	if e != nil {
		return unknown("load", e)
	}
	a := strings.Fields(string(b))
	if len(a) < 3 {
		return unknown("load", fmt.Errorf("invalid load average"))
	}
	v := map[string]float64{}
	for i, k := range []string{"1m", "5m", "15m"} {
		n, e := strconv.ParseFloat(a[i], 64)
		if e != nil {
			return unknown("load", e)
		}
		v[k] = n
	}
	return result("load", model.OK, "system load averages", v)
}
func (m *Machine) Memory() model.Result {
	b, e := os.ReadFile(filepath.Join(m.Proc, "meminfo"))
	if e != nil {
		return unknown("memory", e)
	}
	data := map[string]float64{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			n, e := strconv.ParseFloat(f[1], 64)
			if e == nil {
				data[strings.TrimSuffix(f[0], ":")] = n * 1024
			}
		}
	}
	total, ok := data["MemTotal"]
	if !ok || total == 0 {
		return unknown("memory", fmt.Errorf("missing MemTotal"))
	}
	avail := data["MemAvailable"]
	if _, ok := data["MemAvailable"]; !ok {
		return unknown("memory", fmt.Errorf("missing MemAvailable"))
	}
	sw := data["SwapTotal"]
	free := data["SwapFree"]
	v := map[string]float64{"total_bytes": total, "used_bytes": total - avail, "swap_total_bytes": sw, "swap_used_bytes": sw - free,
		"page_cache_bytes": data["Cached"], "buffers_bytes": data["Buffers"], "shared_bytes": data["Shmem"]}
	if pressure, err := os.ReadFile(filepath.Join(m.Proc, "pressure/memory")); err == nil {
		for _, line := range strings.Split(string(pressure), "\n") {
			f := strings.Fields(line)
			if len(f) == 0 || f[0] != "some" {
				continue
			}
			for _, part := range f[1:] {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}
				n, e := strconv.ParseFloat(kv[1], 64)
				if e == nil {
					v["pressure_some_"+kv[0]] = n
				}
			}
		}
	}
	if dirs, err := filepath.Glob(filepath.Join(m.Sys, "block/zram*/mm_stat")); err == nil {
		for _, path := range dirs {
			if b, e := os.ReadFile(path); e == nil {
				f := strings.Fields(string(b))
				if len(f) >= 3 {
					original, _ := strconv.ParseFloat(f[0], 64)
					compressed, _ := strconv.ParseFloat(f[2], 64)
					v["zram_data_bytes"] += original
					v["zram_memory_bytes"] += compressed
				}
			}
		}
	}
	state := model.OK
	if (total-avail)/total >= 0.9 {
		state = model.Warn
	}
	if (total-avail)/total >= 0.98 {
		state = model.Crit
	}
	return result("memory", state, "memory and swap usage", v)
}
func (m *Machine) Disks() []model.Result {
	b, e := os.ReadFile(filepath.Join(m.Proc, "mounts"))
	if e != nil {
		return []model.Result{unknown("disks", e)}
	}
	out := []model.Result{}
	skip := map[string]bool{"tmpfs": true, "devtmpfs": true, "overlay": true, "squashfs": true, "proc": true, "sysfs": true, "cgroup": true, "cgroup2": true, "devpts": true, "mqueue": true, "securityfs": true, "tracefs": true, "debugfs": true, "pstore": true, "fusectl": true, "configfs": true, "efivarfs": true, "ramfs": true, "autofs": true, "hugetlbfs": true, "bpf": true}
	seen := map[string]bool{}
	seenDevice := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		a := strings.Fields(line)
		if len(a) < 3 || skip[a[2]] || seen[a[1]] || seenDevice[a[0]] || !strings.HasPrefix(a[0], "/dev/") {
			continue
		}
		seen[a[1]] = true
		seenDevice[a[0]] = true
		var st unixStatfs
		if e := statfs(a[1], &st); e != nil {
			out = append(out, unknown("disk:"+a[1], e))
			continue
		}
		state := model.OK
		if st.Blocks > 0 && float64(st.Blocks-st.Bfree)/float64(st.Blocks) >= 0.85 {
			state = model.Warn
		}
		if st.Blocks > 0 && float64(st.Blocks-st.Bfree)/float64(st.Blocks) >= 0.95 {
			state = model.Crit
		}
		out = append(out, result("disk:"+a[1], state, "filesystem usage", map[string]float64{"total_bytes": float64(st.Blocks) * float64(st.Bsize), "used_bytes": float64(st.Blocks-st.Bfree) * float64(st.Bsize), "inodes_total": float64(st.Files), "inodes_used": float64(st.Files - st.Ffree)}))
	}
	if len(out) == 0 {
		return []model.Result{unknown("disks", fmt.Errorf("no readable real filesystems"))}
	}
	return out
}
func (m *Machine) Network() []model.Result {
	b, e := os.ReadFile(filepath.Join(m.Proc, "net/dev"))
	if e != nil {
		return []model.Result{unknown("network", e)}
	}
	now := time.Now()
	current := map[string][2]float64{}
	out := []model.Result{}
	for _, line := range strings.Split(string(b), "\n") {
		a := strings.SplitN(line, ":", 2)
		if len(a) != 2 {
			continue
		}
		name := strings.TrimSpace(a[0])
		f := strings.Fields(a[1])
		if len(f) < 16 {
			continue
		}
		rx, e1 := strconv.ParseFloat(f[0], 64)
		tx, e2 := strconv.ParseFloat(f[8], 64)
		if e1 != nil || e2 != nil {
			continue
		}
		current[name] = [2]float64{rx, tx}
		v := map[string]float64{"rx_bytes": rx, "tx_bytes": tx}
		state := model.Unknown
		detail := "waiting for network delta"
		if prev, ok := m.netCounters[name]; ok && rx >= prev[0] && tx >= prev[1] && now.After(m.netAt) {
			secs := now.Sub(m.netAt).Seconds()
			v["rx_bytes_per_second"] = (rx - prev[0]) / secs
			v["tx_bytes_per_second"] = (tx - prev[1]) / secs
			state = model.OK
			detail = "interface throughput"
		}
		out = append(out, result("network:"+name, state, detail, v))
	}
	m.netCounters = current
	m.netAt = now
	if len(out) == 0 {
		return []model.Result{unknown("network", fmt.Errorf("no readable interfaces"))}
	}
	return out
}

// DriveIO uses the kernel's 512-byte diskstats sectors; device identifiers
// are kernel names, never serial numbers or filesystem labels.
func (m *Machine) DriveIO() []model.Result {
	b, err := os.ReadFile(filepath.Join(m.Proc, "diskstats"))
	if err != nil {
		return []model.Result{unknown("drive_io", err)}
	}
	now := time.Now()
	day := now.UTC().Format("2006-01-02")
	if day != m.diskDay {
		m.diskToday = map[string]float64{}
		m.diskDay = day
	}
	current := map[string][2]float64{}
	out := []model.Result{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 10 {
			continue
		}
		name := f[2]
		if _, err := os.Stat(filepath.Join(m.Sys, "class/block", name, "partition")); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(m.Sys, "class/block", name)); err != nil {
			continue
		}
		read, e1 := strconv.ParseFloat(f[5], 64)
		write, e2 := strconv.ParseFloat(f[9], 64)
		if e1 != nil || e2 != nil {
			continue
		}
		read *= 512
		write *= 512
		current[name] = [2]float64{read, write}
		v := map[string]float64{"read_bytes": read, "write_bytes": write, "written_today_bytes": m.diskToday[name]}
		state := model.Unknown
		detail := "waiting for drive delta"
		if prev, ok := m.diskCounters[name]; ok && read >= prev[0] && write >= prev[1] && now.After(m.diskAt) {
			seconds := now.Sub(m.diskAt).Seconds()
			v["read_bytes_per_second"] = (read - prev[0]) / seconds
			v["write_bytes_per_second"] = (write - prev[1]) / seconds
			m.diskToday[name] += write - prev[1]
			v["written_today_bytes"] = m.diskToday[name]
			state = model.OK
			detail = "drive activity"
		}
		out = append(out, result("drive:"+name, state, detail, v))
	}
	m.diskCounters = current
	m.diskAt = now
	if len(out) == 0 {
		return []model.Result{unknown("drive_io", fmt.Errorf("no readable drives"))}
	}
	return out
}

func (m *Machine) GPU() []model.Result {
	paths, _ := filepath.Glob(filepath.Join(m.Sys, "class/drm/card[0-9]*/device"))
	out := []model.Result{}
	for _, path := range paths {
		name := filepath.Base(filepath.Dir(path))
		v := map[string]float64{}
		state := model.Unknown
		if b, err := os.ReadFile(filepath.Join(path, "gpu_busy_percent")); err == nil {
			if pct, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64); err == nil && pct >= 0 && pct <= 100 {
				v["busy_percent"] = pct
				state = model.OK
			}
		}
		out = append(out, result("gpu:"+name, state, "GPU utilization where exposed by the driver", v))
	}
	return out
}
func (m *Machine) Sensors() []model.Result {
	dirs, e := filepath.Glob(filepath.Join(m.Sys, "class/hwmon/hwmon*"))
	if e != nil {
		return []model.Result{unknown("sensors", e)}
	}
	out := []model.Result{}
	for _, dir := range dirs {
		chip, _ := os.ReadFile(filepath.Join(dir, "name"))
		chipName := strings.TrimSpace(string(chip))
		for _, kind := range []string{"temp", "fan"} {
			files, _ := filepath.Glob(filepath.Join(dir, kind+"*_input"))
			for _, file := range files {
				base := strings.TrimSuffix(file, "_input")
				n, e := readUint(file)
				if e != nil {
					out = append(out, unknown("sensor:"+filepath.Base(base), e))
					continue
				}
				lab, _ := os.ReadFile(base + "_label")
				label := strings.TrimSpace(string(lab))
				if label == "" {
					label = filepath.Base(base)
				}
				name := "sensor:" + chipName + ":" + label
				v := map[string]float64{}
				state := model.OK
				detail := ""
				if kind == "temp" {
					v["celsius"] = float64(n) / 1000
					detail = fmt.Sprintf("%.1f C", v["celsius"])
					for _, threshold := range []struct {
						suffix string
						key    string
						level  model.State
					}{{"_max", "max_celsius", model.Warn}, {"_crit", "crit_celsius", model.Crit}} {
						if t, e := readUint(base + threshold.suffix); e == nil && t > 0 && t < 200000 {
							v[threshold.key] = float64(t) / 1000
							detail += fmt.Sprintf(", %s %.1f C", strings.TrimPrefix(threshold.suffix, "_"), v[threshold.key])
							if n >= t {
								state = threshold.level
							}
						}
					}
				} else {
					v["rpm"] = float64(n)
					detail = fmt.Sprintf("%d rpm", n)
					if n == 0 {
						state = model.Warn
					}
				}
				out = append(out, result(name, state, detail, v))
			}
		}
	}
	if len(out) == 0 {
		return []model.Result{unknown("sensors", fmt.Errorf("no readable hwmon sensors"))}
	}
	return out
}
func (m *Machine) Power() model.Result {
	sample, err := rapl.Read(m.Sys)
	if err != nil {
		return unknown("cpu_package_power", err)
	}
	return m.PowerSample(sample)
}

func (m *Machine) PowerSample(sample rapl.Sample) model.Result {
	watts, energyKWh, seconds, ok := m.powerTracker.Measure(sample, time.Now())
	if !ok {
		return result("cpu_package_power", model.Unknown, "waiting for RAPL delta", nil)
	}
	values := map[string]float64{}
	for id, w := range watts {
		values["watts:"+id] = w
		values["watts"] += w
	}
	values["energy_kwh"] = energyKWh
	values["energy_seconds"] = seconds
	return result("cpu_package_power", model.OK, "CPU package power", values)
}
