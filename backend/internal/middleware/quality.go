package middleware

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/serveflow/serveflow/backend/internal/response"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	if recorder.status != 0 {
		return
	}
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	if recorder.status == 0 {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.ResponseWriter.Write(body)
}

func RequestLogging(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			entry, err := json.Marshal(struct {
				Method     string `json:"method"`
				Route      string `json:"route"`
				Status     int    `json:"status"`
				DurationMS int64  `json:"duration_ms"`
			}{Method: r.Method, Route: route, Status: status, DurationMS: time.Since(started).Milliseconds()})
			if err == nil {
				logger.Print(string(entry))
			}
		})
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type rateLimitEntry struct {
	started time.Time
	count   int
}

type rateLimiter struct {
	mu       sync.Mutex
	entries  map[string]rateLimitEntry
	limit    int
	window   time.Duration
	requests uint64
}

func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	if limit < 1 || window <= 0 {
		panic("rate limit and window must be positive")
	}
	limiter := &rateLimiter{entries: make(map[string]rateLimitEntry), limit: limit, window: window}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			now := time.Now()
			limiter.mu.Lock()
			entry := limiter.entries[key]
			if entry.started.IsZero() || now.Sub(entry.started) >= limiter.window {
				entry = rateLimitEntry{started: now}
			}
			entry.count++
			limited := entry.count > limiter.limit
			limiter.entries[key] = entry
			limiter.requests++
			if limiter.requests%256 == 0 {
				for address, previous := range limiter.entries {
					if now.Sub(previous.started) >= limiter.window {
						delete(limiter.entries, address)
					}
				}
			}
			limiter.mu.Unlock()
			if limited {
				remaining := time.Until(entry.started.Add(limiter.window))
				retryAfter := int(remaining / time.Second)
				if remaining%time.Second != 0 {
					retryAfter++
				}
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests; please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
