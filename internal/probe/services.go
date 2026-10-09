package probe

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/dockerapi"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

type Runner func(context.Context, string, ...string) ([]byte, error)

func Exec(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type Services struct {
	Units []string
	Run   Runner
}
type failedUnit struct {
	Unit   string `json:"unit"`
	Active string `json:"active"`
	Sub    string `json:"sub"`
}

func (s Services) Check(ctx context.Context) []model.Result {
	run := s.Run
	if run == nil {
		run = Exec
	}
	out := []model.Result{}
	b, e := run(ctx, "systemctl", "list-units", "--state=failed", "--output=json", "--no-pager")
	if e != nil {
		out = append(out, unknown("systemd:failed", e))
	} else {
		var units []failedUnit
		if e := json.Unmarshal(b, &units); e != nil {
			out = append(out, unknown("systemd:failed", e))
		} else {
			state := model.OK
			if len(units) > 0 {
				state = model.Crit
			}
			out = append(out, result("systemd:failed", state, fmt.Sprintf("%d failed units", len(units)), map[string]float64{"count": float64(len(units))}))
			for _, u := range units {
				out = append(out, result("systemd:"+u.Unit, model.Crit, "failed unit", nil))
			}
		}
	}
	for _, unit := range s.Units {
		b, e := run(ctx, "systemctl", "show", unit, "--property=ActiveState,SubState", "--no-pager")
		if e != nil {
			out = append(out, unknown("systemd:"+unit, e))
			continue
		}
		state := model.Crit
		active, sub := "unknown", "unknown"
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(line, "ActiveState="); ok {
				active = strings.TrimSpace(v)
			}
			if v, ok := strings.CutPrefix(line, "SubState="); ok {
				sub = strings.TrimSpace(v)
			}
		}
		if active == "active" {
			state = model.OK
		}
		out = append(out, result("systemd:"+unit, state, active+" ("+sub+")", nil))
	}
	return out
}

type Docker struct {
	Sockets      []string
	HelperSocket string
	Dial         func(context.Context, string) (net.Conn, error)
}
type container struct {
	ID           string           `json:"Id"`
	Names        []string         `json:"Names"`
	State        string           `json:"State"`
	Status       string           `json:"Status"`
	RestartCount int              `json:"RestartCount"`
	ImageID      string           `json:"ImageID"`
	Health       dockerapi.Health `json:"Health"`
}
type dockerSummary struct {
	Name         string `json:"name"`
	State        string `json:"state"`
	Health       string `json:"health"`
	RestartCount int    `json:"restart_count"`
	ImageHash    string `json:"image_hash"`
}

func imageHash(id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:8])
}

func (d Docker) Check(ctx context.Context) []model.Result {
	out := []model.Result{}
	for i, socket := range d.Sockets {
		transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
			if d.Dial != nil {
				return d.Dial(c, socket)
			}
			return (&net.Dialer{}).DialContext(c, "unix", socket)
		}}
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://docker/containers/json?all=1", nil)
		resp, e := client.Do(req)
		if e != nil {
			if errors.Is(e, os.ErrPermission) && d.HelperSocket != "" {
				out = append(out, d.helperResults(ctx, i)...)
			} else {
				out = append(out, unknown("docker:"+strconv.Itoa(i), e))
			}
			transport.CloseIdleConnections()
			continue
		}
		var containers []container
		if resp.StatusCode != 200 {
			e = fmt.Errorf("Docker HTTP %d", resp.StatusCode)
		} else {
			e = json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4<<20)).Decode(&containers)
		}
		resp.Body.Close()
		transport.CloseIdleConnections()
		if e != nil {
			out = append(out, unknown("docker:"+strconv.Itoa(i), e))
			continue
		}
		out = append(out, result("docker:"+strconv.Itoa(i), model.OK, fmt.Sprintf("%d containers", len(containers)), map[string]float64{"count": float64(len(containers))}))
		for _, c := range containers {
			name := c.ID
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			state := model.OK
			if c.State != "running" {
				state = model.Warn
			}
			if strings.Contains(c.Status, "unhealthy") || c.Health == "unhealthy" {
				state = model.Crit
			}
			r := result("container:"+strconv.Itoa(i)+":"+name, state, c.State+"; "+c.Status, map[string]float64{"restart_count": float64(c.RestartCount)})
			r.Fields = map[string]string{"runtime_state": c.State, "health": string(c.Health)}
			if len(c.ID) >= 12 {
				r.Fields["container_id"] = c.ID[:12]
			}
			if hash := imageHash(c.ImageID); hash != "" {
				if r.Fields == nil {
					r.Fields = map[string]string{}
				}
				r.Fields["image_hash"] = hash
			}
			out = append(out, r)
		}
	}
	return out
}

func (d Docker) helperResults(ctx context.Context, i int) []model.Result {
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", d.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://helper/docker/"+strconv.Itoa(i), nil)
	resp, err := client.Do(req)
	if err != nil {
		return []model.Result{result("docker:"+strconv.Itoa(i), model.Unknown, "Docker helper unavailable", nil)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusServiceUnavailable {
			return []model.Result{result("docker:"+strconv.Itoa(i), model.Unknown, "Docker daemon not reachable", nil)}
		}
		return []model.Result{result("docker:"+strconv.Itoa(i), model.Unknown, "invalid Docker helper response", nil)}
	}
	var containers []dockerSummary
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4<<20)).Decode(&containers); err != nil {
		return []model.Result{result("docker:"+strconv.Itoa(i), model.Unknown, "invalid Docker helper response", nil)}
	}
	out := []model.Result{result("docker:"+strconv.Itoa(i), model.OK, fmt.Sprintf("%d containers", len(containers)), map[string]float64{"count": float64(len(containers))})}
	for _, c := range containers {
		state := model.OK
		if c.State != "running" {
			state = model.Warn
		}
		if c.Health == "unhealthy" {
			state = model.Crit
		}
		detail := c.State
		if c.Health != "" {
			detail += "; " + c.Health
		}
		r := result("container:"+strconv.Itoa(i)+":"+c.Name, state, detail, map[string]float64{"restart_count": float64(c.RestartCount)})
		r.Fields = map[string]string{"runtime_state": c.State, "health": c.Health}
		if c.ImageHash != "" {
			r.Fields["image_hash"] = c.ImageHash
		}
		out = append(out, r)
	}
	return out
}

func TLS(ctx context.Context, domains []string) []model.Result {
	out := []model.Result{}
	for _, domain := range domains {
		host := domain
		if h, _, e := net.SplitHostPort(domain); e == nil {
			host = h
		} else {
			domain = net.JoinHostPort(domain, "443")
		}
		d := &net.Dialer{Timeout: 5 * time.Second}
		// Inspect even expired certificates so expiry is reported as critical.
		conn, e := tls.DialWithDialer(d, "tcp", domain, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: true})
		if e != nil {
			out = append(out, unknown("tls:"+host, e))
			continue
		}
		cert := conn.ConnectionState().PeerCertificates[0]
		conn.Close()
		days := time.Until(cert.NotAfter).Hours() / 24
		state := model.OK
		if days < 30 {
			state = model.Warn
		}
		if days < 7 {
			state = model.Crit
		}
		if cert.VerifyHostname(host) != nil {
			state = model.Crit
		}
		out = append(out, result("tls:"+host, state, fmt.Sprintf("expires in %.1f days", days), map[string]float64{"days_remaining": days}))
	}
	return out
}
