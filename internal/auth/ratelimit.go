package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LoginLimiter counts failed logins per key within a sliding window. It is
// in memory: the application runs as a single process.
type LoginLimiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu       sync.Mutex
	failures map[string][]time.Time
}

// NewLoginLimiter allows max failures per key within window.
func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{max: max, window: window, now: time.Now, failures: map[string][]time.Time{}}
}

// prune drops expired failures for key and returns the live ones. Caller holds mu.
func (l *LoginLimiter) prune(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	live := l.failures[key][:0]
	for _, t := range l.failures[key] {
		if t.After(cutoff) {
			live = append(live, t)
		}
	}
	if len(live) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = live
	return live
}

// Allow reports whether another attempt may be made for key.
func (l *LoginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key)) < l.max
}

// Fail records a failed attempt.
func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key)
	l.failures[key] = append(l.failures[key], l.now())
}

// Reset forgets the failures of key (after a successful login).
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// limiterKey is the lower-cased email plus the client IP. The IP is the
// direct peer address; trusted-proxy handling is not implemented yet.
func limiterKey(email string, r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return strings.ToLower(strings.TrimSpace(email)) + "|" + ip
}
