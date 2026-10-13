package netguard

import (
	"net"
	"strings"
)

// LoopbackHost accepts only literal loopback addresses and localhost as HTTP Host.
// A hostname resolving to loopback is insufficient because its DNS can change.
func LoopbackHost(hostport string) bool {
	host := hostport
	if strings.Contains(hostport, ":") {
		var err error
		host, _, err = net.SplitHostPort(hostport)
		if err != nil {
			return false
		}
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
