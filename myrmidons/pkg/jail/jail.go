package jail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"html/template"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Challenge represents a CAPTCHA challenge
type Challenge struct {
	ID          uuid.UUID
	OnionAddr   string
	Solution    string
	HiddenChars []int
	ExpiresAt   time.Time
	Used       bool
}

// Generator generates CAPTCHA challenges
type Generator struct {
	address   string
	difficulty int
}

// NewGenerator creates a new CAPTCHA generator
func NewGenerator(onionAddress string) *Generator {
	return &Generator{
		address:   onionAddress,
		difficulty: 4,
	}
}

// Generate creates a new CAPTCHA challenge
func (g *Generator) Generate() *Challenge {
	// Extract 4 characters from onion address
	positions := []int{6, 12, 18, 24}
	solution := make([]byte, len(positions))
	hiddenChars := make([]int, len(positions))

	onion := g.address
	if len(onion) < 25 {
		onion = strings.Repeat("x", 30)
	}

	for i, pos := range positions {
		if pos < len(onion) {
			solution[i] = onion[pos]
			hiddenChars[i] = pos
		} else {
			solution[i] = 'x'
			hiddenChars[i] = -1
		}
	}

	return &Challenge{
		ID:          uuid.New(),
		OnionAddr:   g.address,
		Solution:    strings.ToUpper(string(solution)),
		HiddenChars: hiddenChars,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
}

// Render renders the challenge as HTML
func (c *Challenge) Render() template.HTML {
	// Create masked address display
	display := c.OnionAddr
	for i, pos := range c.HiddenChars {
		if pos >= 0 && pos < len(display) {
			display = display[:pos] + "_" + display[pos+1:]
		}
	}

	// Generate HTML
	html := fmt.Sprintf(`
		<div class="captcha-container">
			<p class="captcha-instructions">
				Enter the missing characters from the onion address below:
			</p>
			<div class="captcha-address">
				<code>%s</code>
			</div>
			<div class="captcha-input">
				<input 
					type="text" 
					name="captcha_solution" 
					maxlength="4" 
					pattern="[A-Za-z0-9]{4}"
					required
					autocomplete="off"
					placeholder="????"
				/>
			</div>
			<input type="hidden" name="captcha_id" value="%s"/>
		</div>
	`, display, c.ID.String())

	return template.HTML(html)
}

// Verify verifies a solution
func (c *Challenge) Verify(solution string) bool {
	if c.Used || time.Now().After(c.ExpiresAt) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(solution), c.Solution)
}

// MarkUsed marks the challenge as used
func (c *Challenge) MarkUsed() {
	c.Used = true
}

// ProofOfWork represents a PoW challenge
type ProofOfWork struct {
	Prefix     string
	Difficulty int
	ExpiresAt  time.Time
}

// PoWGenerator generates PoW challenges
type PoWGenerator struct {
	prefix     string
	difficulty int
	expiresIn  time.Duration
}

// NewPoWGenerator creates a new PoW generator
func NewPoWGenerator(difficulty int) *PoWGenerator {
	// Generate random prefix
	bytes := make([]byte, 8)
	rand.Read(bytes)
	prefix := fmt.Sprintf("%x", bytes)

	return &PoWGenerator{
		prefix:     prefix,
		difficulty: difficulty,
		expiresIn:  5 * time.Minute,
	}
}

// Generate creates a new PoW challenge
func (g *PoWGenerator) Generate() *ProofOfWork {
	return &ProofOfWork{
		Prefix:     g.prefix,
		Difficulty: g.difficulty,
		ExpiresAt:  time.Now().Add(g.expiresIn),
	}
}

// Render renders the PoW challenge as HTML
func (p *ProofOfWork) Render() template.HTML {
	html := fmt.Sprintf(`
		<div class="pow-container">
			<p class="pow-instructions">
				Solve this proof of work to continue:
			</p>
			<div class="pow-prefix">
				Prefix: <code>%s</code>
			</div>
			<div class="pow-input">
				<input 
					type="text" 
					name="pow_solution" 
					placeholder="Solution (hex)"
					required
				/>
			</div>
			<p class="pow-hint">
				Find a hex string where SHA256(prefix + solution) starts with %d zeros.
			</p>
		</div>
	`, p.Prefix, p.Difficulty)

	return template.HTML(html)
}

// Verify verifies a PoW solution
func (p *ProofOfWork) Verify(solution string) bool {
	if time.Now().After(p.ExpiresAt) {
		return false
	}

	data := []byte(p.Prefix + solution)
	hash := sha256.Sum256(data)

	// Check leading zero bits
	required := new(big.Int).Lsh(big.NewInt(1), 256-uint(p.Difficulty))
	hashInt := new(big.Int).SetBytes(hash[:])

	return hashInt.Cmp(required) <= 0
}

// Solve attempts to solve a PoW (for testing/benchmarking)
func (p *ProofOfWork) Solve() (string, error) {
	var solution string
	for i := 0; i < 10000000; i++ { // Limit iterations
		solution = fmt.Sprintf("%x", i)
		if p.Verify(solution) {
			return solution, nil
		}
	}
	return "", fmt.Errorf("failed to solve PoW")
}

// RateLimiter implements rate limiting
type RateLimiter struct {
	requests map[string]*requestCount
	limit    int
	window   time.Duration
}

type requestCount struct {
	count     int
	windowEnd time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string]*requestCount),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request should be allowed
func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()

	rc, exists := r.requests[key]
	if !exists || now.After(rc.windowEnd) {
		r.requests[key] = &requestCount{
			count:     1,
			windowEnd: now.Add(r.window),
		}
		return true
	}

	if rc.count >= r.limit {
		return false
	}

	rc.count++
	return true
}

// Reset resets the rate limit for a key
func (r *RateLimiter) Reset(key string) {
	delete(r.requests, key)
}

// Cleanup removes expired entries
func (r *RateLimiter) Cleanup() {
	now := time.Now()
	for key, rc := range r.requests {
		if now.After(rc.windowEnd) {
			delete(r.requests, key)
		}
	}
}

// IPSelector selects CAPTCHA or PoW based on client behavior
type IPSelector struct {
	rateLimiter *RateLimiter
	jailGenerator *Generator
	powGenerator *PoWGenerator
}

// NewIPSelector creates a new selector
func NewIPSelector(address string) *IPSelector {
	return &IPSelector{
		rateLimiter: NewRateLimiter(10, time.Minute),
		jailGenerator: NewGenerator(address),
		powGenerator: NewPoWGenerator(16),
	}
}

// ShouldChallenge determines if a client should be challenged
func (s *IPSelector) ShouldChallenge(ip string) bool {
	return !s.rateLimiter.Allow(ip)
}

// GetChallenge returns an appropriate challenge for the client
func (s *IPSelector) GetChallenge() interface{} {
	// Return CAPTCHA for normal users
	return s.jailGenerator.Generate()
}

// GetPoWChallenge returns a PoW challenge
func (s *IPSelector) GetPoWChallenge() *ProofOfWork {
	return s.powGenerator.Generate()
}

// Example usage in handler:
//
//   selector := jail.NewIPSelector(siteOnionAddress)
//   
//   func loginHandler(w http.ResponseWriter, r *http.Request) {
//       clientIP := r.RemoteAddr
//       
//       if selector.ShouldChallenge(clientIP) {
//           challenge := selector.GetChallenge()
//           // Show challenge page
//           return
//       }
//       
//       // Process login...
//   }
