package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"infineon/internal/api"
)

type ipLimitEntry struct {
	Count       int
	WindowStart time.Time
	BlockedTill time.Time
}

type IPRateLimiter struct {
	mu          sync.Mutex
	entries     map[string]ipLimitEntry
	limit       int
	window      time.Duration
	block       time.Duration
	maxEntries  int
}

func NewIPRateLimiter(
	limit int,
	window time.Duration,
	block time.Duration,
) *IPRateLimiter {
	return &IPRateLimiter{
		entries:    make(map[string]ipLimitEntry),
		limit:      limit,
		window:     window,
		block:      block,
		maxEntries: 10000,
	}
}

func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		if !l.allow(ip) {
			w.Header().Set("Retry-After", strconv.Itoa(int(l.block.Seconds())))
			api.WriteError(
				w,
				r,
				http.StatusTooManyRequests,
				"rate_limited",
				"Too many login attempts. Try again later.",
				nil,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *IPRateLimiter) allow(ip string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.entries) >= l.maxEntries {
		for key, entry := range l.entries {
			if now.Sub(entry.WindowStart) > l.window &&
				now.After(entry.BlockedTill) {
				delete(l.entries, key)
			}
		}
	}

	entry, exists := l.entries[ip]
	if !exists || now.Sub(entry.WindowStart) >= l.window {
		l.entries[ip] = ipLimitEntry{
			Count:       1,
			WindowStart: now,
		}
		return true
	}

	if now.Before(entry.BlockedTill) {
		return false
	}

	entry.Count++
	if entry.Count > l.limit {
		entry.BlockedTill = now.Add(l.block)
		l.entries[ip] = entry
		return false
	}

	l.entries[ip] = entry
	return true
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if host, _, err := net.SplitHostPort(forwarded); err == nil {
			return host
		}
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
