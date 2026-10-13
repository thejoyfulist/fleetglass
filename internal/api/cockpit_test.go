package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/store"
)

func cockpitAPI(t *testing.T) *API {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s, "ingest-token", "")
}
func TestEmbeddedCockpit(t *testing.T) {
	h := cockpitAPI(t).Handler()
	for _, tc := range []struct{ path, content, cache string }{
		{"/", "text/html", "no-cache"}, {"/agents", "text/html", "no-cache"}, {"/costs", "text/html", "no-cache"}, {"/favicon.svg", "image/svg+xml", ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, localRequest("GET", tc.path, nil))
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), tc.content) || w.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: %d %v", tc.path, w.Code, w.Header())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, localRequest("GET", "/", nil))
	body := w.Body.String()
	assets := regexp.MustCompile(`/assets/[^" ]+\.(?:js|css)`).FindAllString(body, -1)
	if len(assets) != 2 {
		t.Fatalf("expected JS and CSS assets, got %v", assets)
	}
	for _, asset := range assets {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, localRequest("GET", asset, nil))
		content := "javascript"
		if strings.HasSuffix(asset, ".css") {
			content = "text/css"
		}
		if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") || !strings.Contains(w.Header().Get("Content-Type"), content) {
			t.Fatalf("%s: %d %v", asset, w.Code, w.Header())
		}
	}
}
func TestReadSessionFlow(t *testing.T) {
	a := cockpitAPI(t)
	a.ReadToken = fixture("read-token")
	h := a.Handler()
	req := func(method, path string, body []byte, c *http.Cookie) *httptest.ResponseRecorder {
		r := localRequest(method, path, bytes.NewReader(body))
		if c != nil {
			r.AddCookie(c)
		}
		if path == "/api/v1/live" {
			ctx, cancel := context.WithCancel(r.Context())
			cancel()
			r = r.WithContext(ctx)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := req("GET", "/api/v1/hosts", nil, nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := req("GET", "/", nil, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Fleetglass") {
		t.Fatalf("sign-in shell unavailable: %d", w.Code)
	}
	if w := req("POST", "/api/v1/auth", []byte(`{"token":"wrong"}`), nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := req("POST", "/api/v1/auth", []byte(`{"token":"read-token"}`), nil)
	if w.Code != 204 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("sign-in: %d", w.Code)
	}
	c := w.Result().Cookies()[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.MaxAge <= 0 {
		t.Fatalf("cookie: %+v", c)
	}
	for _, p := range []string{"/api/v1/hosts", "/api/v1/live"} {
		w = req("GET", p, nil, c)
		if w.Code != 200 {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	a.mu.Lock()
	a.sessions[c.Value] = time.Now().Add(-time.Second)
	a.mu.Unlock()
	if w = req("GET", "/api/v1/hosts", nil, c); w.Code != 401 {
		t.Fatalf("expired: %d", w.Code)
	}
	w = req("POST", "/api/v1/auth", []byte(`{"token":"read-token"}`), nil)
	c = w.Result().Cookies()[0]
	if w = req("POST", "/api/v1/auth/signout", nil, c); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = req("GET", "/api/v1/hosts", nil, c); w.Code != 401 {
		t.Fatalf("sign-out: %d", w.Code)
	}
}
func TestLoopbackReadsStayOpen(t *testing.T) {
	h := cockpitAPI(t).Handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, localRequest("GET", "/api/v1/hosts", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestSecureSessionBehindHTTPS(t *testing.T) {
	a := cockpitAPI(t)
	a.ReadToken = fixture("read-token")
	r := localRequest("POST", "/api/v1/auth", strings.NewReader(`{"token":"read-token"}`))
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("cookie is not Secure")
	}
}

func TestSignInRateLimitByAddress(t *testing.T) {
	a := cockpitAPI(t)
	a.ReadToken = fixture("read-token")
	h := a.Handler()
	for i := 0; i < maxLoginAttempts+1; i++ {
		r := localRequest("POST", "/api/v1/auth", strings.NewReader(`{"token":"wrong"}`))
		r.RemoteAddr = "192.0.2.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := http.StatusUnauthorized
		if i == maxLoginAttempts {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("attempt %d: got %d, want %d", i+1, w.Code, want)
		}
	}
	r := localRequest("POST", "/api/v1/auth", strings.NewReader(`{"token":"read-token"}`))
	r.RemoteAddr = "192.0.2.2:9876"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("other address: %d", w.Code)
	}
	a.mu.Lock()
	a.loginAttempts["192.0.2.1"] = []time.Time{time.Now().Add(-2 * loginWindow)}
	a.mu.Unlock()
	r = localRequest("POST", "/api/v1/auth", strings.NewReader(`{"token":"read-token"}`))
	r.RemoteAddr = "192.0.2.1:4567"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("window expired: %d", w.Code)
	}
}

func TestSessionCapacityEvictsOldest(t *testing.T) {
	a := cockpitAPI(t)
	a.ReadToken = fixture("read-token")
	h := a.Handler()
	a.mu.Lock()
	for i := 0; i < maxSessions; i++ {
		a.sessions[fmt.Sprintf("session-%d", i)] = time.Now().Add(time.Hour + time.Duration(i)*time.Second)
	}
	a.mu.Unlock()
	r := localRequest("POST", "/api/v1/auth", strings.NewReader(`{"token":"read-token"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatal(w.Code)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.sessions) != maxSessions {
		t.Fatalf("session count: %d", len(a.sessions))
	}
	if _, exists := a.sessions["session-0"]; exists {
		t.Fatal("oldest session was not evicted")
	}
}

func TestLoginAddressCapacity(t *testing.T) {
	a := cockpitAPI(t)
	now := time.Now()
	for i := 0; i <= maxLoginAddresses; i++ {
		if !a.allowLogin(fmt.Sprintf("address-%d:1234", i), now.Add(time.Duration(i)*time.Nanosecond)) {
			t.Fatal("unexpected rate limit")
		}
	}
	if len(a.loginAttempts) != maxLoginAddresses {
		t.Fatalf("address count: %d", len(a.loginAttempts))
	}
	if _, exists := a.loginAttempts["address-0"]; exists {
		t.Fatal("oldest address was not evicted")
	}
}
