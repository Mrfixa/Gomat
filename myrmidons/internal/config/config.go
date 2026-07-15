package config

import (
	"flag"
	"os"
)

// Config holds all application configuration
type Config struct {
	// Network
	Addr       string
	DSN        string
	OnionAddr  string
	TorSocks   string

	// Identity
	SiteName   string

	// Monero
	MoneroRPC  string
	ViewKey    string
	SignKey    string

	// Market
	MarketSeed        string
	OperatorBond     string
	InsurancePercent float64
	DefaultFee       float64
	ZeroFeeThreshold int

	// Security
	EntryGuardEnabled bool
	CaptchaEnabled    bool
	DevMode          bool

	// Database
	CsrfAuthKey string
}

// Global config instance
var Cfg Config

// Define all configuration flags
func Define() {
	flag.StringVar(&Cfg.Addr, "addr", "127.0.0.1:4000", "HTTP listen address")
	flag.StringVar(&Cfg.DSN, "dsn", os.Getenv("DSN"), "PostgreSQL connection string")
	flag.StringVar(&Cfg.OnionAddr, "onion", os.Getenv("ONION_ADDRESS"), "Tor onion address")
	flag.StringVar(&Cfg.TorSocks, "tor-socks", "127.0.0.1:9050", "Tor SOCKS5 proxy address")
	flag.StringVar(&Cfg.SiteName, "name", "Myrmidons", "Site name")

	// Monero configuration
	flag.StringVar(&Cfg.MoneroRPC, "monero-rpc", os.Getenv("MONERO_RPC"), "Monero wallet RPC address")
	flag.StringVar(&Cfg.ViewKey, "view-key", os.Getenv("VIEW_KEY"), "Monero view key")
	flag.StringVar(&Cfg.SignKey, "sign-key", os.Getenv("SIGN_KEY"), "Monero sign key")

	// Market parameters
	flag.StringVar(&Cfg.MarketSeed, "market-seed", os.Getenv("MARKET_SEED"), "Market seed phrase")
	flag.StringVar(&Cfg.OperatorBond, "operator-bond", os.Getenv("OPERATOR_BOND"), "Operator bond amount in piconero")
	flag.Float64Var(&Cfg.InsurancePercent, "insurance-percent", 2.0, "Insurance fund percentage (0-100)")
	flag.Float64Var(&Cfg.DefaultFee, "default-fee", 5.0, "Default marketplace fee percentage")
	flag.IntVar(&Cfg.ZeroFeeThreshold, "zero-fee-threshold", 1000, "Volume threshold for zero fees")

	// Security flags
	flag.BoolVar(&Cfg.EntryGuardEnabled, "entry-guard", true, "Enable entry guard (jail/CAPTCHA)")
	flag.BoolVar(&Cfg.CaptchaEnabled, "captcha", true, "Enable CAPTCHA challenges")
	flag.BoolVar(&Cfg.DevMode, "dev", false, "Development mode")

	// Database
	flag.StringVar(&Cfg.CsrfAuthKey, "csrf-key", os.Getenv("CSRF_AUTH_KEY"), "CSRF authentication key")
}

// Parse command line flags
func Parse() {
	flag.Parse()
}

// Helper functions
func DevMode() bool { return Cfg.DevMode }
func CaptchaEnabled() bool { return Cfg.CaptchaEnabled }
func EntryGuardEnabled() bool { return Cfg.EntryGuardEnabled }
