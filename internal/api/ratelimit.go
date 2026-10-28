package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// authWindow is how long failed admin authentications are remembered, and
// authMaxFailures how many are tolerated within it. The limit is deliberately
// generous: it exists to stop an unattended script grinding through a bearer
// token, not to punish an operator who mistypes once or twice.
const (
	authWindow      = 5 * time.Minute
	authMaxFailures = 20
)

type failureRecord struct {
	count   int
	firstAt time.Time
}

// AuthLimiter throttles repeated failed admin authentication per client
// address. Subscription delivery is exempt on purpose: airport clients retry
// on their own schedule and locking them out would break the service the
// panel exists to provide.
type AuthLimiter struct {
	mu        sync.Mutex
	failures  map[string]*failureRecord
	now       func() time.Time
	trustProp bool
}

func NewAuthLimiter(trustProxy bool) *AuthLimiter {
	return &AuthLimiter{
		failures:  make(map[string]*failureRecord),
		now:       time.Now,
		trustProp: trustProxy,
	}
}

// clientKey resolves the address bucket a request counts against. A reverse
// proxy collapses every browser onto one address, so X-Forwarded-For is only
// consulted when the operator declares the fronting proxy trustworthy.
func (l *AuthLimiter) clientKey(r *http.Request) string {
	if l.trustProp {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			origin := strings.TrimSpace(strings.Split(fwd, ",")[0])
			if origin != "" {
				return origin
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// retryAfter reports how long the address must wait, or 0 when allowed.
func (l *AuthLimiter) retryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec, ok := l.failures[key]
	if !ok {
		return 0
	}
	age := l.now().Sub(rec.firstAt)
	if age > authWindow {
		delete(l.failures, key)
		return 0
	}
	if rec.count < authMaxFailures {
		return 0
	}
	return authWindow - age
}

func (l *AuthLimiter) recordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	rec, ok := l.failures[key]
	if !ok {
		// Bound the map: an attacker spoofing many addresses must not grow it
		// without limit. Expired buckets are swept before inserting a new one.
		for k, v := range l.failures {
			if now.Sub(v.firstAt) > authWindow {
				delete(l.failures, k)
			}
		}
		if len(l.failures) >= 4096 {
			l.evictOldestLocked(now)
		}
		l.failures[key] = &failureRecord{count: 1, firstAt: now}
		return
	}
	if now.Sub(rec.firstAt) > authWindow {
		rec.count, rec.firstAt = 1, now
		return
	}
	rec.count++
}

func (l *AuthLimiter) evictOldestLocked(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for k, v := range l.failures {
		if oldestKey == "" || v.firstAt.Before(oldest) {
			oldestKey, oldest = k, v.firstAt
		}
	}
	if oldestKey != "" {
		delete(l.failures, oldestKey)
	}
}

func (l *AuthLimiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// retryAfterHeader renders a Retry-After value for a throttled client, so the
// console can show when to try again instead of spinning.
func retryAfterHeader(wait time.Duration) string {
	seconds := int64(wait / time.Second)
	if wait%time.Second != 0 {
		seconds++
	}
	return strconv.FormatInt(seconds, 10)
}
