package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Batches are stored as individual files so a failed request cannot reorder
// observations or lose data during an agent restart.
func (a *agent) enqueueAndFlush(batch []byte) bool {
	if a.c.BufferDir == "" {
		return a.postBatch(batch)
	}
	if err := os.MkdirAll(a.c.BufferDir, 0700); err != nil {
		log.Printf("ingest buffer unavailable: %v", err)
		return false
	}
	name := fmt.Sprintf("%020d-%06d.json", time.Now().UnixNano(), os.Getpid())
	tmp, err := os.CreateTemp(a.c.BufferDir, ".batch-")
	if err != nil {
		log.Printf("ingest buffer unavailable: %v", err)
		return false
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(batch)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(a.c.BufferDir, name))
	}
	if err != nil {
		log.Printf("ingest buffer unavailable: %v", err)
		return false
	}
	a.flushBuffer()
	return true
}

func (a *agent) postBatch(batch []byte) bool {
	req, err := http.NewRequest("POST", strings.TrimRight(a.c.ServerURL, "/")+"/api/v1/ingest", bytes.NewReader(batch))
	if err != nil {
		log.Print("ingest request failed")
		return false
	}
	req.Header.Set("Authorization", "Bearer "+a.c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		log.Print("ingest connection failed")
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		log.Printf("ingest HTTP %d", resp.StatusCode)
		return false
	}
	return true
}

func (a *agent) flushBuffer() {
	entries, err := os.ReadDir(a.c.BufferDir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	// The fast cadence produces about forty batches in ten minutes. The cap
	// also bounds storage if a caller invokes send more often than scheduled.
	for len(names) > 64 {
		_ = os.Remove(filepath.Join(a.c.BufferDir, names[0]))
		names = names[1:]
	}
	for _, name := range names {
		path := filepath.Join(a.c.BufferDir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > 10*time.Minute {
			_ = os.Remove(path)
			continue
		}
		if time.Now().Before(a.retryAt) {
			return
		}
		batch, err := os.ReadFile(path)
		if err != nil || !a.postBatch(batch) {
			if a.retryDelay == 0 {
				a.retryDelay = time.Second
			} else {
				a.retryDelay *= 2
				if a.retryDelay > 30*time.Second {
					a.retryDelay = 30 * time.Second
				}
			}
			a.retryAt = time.Now().Add(a.retryDelay)
			return
		}
		a.retryDelay = 0
		a.retryAt = time.Time{}
		if err := os.Remove(path); err != nil {
			return
		}
	}
}
