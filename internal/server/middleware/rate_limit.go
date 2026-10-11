package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

type IPRateLimiter struct {
	mu              sync.Mutex
	entries         map[string]rateLimitEntry
	limit           int
	window          time.Duration
	maxEntries      int
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

func NewIPRateLimiter(
	limit int,
	window time.Duration,
) *IPRateLimiter {
	if limit < 1 {
		limit = 10
	}

	if window <= 0 {
		window = time.Minute
	}

	return &IPRateLimiter{
		entries:         make(map[string]rateLimitEntry),
		limit:           limit,
		window:          window,
		maxEntries:      10000,
		cleanupInterval: 5 * time.Minute,
		lastCleanup:     time.Now(),
	}
}

func (l *IPRateLimiter) Allow(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= l.cleanupInterval {
		for key, entry := range l.entries {
			if now.Sub(entry.windowStart) >= l.window {
				delete(l.entries, key)
			}
		}
		l.lastCleanup = now
	}

	if len(l.entries) >= l.maxEntries {
		return false, l.window
	}

	entry, exists := l.entries[ip]
	if !exists || now.Sub(entry.windowStart) >= l.window {
		l.entries[ip] = rateLimitEntry{
			windowStart: now,
			count:       1,
		}
		return true, 0
	}

	if entry.count >= l.limit {
		retryAfter := l.window - now.Sub(entry.windowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	entry.count++
	l.entries[ip] = entry
	return true, 0
}

func LoginRateLimit(
	limiter *IPRateLimiter,
	next http.Handler,
) http.Handler {
	if limiter == nil {
		limiter = NewIPRateLimiter(10, time.Minute)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		allowed, retryAfter := limiter.Allow(ip, time.Now().UTC())
		if !allowed {
			w.Header().Set(
				"Retry-After",
				strconv.Itoa(int(retryAfter.Round(time.Second)/time.Second)),
			)
			http.Error(
				w,
				http.StatusText(http.StatusTooManyRequests),
				http.StatusTooManyRequests,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}

	return "unknown"
}
