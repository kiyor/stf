# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**STF (Simple Transfer)** - A versatile multi-protocol network utility written in Go that provides:
- HTTP/HTTPS file server with upload capability
- SOCKS5 proxy server with authentication and rate limiting
- TCP proxy for raw connections
- HTTP tunnel proxy
- Test file generation for bandwidth testing
- Bridge mode for HTTP/HTTPS forwarding

**Refactored Structure (2025-01-10)**:
```
pkg/
├── common/        # Shared utilities
│   ├── auth/      # HTTP authentication
│   ├── middleware/# Request/response middleware
│   └── utils/     # Common utilities
├── config/        # Configuration management
├── http/          # HTTP server and handlers
├── socks5/        # SOCKS5 proxy implementation
└── tcp/           # TCP proxy functionality
```

## Architecture Philosophy

- **大而全 (Comprehensive)**: Single binary providing multiple network utilities
- **性能优先**: Direct TCP/HTTP handling, concurrent request processing with goroutines
- **简单易读**: Clear flag-based configuration, no complex config files
- **单体架构**: All functionality in a single deployable binary

## Build and Development

### Build the project
```bash
go build -v
```

### Run tests
```bash
go test ./...
```

### Check code quality
```bash
go vet ./...
```

### Common Issues
- `tcproxy.go` has Printf formatting issues in log.Println calls (lines 51, 56, 70)

## Timeout and Auto-shutdown Feature

STF includes an automatic timeout feature that helps prevent resource waste:
- **Default behavior**: Server automatically stops accepting new connections after 5 minutes
- **Graceful shutdown**: Existing connections are allowed to complete (up to 1 hour)
- **Activity tracking**: Timer resets on each new request
- **Disable timeout**: Use `-notimeout` flag for permanent service

### Timeout Workflow
1. Server starts with 5-minute timer (configurable via `-timeout`)
2. Each request resets the timer
3. When timeout expires:
   - New connections are rejected (HTTP 503)
   - Existing connections continue
   - Server waits for all connections to close
4. After 1 hour maximum, force shutdown occurs

### Examples
```bash
# Default 5-minute timeout
./stf

# Custom 10-second timeout (for testing)
./stf -timeout 10s

# Permanent service (no auto-shutdown)
./stf -notimeout
```

## Key Commands and Usage

### Basic HTTP File Server
```bash
# Serve current directory on port 30000
./stf

# Serve specific directory on custom port
./stf -d /path/to/dir -p :8080

# Enable upload capability
./stf -uploadonly
```

### SOCKS5 Proxy Mode
```bash
# Basic SOCKS5 proxy
./stf -socks5 -p :1080

# With authentication (user:pass format in file)
./stf -socks5 -socks5auth auth.txt

# With rate limiting (bytes/sec)
./stf -socks5 -socks5in 1000000 -socks5out 1000000

# Chain to another SOCKS5 proxy
./stf -socks5 -socks5next proxy.example.com:1080
```

### HTTP Proxy/Tunnel Mode
```bash
# HTTP tunnel mode
./stf -tunnel -p :8888

# Upstream proxy mode
./stf -upstream http://backend:8080 -p :3000

# Bridge mode (forward to specific host)
./stf -bridge example.com:192.168.1.100
```

### Test File Generation
```bash
# Generate test files of specific sizes
# /1M generates 1MB, /100K generates 100KB, /5G generates 5GB
./stf -testfile -p :8080

# With custom cache control
./stf -testfile -testcc "no-cache"

# With bandwidth counter
./stf -testfile -testc
```

### Security Features
```bash
# HTTP Basic Auth
./stf -httpauth user:pass
./stf -httpauth auth.json

# IP Allow/Deny lists
./stf -allow 192.168.1.0/24 -allow 10.0.0.1
./stf -deny 1.2.3.4
```

### Advanced Options
```bash
# Custom timeout (default 5 minutes, auto-shutdown after inactivity)
./stf -timeout 10m

# No timeout (permanent service mode)
./stf -notimeout

# Return specific HTTP status code (debug)
./stf -return 404

# Redirect responses
./stf -3xx "301 https://example.com"

# Show request/response details
./stf -v    # verbose
./stf -vv   # very verbose with body
./stf -body # show body content
./stf -c    # enable colors
```

## Core Components (Refactored)

### Entry Points
- `main.go` - Main server initialization and mode selection
- `pkg/socks5/server.go` - SOCKS5 proxy implementation
- `pkg/tcp/proxy.go` - TCP proxy for non-HTTP connections
- `pkg/http/tunnel.go` - HTTP CONNECT tunnel proxy
- `pkg/http/testfile.go` - Dynamic test file generation with range support

### Key Handlers (in packages)
- `fileHandler` (pkg/http/fileserver.go) - Static file serving with directory listing
- `uploadHandler` - File upload processing (to be implemented)
- `testFileHandler` (pkg/http/testfile.go) - Dynamic content generation
- `LogHandler` (pkg/common/middleware/log_handler.go) - Request/response logging middleware

### Package Structure
- `pkg/config/` - Configuration and flag management
- `pkg/http/` - HTTP server, file serving, tunnel, test files
- `pkg/socks5/` - SOCKS5 proxy server
- `pkg/tcp/` - TCP proxy and connection handling
- `pkg/common/auth/` - Authentication utilities
- `pkg/common/middleware/` - HTTP middleware
- `pkg/common/utils/` - Shared utilities

## Dependencies

Key external dependencies:
- `github.com/NYTimes/gziphandler` - Automatic gzip compression
- `github.com/kiyor/go-socks5` - SOCKS5 server implementation
- `github.com/juju/ratelimit` - Token bucket rate limiting
- `github.com/viki-org/dnscache` - DNS caching
- `golang.org/x/net/proxy` - SOCKS5 client for chaining

## Development Tips

1. The project uses a timeout mechanism - all handlers must update the `ch` channel to prevent auto-shutdown
2. Request processing uses goroutines with `sync.WaitGroup` for concurrent handling
3. Verbose flags (`-v`, `-vv`) dump requests/responses to `/tmp/stfdump/` for debugging
4. The test file handler supports HTTP Range requests for partial content
5. Bridge mode automatically detects HTTP vs HTTPS based on `req.TLS`