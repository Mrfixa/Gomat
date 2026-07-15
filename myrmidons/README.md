# Myrmidons

**Privacy-Preserving Marketplace** - A decentralized, anonymous marketplace built on trust minimization principles.

## Features

- 🔒 **Zero JavaScript** - Progressive enhancement with pure HTML/CSS
- 🛡️ **Monero Only** - Privacy-focused cryptocurrency payments
- 🌐 **Tor Integration** - Built-in onion service support
- 🌐 **10 Languages** - Full internationalization (EN, RU, DE, FR, ES, ZH, JA, PT, AR, IT)
- 🎨 **Dual Themes** - Dark and Light mode
- ⚖️ **Trust Minimization**:
  - Provable Reserves
  - Jury System
  - Insurance Fund
  - DAO Governance
  - Operator Bonds

## Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 15+
- Tor (optional, for onion services)

### Setup

```bash
# Clone and enter directory
cd myrmidons

# Copy environment file
cp .env.example .env
# Edit .env with your configuration

# Run migrations
migrate -path migrations -database "$DSN" up

# Build and run
make build
./myrmidons
```

### Docker

```bash
# Build and run
docker-compose up -d

# View logs
docker-compose logs -f app
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DSN` | PostgreSQL connection string | Required |
| `CSRF_AUTH_KEY` | CSRF signing key (32+ bytes) | Required |
| `ONION_ADDRESS` | Tor onion address | localhost |
| `TOR_SOCKS` | Tor SOCKS5 proxy | 127.0.0.1:9050 |
| `MONERO_RPC` | Monero wallet RPC | localhost:38083 |
| `DEV_MODE` | Development mode | false |

### Command Line Flags

```bash
./myrmidons --help
```

## Security

### OWASP Top 10 Compliance

| Vulnerability | Mitigation |
|--------------|------------|
| A01 - Broken Access Control | IDOR checks, authorization middleware |
| A02 - Cryptographic Failures | bcrypt, CSRF tokens, PGP 2FA |
| A03 - Injection | sqlc parameterized queries |
| A04 - Insecure Design | Advisory locks, rate limiting |
| A05 - Security Misconfiguration | Security headers, HSTS |
| A06 - Vulnerable Components | Dependency scanning |
| A07 - Auth Failures | Account lockout, IP tracking |
| A08 - Data Integrity | Path validation, checksums |
| A09 - Logging | Structured logging, correlation IDs |
| A10 - SSRF | Tor integration for RPC |

### Built-in Protections

- **bcrypt** password hashing with history (12 rounds)
- **Account lockout** after 5 failed attempts (15 min)
- **PGP 2FA** with RSA-OAEP encryption
- **CAPTCHA** from onion address recognition
- **Proof of Work** for bot mitigation (16-bit difficulty)
- **Rate limiting** per IP and session
- **CSRF tokens** on all forms
- **Parameterized SQL** queries (sqlc)
- **Advisory locks** for wallet operations
- **Security headers** (CSP, HSTS, X-Frame-Options)

### Security Headers

```
Content-Security-Policy: default-src 'self'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000
Referrer-Policy: same-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
Cache-Control: no-store, no-cache, must-revalidate
```

### Zero JavaScript

All pages work without JavaScript. Features use:
- CSS `:hover` and `:focus`
- HTML `<form>` with server-side validation
- CSS transitions and animations

## Architecture

### Core Components

```
myrmidons/
├── cmd/server/         # Application entry point
├── internal/
│   ├── app/          # HTTP handlers and routing
│   ├── config/       # Configuration management
│   ├── service/      # Business logic
│   │   ├── order/    # Order state machine
│   │   ├── dao/      # Governance
│   │   ├── reserves/ # Provable reserves
│   │   └── ...
│   ├── tor/          # Tor integration
│   └── translations/ # i18n
├── migrations/         # Database migrations
├── pkg/              # Shared packages
│   └── jail/         # Anti-bot protection
└── ui/               # Templates and styles
```

### Order State Machine

```
PENDING → PAID → ACCEPTED → DISPATCHED → DELIVERED → FINALIZED
              ↓         ↓
          DECLINED   DISPUTE → JURY → REFUNDED/FINALIZED
```

## Development

### Build

```bash
make build        # Production build
make build-dev    # Development build
make test         # Run tests
make test-coverage # With coverage
make fmt          # Format code
make vet          # Run vet
```

### Database

```bash
# Run migrations
make migrate-up

# Create new migration
make migrate-create NAME=add_feature

# Rollback
make migrate-down
```

### Docker

```bash
# Build image
make docker-build

# Run
make docker-run

# Push
make docker-push
```

## Testing

```bash
# Run all tests
make test

# With race detection
go test -race ./...

# Coverage report
make test-coverage
open coverage.html
```

## License

MIT License - See LICENSE file for details.

## Contributing

Contributions welcome! Please read our contributing guidelines before submitting PRs.

---

**Myrmidons**: Privacy-Preserving Marketplace
