package probe

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

// HTTPCheck performs one bounded request. It never stores a URL or response body.
func HTTPCheck(ctx context.Context, p config.HTTPProbe, now time.Time) model.ProbeSample {
	s := model.ProbeSample{Name: p.Name, DisplayName: p.DisplayName, Public: p.Public, Kind: "http", State: model.Crit, IntervalSeconds: p.IntervalSeconds, At: now}
	timeout := time.Duration(p.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	transport := &http.Transport{DisableKeepAlives: true, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	if !p.FollowRedirects {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	} else {
		client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
			if req.URL.User != nil || req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return http.ErrUseLastResponse
			}
			return nil
		}
	}
	var dnsStart, connectStart, tlsStart, requestStart time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				s.DNSMS = ms(time.Since(dnsStart))
			}
		},
		ConnectStart: func(_, _ string) { connectStart = time.Now() },
		ConnectDone: func(_, _ string, _ error) {
			if !connectStart.IsZero() {
				s.ConnectMS = ms(time.Since(connectStart))
			}
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			if !tlsStart.IsZero() {
				s.TLSMS = ms(time.Since(tlsStart))
			}
		},
		GotFirstResponseByte: func() { s.FirstByteMS = ms(time.Since(requestStart)) },
	}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), p.Method, p.URL, nil)
	if err != nil {
		return s
	}
	req.Header.Set("User-Agent", "Fleetglass/1.0 (+HTTP availability probe)")
	requestStart = time.Now()
	resp, err := client.Do(req)
	s.LatencyMS = ms(time.Since(requestStart))
	if err != nil {
		return s
	}
	defer resp.Body.Close()
	s.StatusCode = resp.StatusCode
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		days := int(resp.TLS.PeerCertificates[0].NotAfter.Sub(now).Hours() / 24)
		s.CertificateDays = &days
	}
	expected := len(p.ExpectedStatus) == 0 && resp.StatusCode >= 200 && resp.StatusCode < 300
	for _, code := range p.ExpectedStatus {
		if resp.StatusCode == code {
			expected = true
			break
		}
	}
	if !expected {
		return s
	}
	if p.Method == "GET" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		s.LatencyMS = ms(time.Since(requestStart))
		if err != nil || p.BodyKeyword != "" && !strings.Contains(string(body), p.BodyKeyword) {
			return s
		}
	}
	s.State = model.OK
	return s
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
