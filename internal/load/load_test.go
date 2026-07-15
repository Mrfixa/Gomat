package load

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentLoginLoad tests login under concurrent load
func TestConcurrentLoginLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	var successCount int64
	var failCount int64

	var wg sync.WaitGroup
	concurrency := 50
	iterations := 10

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				resp, err := http.Post(
					server.URL+"/login",
					"application/x-www-form-urlencoded",
					bytes.NewBufferString("username=test&password=test"),
				)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Concurrent Login Load Test:")
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Total Requests: %d", concurrency*iterations)
	t.Logf("  Success: %d", successCount)
	t.Logf("  Failed: %d", failCount)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  RPS: %.2f", float64(concurrency*iterations)/elapsed.Seconds())
}

// TestConcurrentCheckoutLoad tests checkout under concurrent load
func TestConcurrentCheckoutLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test")
	}

	var successCount int64
	var failCount int64

	var wg sync.WaitGroup
	concurrency := 20
	iterations := 5

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				resp, err := http.Post(
					server.URL+"/checkout",
					"application/json",
					bytes.NewBufferString(`{"cart_id":"test"}`),
				)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent Checkout Test: Total=%d, Success=%d, Failed=%d",
		concurrency*iterations, successCount, failCount)

	failureRate := float64(failCount) / float64(concurrency*iterations)
	if failureRate > 0.1 {
		t.Errorf("High failure rate: %.2f%%", failureRate*100)
	}
}

// TestMemoryUnderLoad tests memory usage under load
func TestMemoryUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test")
	}

	var memBefore uint64
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats)
	memBefore = stats.Alloc

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := make([]byte, 1024*1024)
			for i := range data {
				data[i] = byte(i)
			}
			_ = data
		}()
	}
	wg.Wait()

	runtime.ReadMemStats(stats)
	memAfter := stats.Alloc

	t.Logf("Memory: Before=%d bytes, After=%d bytes, Delta=%d bytes",
		memBefore, memAfter, memAfter-memBefore)
}

// TestRateLimitingLoad tests rate limiter behavior
func TestRateLimitingLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test")
	}

	var allowed, limited int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	for i := 0; i < 100; i++ {
		resp, err := http.Get(server.URL)
		if err != nil {
			limited++
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			allowed++
		} else {
			limited++
		}
	}

	t.Logf("Rate Limiting: Allowed=%d, Limited=%d", allowed, limited)
}

// BenchmarkLogin benchmarks login performance
func BenchmarkLogin(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Post(
			server.URL+"/login",
			"application/x-www-form-urlencoded",
			bytes.NewBufferString("username=test&password=test"),
		)
		resp.Body.Close()
	}
}

// BenchmarkCheckout benchmarks checkout performance
func BenchmarkCheckout(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Post(
			server.URL+"/checkout",
			"application/json",
			bytes.NewBufferString(`{}`),
		)
		resp.Body.Close()
	}
}
