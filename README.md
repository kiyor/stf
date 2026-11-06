# STF (Simple Transfer)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.19-blue)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Available-blue)](https://hub.docker.com/r/kiyor/stf)

STF is a versatile multi-protocol network utility written in Go that provides HTTP file server, SOCKS5 proxy, TCP proxy, HTTP tunnel, and test file generation capabilities in a single binary.

## 🚀 Features

- **HTTP File Server** - Serve files with upload capability and directory listing
- **SOCKS5 Proxy** - Full-featured SOCKS5 proxy with authentication and rate limiting
- **TCP Proxy** - Raw TCP connection proxying
- **HTTP Tunnel** - HTTP CONNECT tunnel proxy
- **Test File Generator** - Dynamic test files for bandwidth testing
- **Bridge Mode** - HTTP/HTTPS forwarding with automatic protocol detection
- **Authentication** - HTTP Basic Auth and SOCKS5 authentication
- **Rate Limiting** - Configurable bandwidth limits for SOCKS5
- **Security Features** - IP allow/deny lists, timeout controls

## 📦 Installation

### Pre-built Binary
Download from [releases](https://github.com/kiyor/stf/releases) or use Docker:

```bash
docker run --rm -p 30000:30000 kiyor/stf
```

### Build from Source
```bash
git clone https://github.com/kiyor/stf.git
cd stf
go build -v
```

## 🎯 Quick Start

### Basic HTTP File Server
```bash
# Serve current directory on port 30000
./stf

# Serve specific directory on custom port
./stf -d /path/to/files -p :8080

# Enable file uploads
./stf -uploadonly
```

Access your server at `http://localhost:30000`

### SOCKS5 Proxy
```bash
# Basic SOCKS5 proxy
./stf -socks5 -p :1080

# With authentication
echo "user:password" > auth.txt
./stf -socks5 -socks5auth auth.txt -p :1080

# Configure your applications to use SOCKS5 proxy at localhost:1080
```

### HTTP Tunnel Proxy
```bash
# Start HTTP tunnel
./stf -tunnel -p :8888

# Configure applications to use HTTP proxy at localhost:8888
```

## 📖 Detailed Usage

### HTTP File Server Mode

The default mode serves files from a directory with optional upload capability.

#### Basic File Serving
```bash
# Serve current directory
./stf

# Serve specific directory
./stf -d /var/www/html

# Custom port
./stf -p :8080

# Disable gzip compression
./stf -gzip-disable
```

#### File Upload
```bash
# Enable upload-only mode (only POST/PUT requests)
./stf -uploadonly

# Upload files using curl
curl -X POST -F "file=@example.txt" http://localhost:30000/upload
```

#### Authentication
```bash
# Simple user:password format
./stf -httpauth "user:secret123"

# From file (one user:pass per line)
echo -e "admin:password\nuser:123456" > auth.txt
./stf -httpauth auth.txt

# JSON format
echo '{"users": {"admin": "secret", "user": "pass"}}' > auth.json
./stf -httpauth auth.json
```

#### Security & Access Control
```bash
# Allow specific IP ranges
./stf -allow 192.168.1.0/24 -allow 10.0.0.1

# Deny specific IPs
./stf -deny 1.2.3.4 -deny 5.6.7.8

# Custom timeout (default 5 minutes)
./stf -timeout 10m

# Disable timeout
./stf -notimeout
```

#### Advanced Options
```bash
# Verbose logging
./stf -v          # Basic verbose
./stf -vv         # Very verbose (includes request/response body)
./stf -body       # Show request/response body content
./stf -c          # Colorful output

# Debug responses
./stf -return 404          # Always return 404
./stf -3xx "301 https://example.com"  # Redirect all requests

# Custom gzip types
./stf -gzip-types "text/html application/json"
```

#### Photo Browser (Gallery)

Every directory can be opened in a built-in, ACDSee-style photo browser — no extra flags,
the page is embedded in the binary. The plain listing shows a **Gallery (N)** link when the
folder contains images, or append `?gallery` to any directory URL:

```bash
./stf -d ~/Pictures
open "http://localhost:30000/?gallery"
```

- Thumbnail grid with folder pane, breadcrumbs, natural sort (name/date/size), filter, adjustable thumbnail size
- Viewer: fit / 1:1 / wheel & pinch zoom, drag to pan, rotate, filmstrip, properties panel, slideshow, full screen
- Keyboard: arrows, Enter, Space/PgUp/PgDn, Home/End, `+`/`-`/`0`/`1`, `R`, `I`, `T`, `F`, `S`, Esc, Backspace (press `?` in the page)
- Deep links: `/photos/?gallery#view=IMG_0001.jpg`; browser Back closes the viewer
- Server-side thumbnails: `<image>?thumb=256` returns a JPEG (longest edge 32–1600, EXIF orientation applied,
  128 MB in-memory LRU, ETag/304). JPEG/PNG/GIF/WebP/BMP are thumbnailed; SVG/AVIF/HEIC fall back to the original
- `?gallery=json` returns the folder listing as JSON; `-noindex` disables the gallery like the listing

### SOCKS5 Proxy Mode

STF provides a full-featured SOCKS5 proxy server with authentication and rate limiting.

#### Basic SOCKS5 Proxy
```bash
# Start SOCKS5 proxy on port 1080
./stf -socks5 -p :1080

# Custom port
./stf -socks5 -p :9050
```

#### Authentication
```bash
# File-based auth (username:password per line)
echo -e "user1:pass1\nuser2:pass2" > socks_auth.txt
./stf -socks5 -socks5auth socks_auth.txt

# JSON format
echo '{"users": {"proxy_user": "secret123"}}' > socks_auth.json
./stf -socks5 -socks5auth socks_auth.json
```

#### Rate Limiting
```bash
# Limit upload to 1MB/s, download to 5MB/s
./stf -socks5 -socks5in 1048576 -socks5out 5242880

# No upload limit, 10MB/s download limit
./stf -socks5 -socks5out 10485760
```

#### Proxy Chaining
```bash
# Chain through another SOCKS5 proxy
./stf -socks5 -socks5next upstream.proxy.com:1080

# Chain with authentication
./stf -socks5 -socks5next proxy.com:1080:user:pass

# Disable DNS resolution (let upstream handle it)
./stf -socks5 -socks5next proxy.com:1080 -socks5noresolver
```

#### Hosts File Override
```bash
# Custom hosts resolution
echo -e "example.com 127.0.0.1\ntest.local 192.168.1.100" > hosts.txt
./stf -socks5 -socks5hosts hosts.txt
```

### TCP Proxy Mode

Forward raw TCP connections to an upstream server.

```bash
# Proxy TCP connections to backend server
./stf -upstream backend.example.com:3306 -p :3306

# Proxy with IP restrictions
./stf -upstream 10.0.0.100:5432 -allow 192.168.1.0/24 -p :5432
```

### HTTP Tunnel Mode

Create an HTTP CONNECT tunnel proxy for HTTPS and other protocols.

```bash
# Start HTTP tunnel proxy
./stf -tunnel -p :8888

# With authentication
./stf -tunnel -httpauth "proxy_user:secret" -p :8888

# Configure applications to use HTTP proxy at localhost:8888
```

### Bridge Mode

Automatically forward HTTP and HTTPS traffic to different upstream servers.

```bash
# Forward to single backend (auto-detects HTTP/HTTPS)
./stf -bridge backend.internal.com

# Forward HTTP to one server, HTTPS to another
./stf -bridge "http.internal.com:https.internal.com"

# Custom ports
./stf -bridge "internal.com:192.168.1.100" -p :80

# With SSL certificates for HTTPS termination
./stf -bridge internal.com -crt server.crt -key server.key
```

### Test File Generator

Generate files of specific sizes for bandwidth and performance testing.

```bash
# Enable test file generation
./stf -testfile -p :8080

# Access test files:
# http://localhost:8080/1K    - 1 kilobyte file
# http://localhost:8080/10M   - 10 megabyte file
# http://localhost:8080/1G    - 1 gigabyte file

# Custom cache control
./stf -testfile -testcc "no-cache"

# Enable bandwidth counter
./stf -testfile -testc

# With HTTP Range support for partial downloads
curl -H "Range: bytes=0-1023" http://localhost:8080/10M
```

#### Test File URLs
- `/1K` to `/999K` - Kilobyte files
- `/1M` to `/999M` - Megabyte files  
- `/1G` to `/999G` - Gigabyte files
- `/1T` to `/999T` - Terabyte files

## 🔧 Configuration Examples

### Complete Web Server Setup
```bash
# Production web server with auth and security
./stf -d /var/www \
      -p :80 \
      -httpauth auth.txt \
      -allow 0.0.0.0/0 \
      -deny 192.168.100.0/24 \
      -timeout 30m \
      -v
```

### Corporate Proxy Setup
```bash
# SOCKS5 proxy with rate limiting and auth
./stf -socks5 \
      -p :1080 \
      -socks5auth corporate_users.txt \
      -socks5in 5242880 \
      -socks5out 10485760 \
      -allow 192.168.0.0/16 \
      -v
```

### Development Testing Server
```bash
# Test server with uploads and test files
./stf -d ./public \
      -uploadonly \
      -testfile \
      -testc \
      -p :3000 \
      -vv \
      -c
```

### Load Balancer Bridge
```bash
# Bridge mode for load balancing
./stf -bridge "app1.internal:app2.internal" \
      -p :80 \
      -allow 10.0.0.0/8 \
      -timeout 60s \
      -v
```

## 🐳 Docker Usage

### Basic File Server
```bash
# Serve current directory
docker run --rm -p 30000:30000 -v $(pwd):/data kiyor/stf -d /data

# With custom configuration
docker run --rm -p 8080:8080 -v $(pwd):/data kiyor/stf -d /data -p :8080 -uploadonly
```

### SOCKS5 Proxy
```bash
# SOCKS5 proxy with authentication
echo "user:pass" > auth.txt
docker run --rm -p 1080:1080 -v $(pwd)/auth.txt:/auth.txt kiyor/stf -socks5 -socks5auth /auth.txt -p :1080
```

### Persistent Data with Docker Compose
```yaml
version: '3.8'
services:
  stf:
    image: kiyor/stf
    ports:
      - "30000:30000"
    volumes:
      - ./files:/data
      - ./auth.txt:/auth.txt
    command: [-d, /data, -httpauth, /auth.txt, -uploadonly, -v]
    restart: unless-stopped
```

## 📊 Performance & Monitoring

### Verbose Logging
```bash
# Basic request logging
./stf -v

# Detailed request/response logging
./stf -vv

# Log to file with timestamps
./stf -v 2>&1 | tee stf.log
```

### Debug Mode
```bash
# Dump requests to /tmp/stfdump/ (with -v or -vv)
./stf -vv  # Creates detailed dumps in /tmp/stfdump/

# Monitor connections and bandwidth
./stf -socks5 -v  # Shows connection statistics
```

## 🔐 Security Best Practices

### Authentication
- Use strong passwords in auth files
- Regularly rotate credentials
- Prefer JSON format for complex auth setups

### Network Security
```bash
# Restrict access to trusted networks
./stf -allow 192.168.1.0/24 -allow 10.0.0.0/8 -deny 0.0.0.0/0

# Use reasonable timeouts
./stf -timeout 5m
```

### File Server Security
```bash
# Serve specific directory only
./stf -d /var/www/public

# Enable authentication for sensitive content
./stf -httpauth "admin:$(openssl rand -base64 32)"
```

## 🚨 Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Check what's using the port
sudo netstat -tlnp | grep :30000

# Use different port
./stf -p :30001
```

**Permission denied:**
```bash
# For ports < 1024, run as root or use higher port
sudo ./stf -p :80
# or
./stf -p :8080
```

**Connection timeouts:**
```bash
# Increase timeout or disable it
./stf -timeout 10m
# or
./stf -notimeout
```

**File upload issues:**
```bash
# Ensure upload mode is enabled and directory is writable
./stf -uploadonly -d /tmp/uploads
chmod 755 /tmp/uploads
```

### Debug Mode
```bash
# Enable maximum verbosity
./stf -vv -body -c

# Check configuration
./stf -version
```

### Log Analysis
```bash
# Monitor real-time logs
./stf -v | grep -E "(ERROR|WARN|INFO)"

# Log to file with rotation
./stf -v 2>&1 | rotatelogs stf_%Y%m%d.log 86400
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch: `git checkout -b feature/amazing-feature`
3. Commit your changes: `git commit -m 'Add amazing feature'`
4. Push to the branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with Go and standard libraries
- Uses `github.com/NYTimes/gziphandler` for compression
- SOCKS5 implementation based on `github.com/kiyor/go-socks5`
- Rate limiting via `github.com/juju/ratelimit`

## 📞 Support

- 🐛 **Bug Reports:** [GitHub Issues](https://github.com/kiyor/stf/issues)
- 💡 **Feature Requests:** [GitHub Discussions](https://github.com/kiyor/stf/discussions)
- 📖 **Documentation:** [Wiki](https://github.com/kiyor/stf/wiki)

---

**STF** - Simple, powerful, versatile. One binary, multiple protocols. 🚀