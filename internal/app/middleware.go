package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gobugger/gomarket/internal/repo"
	"github.com/gobugger/gomarket/internal/service/vendor"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// SECURE: IP-based rate limiting state
type IPRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	requests map[string]int
	blocked  map[string]time.Time
}

func NewIPRateLimiter() *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		requests: make(map[string]int),
		blocked:  make(map[string]time.Time),
	}
}

func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.limiters[ip]; !exists {
		// 60 requests per minute per IP (more lenient than session-based)
		i.limiters[ip] = rate.NewLimiter(rate.Limit(60)/60, 10)
	}

	return i.limiters[ip]
}

func (i *IPRateLimiter) Allow(ip string) bool {
	limiter := i.getLimiter(ip)
	return limiter.Allow()
}

func (i *IPRateLimiter) Block(ip string, duration time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.blocked[ip] = time.Now().Add(duration)
}

func (i *IPRateLimiter) IsBlocked(ip string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	if blockedAt, exists := i.blocked[ip]; exists {
		if time.Now().After(blockedAt) {
			delete(i.blocked, ip)
			return false
		}
		return true
	}
	return false
}

func (i *IPRateLimiter) Cleanup() {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := time.Now()
	for ip, blockedAt := range i.blocked {
		if now.After(blockedAt) {
			delete(i.blocked, ip)
		}
	}
}

// GetClientIP extracts the real client IP from request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			xff = xff[:idx]
		}
		xff = strings.TrimSpace(xff)
		if ip := net.ParseIP(xff); ip != nil {
			return ip.String()
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func SetSecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURE: Content Security Policy - restrict to self and data URIs
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline';")

		// SECURE: Referrer Policy - don't leak referrer to external sites
		w.Header().Set("Referrer-Policy", "same-origin")

		// SECURE: Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// SECURE: Clickjacking protection
		w.Header().Set("X-Frame-Options", "deny")

		// SECURE: XSS filter (legacy browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// SECURE: HSTS - Force HTTPS for 1 year (only in production)
		// Note: This should only be set when serving over HTTPS
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// SECURE: Permissions Policy - disable unnecessary browser features
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Remove Date header to prevent information leakage
		w.Header()["Date"] = nil

		next.ServeHTTP(w, r)
	})
}

func (app *Application) InjectLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		requestID := uuid.New()
		userID, ok := app.SessionManager.Get(ctx, userIDKey).(uuid.UUID)

		logger := slog.Default()
		if ok {
			logger = logger.With("requestID", requestID, "userID", userID)
		} else {
			logger = logger.With("requestID", requestID)
		}

		ctx = context.WithValue(ctx, "logger", logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *Application) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("reguest", "addr", r.RemoteAddr, "protocol", r.Proto, "method", r.Method, "url", r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

func (app *Application) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.SessionManager.Exists(r.Context(), userIDKey) {
			next.ServeHTTP(w, r)
		} else {
			l := app.getLocalizer(r.Context())
			app.notifyUser(r.Context(), true, l.Translate("You need to be logged in to access that page"))
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	})
}

func (app *Application) RequireNoAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.SessionManager.Exists(r.Context(), userIDKey) {
			next.ServeHTTP(w, r)
		} else {
			app.clientError(w, http.StatusBadRequest)
		}
	})
}

func (app *Application) CustomerOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if uid, ok := app.SessionManager.Get(ctx, userIDKey).(uuid.UUID); ok && !vendor.HasLicense(ctx, repo.New(app.Db), uid) {
			next.ServeHTTP(w, r)
		} else {
			app.clientError(w, http.StatusUnauthorized)
		}
	})
}

func (app *Application) RequireVendor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if uid, ok := app.SessionManager.Get(ctx, userIDKey).(uuid.UUID); ok && vendor.HasLicense(ctx, repo.New(app.Db), uid) {
			next.ServeHTTP(w, r)
		} else {
			app.clientError(w, http.StatusUnauthorized)
		}
	})
}

func (app *Application) GetRateLimitKey(r *http.Request) (string, error) {
	uid, ok := app.SessionManager.Get(r.Context(), rateLimitKey).(uuid.UUID)
	if !ok {
		return "", fmt.Errorf("you need to go through the gate in order to access this resource")
	}
	return uid.String(), nil
}

// SECURE: Rate limit headers middleware - adds rate limit information to responses
func RateLimitHeaders(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add standard rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Window", window.String())
			w.Header().Set("X-RateLimit-Policy", fmt.Sprintf("limit=%d,window=%s", limit, window))

			next.ServeHTTP(w, r)
		})
	}
}

// SECURE: Circuit breaker pattern for external service calls
type CircuitBreaker struct {
	mu           sync.RWMutex
	state        string // "closed", "open", "half-open"
	failures     int
	threshold    int
	timeout      time.Duration
	lastFailTime time.Time
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     "closed",
		threshold: threshold,
		timeout:   timeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == "closed" || cb.state == "half-open"
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == "half-open" {
		cb.state = "closed"
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = "open"
	}
}

func (cb *CircuitBreaker) Check() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "open" && time.Since(cb.lastFailTime) > cb.timeout {
		cb.state = "half-open"
	}
}

// IP-based tracking for escalation
type IPTracking struct {
	mu          sync.RWMutex
	failedAuth  map[string]int
	lastAttempt map[string]time.Time
}

func NewIPTracking() *IPTracking {
	return &IPTracking{
		failedAuth:  make(map[string]int),
		lastAttempt: make(map[string]time.Time),
	}
}

func (t *IPTracking) RecordFailedAuth(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failedAuth[ip]++
	t.lastAttempt[ip] = time.Now()
}

func (t *IPTracking) ShouldEscalate(ip string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	// SECURE: Require CAPTCHA after 3 failed authentication attempts
	return t.failedAuth[ip] >= 3
}

func (t *IPTracking) Clear(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failedAuth, ip)
	delete(t.lastAttempt, ip)
}

func LimitBodySize(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			next.ServeHTTP(w, r)
		})
	}
}
