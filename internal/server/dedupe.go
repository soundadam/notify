package server

import (
	"sync"
	"time"
)

// Window is an in-memory cooldown keyed by dedupeKey. Single replica is enough.
type Window struct {
	mu   sync.Mutex
	ttl  time.Duration
	seen map[string]time.Time
}

func NewWindow(ttl time.Duration) *Window {
	if ttl <= 0 {
		ttl = DefaultDedupeTTL
	}
	return &Window{ttl: ttl, seen: make(map[string]time.Time)}
}

// Reserve reports whether key is still cooling down. If not, it starts a new window.
func (w *Window) Reserve(key string) (cooldown bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.gcLocked(now)
	if exp, ok := w.seen[key]; ok && now.Before(exp) {
		return true
	}
	w.seen[key] = now.Add(w.ttl)
	return false
}

// Forget drops a reservation after a failed send so the caller can retry.
func (w *Window) Forget(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.seen, key)
}

func (w *Window) gcLocked(now time.Time) {
	if len(w.seen) < 64 {
		return
	}
	for k, exp := range w.seen {
		if !now.Before(exp) {
			delete(w.seen, k)
		}
	}
}
