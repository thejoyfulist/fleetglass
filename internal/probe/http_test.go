package probe

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

func TestHTTPCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "Fleetglass/1.0 (+HTTP availability probe)" {
			t.Errorf("user agent %q", r.UserAgent())
		}
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", 302)
		case "/redirect-credentials":
			http.Redirect(w, r, "http://name:example@"+r.Host+"/ok", 302)
		case "/slow":
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(200)
		default:
			w.WriteHeader(201)
			w.Write([]byte("ready"))
		}
	}))
	defer server.Close()
	p := config.HTTPProbe{Name: "site", DisplayName: "Site", URL: server.URL + "/ok", Method: "GET", ExpectedStatus: []int{201}, BodyKeyword: "ready", TimeoutMS: 500, IntervalSeconds: 60}
	if s := HTTPCheck(context.Background(), p, time.Now()); s.State != model.OK || s.StatusCode != 201 || s.FirstByteMS < 0 {
		t.Fatalf("success: %+v", s)
	}
	p.BodyKeyword = "missing"
	if s := HTTPCheck(context.Background(), p, time.Now()); s.State != model.Crit {
		t.Fatalf("keyword: %+v", s)
	}
	p.BodyKeyword = ""
	p.URL = server.URL + "/redirect"
	if s := HTTPCheck(context.Background(), p, time.Now()); s.State != model.Crit || s.StatusCode != 302 {
		t.Fatalf("redirect blocked: %+v", s)
	}
	p.FollowRedirects = true
	if s := HTTPCheck(context.Background(), p, time.Now()); s.State != model.OK {
		t.Fatalf("redirect followed: %+v", s)
	}
	p.URL = server.URL + "/redirect-credentials"
	if s := HTTPCheck(context.Background(), p, time.Now()); s.State != model.Crit || s.StatusCode != 302 {
		t.Fatalf("credential redirect accepted: %+v", s)
	}
	p.URL = server.URL + "/slow"
	p.TimeoutMS = 20
	if s := HTTPCheck(context.Background(), p, time.Now()); s.State != model.Crit {
		t.Fatalf("timeout: %+v", s)
	}
}

func TestHTTPCheckTLSCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	cert, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test-ca.pem")
	if err = os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", path)
	p := config.HTTPProbe{Name: "tls", DisplayName: "TLS", URL: server.URL, Method: "HEAD", TimeoutMS: 500, IntervalSeconds: 60}
	s := HTTPCheck(context.Background(), p, time.Now())
	if s.State != model.OK || s.CertificateDays == nil || *s.CertificateDays < 0 || s.TLSMS <= 0 {
		t.Fatalf("TLS probe: %+v", s)
	}
}
