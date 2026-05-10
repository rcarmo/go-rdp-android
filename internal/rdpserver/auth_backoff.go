package rdpserver

import (
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

type authBackoffState struct {
	failures    int
	lockedUntil time.Time
}

type authBackoffLimiter struct {
	limit      int
	base       time.Duration
	max        time.Duration
	now        func() time.Time
	mu         sync.Mutex
	byIdentity map[string]authBackoffState
}

func newAuthBackoffLimiter(policy AccessPolicy) *authBackoffLimiter {
	if policy.FailedAuthLimit <= 0 {
		return nil
	}
	return &authBackoffLimiter{
		limit:      policy.FailedAuthLimit,
		base:       policy.FailedAuthBackoff,
		max:        policy.FailedAuthBackoffMax,
		now:        time.Now,
		byIdentity: make(map[string]authBackoffState),
	}
}

func (l *authBackoffLimiter) identity(remote, username string) string {
	return fmt.Sprintf("%s|%s", strings.TrimSpace(remote), strings.ToLower(strings.TrimSpace(username)))
}

func (l *authBackoffLimiter) lockoutRemaining(remote, username string) time.Duration {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := l.identity(remote, username)
	state, ok := l.byIdentity[key]
	if !ok {
		return 0
	}
	now := l.now()
	if state.lockedUntil.After(now) {
		return state.lockedUntil.Sub(now)
	}
	state.lockedUntil = time.Time{}
	l.byIdentity[key] = state
	return 0
}

func (l *authBackoffLimiter) recordFailure(remote, username string) time.Duration {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := l.identity(remote, username)
	state := l.byIdentity[key]
	state.failures++
	if state.failures >= l.limit {
		step := state.failures - l.limit
		state.lockedUntil = l.now().Add(clampBackoff(l.base, l.max, step))
	}
	l.byIdentity[key] = state
	if state.lockedUntil.IsZero() {
		return 0
	}
	return state.lockedUntil.Sub(l.now())
}

func (l *authBackoffLimiter) recordSuccess(remote, username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byIdentity, l.identity(remote, username))
}

func clampBackoff(base, max time.Duration, step int) time.Duration {
	if step <= 0 {
		return base
	}
	if base <= 0 {
		return 0
	}
	if max < base {
		max = base
	}
	multiplier := math.Pow(2, float64(step))
	d := time.Duration(float64(base) * multiplier)
	if d < base {
		d = base
	}
	if d > max {
		return max
	}
	return d
}

func remoteHost(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		return host
	}
	return addr.String()
}
