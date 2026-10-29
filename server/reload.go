package server

import (
	"maps"
	"net/http"
	"sync"

	"github.com/asciimoo/hister/config"
)

// configReloader owns one generation's HTTP configuration. Readers receive
// immutable rule snapshots while writes are serialized with reload preparation.
type configReloader struct {
	mu        sync.Mutex
	current   *config.Config
	overrides []config.Override
}

// beginRequest takes a snapshot for readers and serializes reloads with rule
// writes. No lock is held during searches or long lived WebSocket connections.
func (r *configReloader) beginRequest(req *http.Request) (*config.Config, func()) {
	r.mu.Lock()
	if req.Method == http.MethodPost {
		switch req.URL.Path {
		case "/api/reload", "/api/rules", "/api/add_alias", "/api/delete_alias":
			return r.current, sync.OnceFunc(r.mu.Unlock)
		}
	}
	snapshot := r.snapshotLocked()
	r.mu.Unlock()
	return snapshot, func() {}
}

func (r *configReloader) snapshot() *config.Config {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func (r *configReloader) snapshotLocked() *config.Config {
	snapshot := *r.current
	snapshot.Rules = cloneRules(snapshot.Rules)
	return &snapshot
}

func cloneRules(rules *config.Rules) *config.Rules {
	if rules == nil {
		return nil
	}
	copy := *rules
	copy.Aliases = maps.Clone(rules.Aliases)
	for _, field := range []**config.Rule{&copy.Allow, &copy.Skip, &copy.Priority, &copy.Versioning} {
		if *field != nil {
			rule := **field
			rule.ReStrs = append([]string(nil), rule.ReStrs...)
			*field = &rule
		}
	}
	return &copy
}

func serveReload(c *webContext) {
	lease, ok := c.Request.Context().Value(runtimeRequestKey{}).(*runtimeRequest)
	if !ok {
		http.Error(c.Response, "server runtime is unavailable", http.StatusServiceUnavailable)
		return
	}
	next, err := c.Config.Reload(c.reloader.overrides...)
	if err != nil {
		http.Error(c.Response, err.Error(), http.StatusBadRequest)
		return
	}
	c.releaseConfig()
	status, err := lease.runtime.reload(c.Request, lease, next)
	if err != nil {
		http.Error(c.Response, err.Error(), status)
		return
	}
	serve200(c)
}
