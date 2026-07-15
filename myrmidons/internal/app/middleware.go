package app

import (
	"net/http"
)

// SecurityHeaders adds additional security headers
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy - strict CSP
		w.Header().Set("Content-Security-Policy", 
			"default-src 'self'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; "+
			"font-src 'self'; "+
			"connect-src 'self'; "+
			"frame-ancestors 'none'; "+
			"form-action 'self';")

		// X-Content-Type-Options
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// X-Frame-Options
		w.Header().Set("X-Frame-Options", "DENY")

		// X-XSS-Protection (legacy but still useful)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "same-origin")

		// Permissions Policy
		w.Header().Set("Permissions-Policy", 
			"geolocation=(), "+
			"microphone=(), "+
			"camera=(), "+
			"payment=(self)")

		// HSTS for HTTPS (only when TLS is present)
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", 
				"max-age=31536000; includeSubDomains; preload")
		}

		// Cache control for sensitive pages
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't allow cross-origin requests for API endpoints
		if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != getOrigin(r) {
			// Strict Same-Origin policy - reject cross-origin
			w.Header().Set("Vary", "Origin")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getOrigin returns the allowed origin
func getOrigin(r *http.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// RemoveServerHeader removes the Server header
func RemoveServerHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove version information from headers
		w.Header().Del("Server")
		w.Header().Set("Server", "Myrmidons")
		next.ServeHTTP(w, r)
	})
}
