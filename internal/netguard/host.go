package netguard

import (
	"net"
	"regexp"
	"strconv"
	"strings"
)

var hostName = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// LoopbackHost accepts only literal loopback addresses and localhost as HTTP Host.
// A hostname resolving to loopback is insufficient because its DNS can change.
func LoopbackHost(hostport string) bool {
	if !ValidAllowedHost(hostport) {
		return false
	}
	host := hostport
	if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(hostport, "["), "]")
	}
	if strings.Contains(hostport, ":") {
		var err error
		if host == hostport && net.ParseIP(hostport) == nil {
			host, _, err = net.SplitHostPort(hostport)
			if err != nil {
				return false
			}
		}
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ValidAllowedHost accepts a literal IP or DNS name, optionally with a port.
func ValidAllowedHost(entry string) bool {
	host := entry
	if strings.HasPrefix(entry, "[") && strings.HasSuffix(entry, "]") {
		return net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(entry, "["), "]")) != nil
	}
	if strings.Contains(entry, ":") && net.ParseIP(entry) == nil {
		var port string
		var err error
		host, port, err = net.SplitHostPort(entry)
		if err != nil {
			return false
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return net.ParseIP(host) != nil || hostName.MatchString(host)
}

// AllowedHost matches an exact configured Host or a configured bare name on any port.
func AllowedHost(hostport string, entries []string) bool {
	if !ValidAllowedHost(hostport) {
		return false
	}
	if LoopbackHost(hostport) {
		return true
	}
	host := hostport
	if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(hostport, "["), "]")
	}
	if strings.Contains(hostport, ":") {
		var err error
		if host == hostport && net.ParseIP(hostport) == nil {
			host, _, err = net.SplitHostPort(hostport)
			if err != nil {
				return false
			}
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(hostport, entry) || strings.EqualFold(host, entry) {
			return true
		}
	}
	return false
}
