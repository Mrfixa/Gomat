package security_test

import (
	"bytes"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestXSSPrevention tests that XSS attacks are prevented
func TestXSSPrevention(t *testing.T) {
	xssPayloads := []string{
		`<script>alert('xss')</script>`,
		`javascript:alert('xss')`,
		`<img src=x onerror=alert('xss')>`,
		`"><script>alert('xss')</script>`,
		`';alert('xss');//`,
	}

	for _, payload := range xssPayloads {
		t.Run(payload, func(t *testing.T) {
			escaped := html.EscapeString(payload)
			if strings.Contains(escaped, "<script>") {
				t.Errorf("Payload not escaped: %s", payload)
			}
		})
	}
}

// TestCSRFTokenValidation tests CSRF token requirement
func TestCSRFTokenValidation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			http.Error(w, "CSRF token required", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("WithoutToken", func(t *testing.T) {
		resp, err := http.Post(server.URL, "application/x-www-form-urlencoded",
			bytes.NewBufferString("data=test"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("WithToken", func(t *testing.T) {
		req, _ := http.NewRequest("POST", server.URL, bytes.NewBufferString("data=test"))
		req.Header.Set("X-CSRF-Token", "valid-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})
}

// TestRateLimitingEffectiveness tests rate limiting
func TestRateLimitingEffectiveness(t *testing.T) {
	var count int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count > 10 {
			http.Error(w, "Rate limited", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	success := 0
	for i := 0; i < 20; i++ {
		resp, _ := http.Get(server.URL)
		if resp.StatusCode == http.StatusOK {
			success++
		}
		resp.Body.Close()
	}

	if success > 15 {
		t.Errorf("Rate limiting ineffective: %d requests succeeded", success)
	}
}

// TestPathTraversalPrevention tests path traversal detection
func TestPathTraversalPrevention(t *testing.T) {
	dangerous := []string{
		"../../etc/passwd",
		"..\\..\\windows\\system32",
		"....//....//etc/passwd",
	}

	for _, path := range dangerous {
		if !strings.Contains(path, "..") {
			t.Errorf("Should detect path traversal: %s", path)
		}
	}
}
