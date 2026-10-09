package probe

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func Ports(proc string) []model.Result {
	out := []model.Result{}
	for _, spec := range []struct{ file, proto, family string }{{"tcp", "tcp", "ipv4"}, {"tcp6", "tcp", "ipv6"}, {"udp", "udp", "ipv4"}, {"udp6", "udp", "ipv6"}} {
		b, e := os.ReadFile(filepath.Join(proc, "net", spec.file))
		if e != nil {
			out = append(out, unknown("ports:"+spec.file, e))
			continue
		}
		count := 0
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 4 || f[0] == "sl" {
				continue
			}
			if spec.proto == "tcp" && f[3] != "0A" {
				continue
			}
			parts := strings.Split(f[1], ":")
			if len(parts) != 2 {
				continue
			}
			port, e := strconv.ParseUint(parts[1], 16, 16)
			if e != nil {
				continue
			}
			address := "specific"
			raw := parts[0]
			if strings.Trim(raw, "0") == "" {
				address = "any"
			} else if raw == "0100007F" || raw == "00000000000000000000000001000000" {
				address = "loopback"
			}
			name := fmt.Sprintf("port:%s:%s:%d:%s", spec.proto, spec.family, port, address)
			out = append(out, result(name, model.OK, "listening "+spec.proto+" port", map[string]float64{"port": float64(port)}))
			count++
		}
		out = append(out, result("ports:"+spec.file, model.OK, "port table read", map[string]float64{"count": float64(count)}))
	}
	return out
}
