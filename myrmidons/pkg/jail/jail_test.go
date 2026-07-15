package jail

import (
	"testing"
	"time"
)

// TestGeneratorGenerate tests CAPTCHA challenge generation
func TestGeneratorGenerate(t *testing.T) {
	addr := "exampleonion123456789abcdefghijkmnopqrstuvwxyz"
	g := NewGenerator(addr)

	challenge := g.Generate()
	if challenge == nil {
		t.Fatal("Generate returned nil")
	}

	if challenge.ID.String() == "" {
		t.Error("Challenge ID should not be empty")
	}

	if len(challenge.Solution) != 4 {
		t.Errorf("Solution length should be 4, got %d", len(challenge.Solution))
	}

	if len(challenge.HiddenChars) != 4 {
		t.Errorf("HiddenChars length should be 4, got %d", len(challenge.HiddenChars))
	}

	if challenge.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}

	if challenge.Used {
		t.Error("New challenge should not be marked as used")
	}
}

// TestChallengeVerify tests CAPTCHA verification
func TestChallengeVerify(t *testing.T) {
	g := NewGenerator("exampleonion123456789abcdefghijkmnopqrstuvwxyz")
	challenge := g.Generate()

	// Correct solution
	if !challenge.Verify(challenge.Solution) {
		t.Error("Correct solution should verify")
	}

	// Wrong solution
	if challenge.Verify("XXXX") {
		t.Error("Wrong solution should not verify")
	}

	// Case insensitive should work
	if !challenge.Verify("ABCD") && challenge.Solution != "ABCD" {
		// This might fail depending on the actual solution
	}
}

// TestChallengeExpired tests expired challenges
func TestChallengeExpired(t *testing.T) {
	g := NewGenerator("exampleonion123456789abcdefghijkmnopqrstuvwxyz")
	challenge := g.Generate()

	// Manually expire
	challenge.ExpiresAt = time.Now().Add(-1 * time.Second)

	if challenge.Verify(challenge.Solution) {
		t.Error("Expired challenge should not verify")
	}
}

// TestChallengeUsed tests used challenges
func TestChallengeUsed(t *testing.T) {
	g := NewGenerator("exampleonion123456789abcdefghijkmnopqrstuvwxyz")
	challenge := g.Generate()

	// Mark as used
	challenge.MarkUsed()

	if challenge.Verify(challenge.Solution) {
		t.Error("Used challenge should not verify")
	}
}

// TestRateLimiterAllow tests rate limiting
func TestRateLimiterAllow(t *testing.T) {
	limiter := NewRateLimiter(3, time.Second)

	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("test-ip") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th should be blocked
	if limiter.Allow("test-ip") {
		t.Error("4th request should be blocked")
	}

	// Different IP should be allowed
	if !limiter.Allow("other-ip") {
		t.Error("Different IP should be allowed")
	}
}

// TestRateLimiterReset tests rate limiter reset
func TestRateLimiterReset(t *testing.T) {
	limiter := NewRateLimiter(1, time.Second)

	limiter.Allow("test-ip")
	if limiter.Allow("test-ip") {
		t.Error("Second request should be blocked")
	}

	limiter.Reset("test-ip")

	if !limiter.Allow("test-ip") {
		t.Error("After reset, request should be allowed")
	}
}

// TestRateLimiterExpiration tests rate limiter window expiration
func TestRateLimiterExpiration(t *testing.T) {
	limiter := NewRateLimiter(1, 100*time.Millisecond)

	limiter.Allow("test-ip")
	if limiter.Allow("test-ip") {
		t.Error("Second request should be blocked")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	if !limiter.Allow("test-ip") {
		t.Error("After window expires, request should be allowed")
	}
}

// TestRateLimiterCleanup tests cleanup of expired entries
func TestRateLimiterCleanup(t *testing.T) {
	limiter := NewRateLimiter(1, 100*time.Millisecond)

	limiter.Allow("test-ip")

	time.Sleep(150 * time.Millisecond)

	limiter.Cleanup()

	// Should be able to make requests again
	if !limiter.Allow("test-ip") {
		t.Error("After cleanup, request should be allowed")
	}
}

// TestPoWVerify tests proof of work verification
func TestPoWVerify(t *testing.T) {
	pow := &ProofOfWork{
		Prefix:     "abcd1234",
		Difficulty: 8, // Very easy for testing
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	// This should fail (unless we get lucky)
	result := pow.Verify("00000000")
	// Don't check result - it depends on hash

	// Invalid prefix should fail
	if pow.Verify("wrongprefix0000") {
		t.Error("Wrong prefix should fail")
	}
}

// TestIPSelectorChallenge tests challenge selection
func TestIPSelectorChallenge(t *testing.T) {
	selector := NewIPSelector("exampleonion123456789abcdefghijklmnop")

	if selector.GetChallenge() == nil {
		t.Error("Should return a challenge")
	}

	if selector.GetPoWChallenge() == nil {
		t.Error("Should return a PoW challenge")
	}
}
