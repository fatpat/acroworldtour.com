package auth

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const maxLimiterKeys = 4096 // above this, expired keys are swept on each Attempt

// LoginLimiter counts login attempts per key within a sliding window. It is
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

// Attempt atomically checks the budget of key and, when it is not exhausted,
// reserves one attempt. Call Reset after a successful login.
func (l *LoginLimiter) Attempt(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.failures) > maxLimiterKeys {
		for k := range l.failures {
			l.prune(k)
		}
	}
	if len(l.prune(key)) >= l.max {
		return false
	}
	l.failures[key] = append(l.failures[key], l.now())
	return true
}

// Reset forgets the failures of key (after a successful login).
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

func limiterKey(email, ip string) string {
	return strings.ToLower(strings.TrimSpace(email)) + "|" + ip
}

// ClientIP returns the client address. The configured header
// (CF-Connecting-IP, or the first X-Forwarded-For entry) is honoured only when
// the direct peer is inside a trusted proxy CIDR; otherwise the peer address.
func (a *Auth) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	peer = peer.Unmap()
	for _, p := range a.cfg.TrustedProxies {
		if !p.Contains(peer) {
			continue
		}
		v := r.Header.Get(a.cfg.ClientIPHeader)
		if a.cfg.ClientIPHeader == "X-Forwarded-For" {
			v, _, _ = strings.Cut(v, ",")
		}
		if ip, err := netip.ParseAddr(strings.TrimSpace(v)); err == nil {
			return ip.Unmap().String()
		}
		break
	}
	return host
}
