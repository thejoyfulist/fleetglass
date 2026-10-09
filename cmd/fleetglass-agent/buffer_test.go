package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
)

func TestBufferFlushesInOrderAfterOutage(t *testing.T) {
	var mu sync.Mutex
	available := false
	received := []int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if !available {
			w.WriteHeader(503)
			return
		}
		var b struct {
			Sequence int `json:"sequence"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Error(err)
		}
		received = append(received, b.Sequence)
		w.WriteHeader(204)
	}))
	defer server.Close()
	a := &agent{c: config.Config{ServerURL: server.URL, Token: "test", BufferDir: filepath.Join(t.TempDir(), "buffer")}, client: server.Client()}
	for i := 1; i <= 2; i++ {
		b, _ := json.Marshal(map[string]int{"sequence": i})
		if !a.enqueueAndFlush(b) {
			t.Fatal("could not queue batch")
		}
	}
	mu.Lock()
	available = true
	mu.Unlock()
	// A new process must recover the previous process's on-disk queue.
	a = &agent{c: a.c, client: server.Client(), retryAt: time.Time{}}
	if !a.enqueueAndFlush([]byte(`{"sequence":3}`)) {
		t.Fatal("could not queue third batch")
	}
	if !reflect.DeepEqual(received, []int{1, 2, 3}) {
		t.Fatalf("flush order: %v", received)
	}
	entries, err := os.ReadDir(a.c.BufferDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("buffer not empty: %v, %v", entries, err)
	}
}
