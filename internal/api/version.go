package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// The optional request contains no host or installation identifiers.
func (a *API) version(w http.ResponseWriter, r *http.Request) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if a.UpdateCheckURL != "" && time.Since(a.updateAt) >= 24*time.Hour {
		a.updateAt = time.Now()
		client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if resp, err := client.Get(a.UpdateCheckURL); err == nil {
			if resp.StatusCode == http.StatusOK && resp.ContentLength <= 4096 {
				var v struct {
					TagName string `json:"tag_name"`
				}
				if json.NewDecoder(http.MaxBytesReader(w, resp.Body, 4096)).Decode(&v) == nil && len(v.TagName) <= 64 {
					a.updateTag = v.TagName
				}
			}
			resp.Body.Close()
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"version": a.Version, "commit": a.Commit, "latest": a.updateTag, "update_check_enabled": a.UpdateCheckURL != ""})
}
