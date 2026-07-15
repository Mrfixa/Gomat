package tor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Tor provides SOCKS5 proxy functionality through Tor
type Tor struct {
	socksAddr   string
	controlAddr string
	controlPort string
}

// Dialer implements net.Dialer through Tor SOCKS5
func (t *Tor) Dial(network, addr string) (net.Conn, error) {
	return net.Dial("tcp", t.socksAddr)
}

// DialContext dials through Tor with context support
func (t *Tor) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{}
	return dialer.DialContext(ctx, "tcp", t.socksAddr)
}

// HTTPClient returns an HTTP client that routes through Tor
func (t *Tor) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: t.DialContext,
		},
		Timeout: 30 * time.Second,
	}
}

// GetOnionAddress retrieves the onion address from Tor control port
func (t *Tor) GetOnionAddress() (string, error) {
	conn, err := net.Dial("tcp", t.controlAddr)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Tor control: %w", err)
	}
	defer conn.Close()

	// Authenticate (no password)
	fmt.Fprintf(conn, "AUTHENTICATE\r\n")
	
	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "250") {
		return "", fmt.Errorf("Tor authentication failed")
	}

	// Get onion address for service 0
	fmt.Fprintf(conn, "GETINFO ns/all\r\n")
	n, _ = conn.Read(buf)
	
	// Parse onion address from response
	// In production, use ADD_ONION to create a new service
	return "", fmt.Errorf("onion address retrieval not implemented - use --onion flag")
}

// New creates a new Tor instance with the given SOCKS5 proxy address
func New(socksAddr string) *Tor {
	return &Tor{
		socksAddr: socksAddr,
	}
}

// StartTor attempts to start Tor as a subprocess
func StartTor(ctx context.Context, dataDir string) (*Tor, error) {
	torPath, err := exec.LookPath("tor")
	if err != nil {
		return nil, fmt.Errorf("tor not found in PATH: %w", err)
	}

	// Create torrc configuration
	torrc := fmt.Sprintf(`
DataDirectory %s
SOCKSPort 127.0.0.1:9050
ControlPort 9051
CookieAuthentication 1
`, dataDir)

	torrcPath := dataDir + "/torrc"
	if err := os.WriteFile(torrcPath, []byte(torrc), 0600); err != nil {
		return nil, fmt.Errorf("failed to write torrc: %w", err)
	}

	// Start tor process
	cmd := exec.CommandContext(ctx, torPath, "-f", torrcPath)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tor: %w", err)
	}

	// Wait for tor to be ready
	for i := 0; i < 30; i++ {
		conn, err := net.Dial("tcp", "127.0.0.1:9050")
		if err == nil {
			conn.Close()
			return New("127.0.0.1:9050"), nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("tor failed to start within timeout")
}

// IsTorAddress checks if the given address is a .onion address
func IsTorAddress(addr string) bool {
	return strings.HasSuffix(addr, ".onion")
}

// ParseOnionAddress extracts the hostname from an onion URL
func ParseOnionAddress(addr string) string {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimSuffix(addr, "/")
	return addr
}
