package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/juju/ratelimit"
	gosocks5 "github.com/kiyor/go-socks5"
	"github.com/kiyor/stf/pkg/common/auth"
	"github.com/kiyor/stf/pkg/common/middleware"
	"github.com/kiyor/stf/pkg/config"
	stfhttp "github.com/kiyor/stf/pkg/http"
	stfsocks5 "github.com/kiyor/stf/pkg/socks5"

	// "github.com/kiyor/stf/pkg/tcp" // Disabled due to flag conflicts

	// "github.com/kiyor/stf/pkg/tcp" // Disabled due to flag conflicts
	"github.com/viki-org/dnscache"
	"golang.org/x/net/proxy"
)

var (
	fdir     *string = flag.String("d", ".", "Mount Dir")
	fport    *string = flag.String("p", ":30000", "Listening Port")
	upstream *string = flag.String("upstream", "scheme://ip:port or ip:port", "setup proxy")
	next     *string = flag.String("next", "[ip:port](:user:pass)", "proxy mode with socks5 proxy to upstream")

	httpAuthFlag *string = flag.String("httpauth", "", "http base auth mode, import txt/json/string")

	sock              *bool   = flag.Bool("socks5", false, "socks5 mode")
	sockAuth          *string = flag.String("socks5auth", "", "socks5 auth mode, import txt/json/string")
	sockHosts         *string = flag.String("socks5hosts", "", "socks5 hosts file")
	sockNext          *string = flag.String("socks5next", "", "socks5 proxy chan next point")
	sockNoResolver    *bool   = flag.Bool("socks5noresolver", false, "socks5 without resolver for proxy chan")
	socksInRateLimit          = flag.Int64("socks5in", math.MaxInt64, "socks5 max input ratelimit (byte upload)")
	socksOutRateLimit         = flag.Int64("socks5out", math.MaxInt64, "socks5 max input ratelimit (byte download)")

	httpTunnel *bool = flag.Bool("tunnel", false, "http tunnel mode")
	uploadonly *bool = flag.Bool("uploadonly", false, "upload only POST/PUT")
	showBody   *bool = flag.Bool("body", false, "show body")

	testFile       *bool   = flag.Bool("testfile", false, "testfile, /1(K/M/G)")
	testFileCC     *string = flag.String("testcc", "public,max-age=60", "testfile Cache-Control Header")
	testDisCounter *bool   = flag.Bool("testc", false, "disable test counter")
	noindex        *bool   = flag.Bool("noindex", false, "disable directory listing, require exact file path")

	bridge *string = flag.String("bridge", "host/ip/host:ip", "quick setup http/+https proxy with upstream 80/+443")
	crt    *string = flag.String("crt", "", "crt location if using brdige mode")
	key    *string = flag.String("key", "", "key location if using brdige mode")

	tlsEnable *bool   = flag.Bool("tls", false, "enable HTTPS mode with auto-generated self-signed certificate")
	tlsCert   *string = flag.String("tlscert", "", "path to custom TLS certificate file (requires -tlskey)")
	tlsKey    *string = flag.String("tlskey", "", "path to custom TLS private key file (requires -tlscert)")

	version *bool = flag.Bool("version", false, "output version and exit")

	rt      = flag.Int("return", -1, "debug test return code")
	threexx = flag.String("3xx", "", "301/302 https://{{.Host}}{{.URL}}")

	timeout   *time.Duration = flag.Duration("timeout", 5*time.Minute, "timeout")
	notimeout                = flag.Bool("notimeout", false, "no timeout")

	disableGzip = flag.Bool("gzip-disable", false, "disable gzip, default gzip = on")
	gzipTypes   = flag.String("gzip-types", "text/html text/plain text/css text/javascript text/xml application/json application/javascript application/x-javascript application/xml application/atom+xml application/rss+xml application/vnd.ms-fontobject application/x-font-ttf font/opentype font/x-woff", "gzip type")

	verbose  = flag.Bool("v", false, "verbose mode")
	vverbose = flag.Bool("vv", false, "very verbose mode")
	usecolor = flag.Bool("c", false, "colorful output")

	// Global timeout control variables
	shutdownTimer      *time.Timer
	forceShutdownTimer *time.Timer
	stop               bool
	wg                 = new(sync.WaitGroup)
	shutdownChan       = make(chan bool, 1)
	ch                 = make(chan bool, 1)
	shutdownOnce       sync.Once
)

// getListeningAddresses returns all possible listening addresses for the given port
func getListeningAddresses(port string) []string {
	var addresses []string

	// Parse port from string like ":30000" or "127.0.0.1:30000"
	portOnly := port
	if strings.HasPrefix(port, ":") {
		portOnly = port[1:]
	} else if strings.Contains(port, ":") {
		_, portOnly, _ = net.SplitHostPort(port)
	}

	// Get all network interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Failed to get network interfaces: %v", err)
		return []string{port}
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ip := ipnet.IP

			// Skip loopback and IPv6 addresses
			if ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			addresses = append(addresses, fmt.Sprintf("%s:%s", ip.String(), portOnly))
		}
	}

	// If no valid addresses found, return the original port
	if len(addresses) == 0 {
		addresses = append(addresses, port)
	}

	return addresses
}

// trackedConn wraps a net.Conn to track when it's closed
type trackedConn struct {
	net.Conn
	wg   *sync.WaitGroup
	once sync.Once
}

func (tc *trackedConn) Close() error {
	tc.once.Do(func() {
		if tc.wg != nil {
			tc.wg.Done()
		}
	})
	return tc.Conn.Close()
}

func init() {
	flag.Var(&config.FlagAllowIP, "allow", "allow ip")
	flag.Var(&config.FlagDenyIP, "deny", "deny ip")
}

func main() {
	flag.Parse()

	if *version {
		fmt.Println("STF version 2.0.0 (refactored)")
		os.Exit(0)
	}

	// Start shutdown timer if not in no-timeout mode
	if !*notimeout {
		startShutdownTimer()
	}

	// Determine which mode to run
	if *sock {
		// SOCKS5 proxy mode
		addresses := getListeningAddresses(*fport)
		log.Println("Starting SOCKS5 proxy server:")
		for _, addr := range addresses {
			log.Printf("  socks5://%s", addr)
		}
		startSocks5Server()
	} else if *httpTunnel {
		// HTTP tunnel mode
		addresses := getListeningAddresses(*fport)
		log.Println("Starting HTTP tunnel server:")
		for _, addr := range addresses {
			log.Printf("  http://%s", addr)
		}
		startHTTPTunnel()
	} else if *upstream != "" && *upstream != "scheme://ip:port or ip:port" {
		// TCP proxy mode
		addresses := getListeningAddresses(*fport)
		log.Printf("Starting TCP proxy server to %s:", *upstream)
		for _, addr := range addresses {
			log.Printf("  tcp://%s", addr)
		}
		startTCPProxy()
	} else {
		// Default: HTTP file server
		startHTTPServer()
	}
}

// startShutdownTimer starts the automatic shutdown timer
func startShutdownTimer() {
	log.Printf("Auto-shutdown enabled: will stop accepting new connections after %v", *timeout)

	// Start the timeout timer
	shutdownTimer = time.AfterFunc(*timeout, func() {
		log.Println("Timeout reached, blocking new connections")
		stop = true

		// Wait for existing connections to finish (max 1 hour)
		forceShutdownTimer = time.AfterFunc(time.Hour, func() {
			log.Println("Force shutdown after 1 hour")
			shutdownOnce.Do(func() {
				close(shutdownChan)
			})
		})

		// Start a goroutine to wait for all connections to close
		go func() {
			wg.Wait()
			log.Println("All connections closed gracefully")
			if forceShutdownTimer != nil {
				forceShutdownTimer.Stop()
			}
			shutdownOnce.Do(func() {
				close(shutdownChan)
			})
		}()
	})

	// Start a goroutine to reset timer on activity and handle shutdown
	go func() {
		for {
			select {
			case <-ch:
				// Reset timer when activity is detected
				if shutdownTimer != nil && !stop {
					shutdownTimer.Reset(*timeout)
				}
			case <-shutdownChan:
				// Give servers a moment to close gracefully
				time.Sleep(100 * time.Millisecond)
				log.Println("Shutting down...")
				os.Exit(0)
			}
		}
	}()
}

func startSocks5Server() {
	conf := &gosocks5.Config{}
	conf.Resolver = &stfsocks5.Resolver{Resolver: dnscache.New(time.Minute * 5)}
	conf.Rewriter = &stfsocks5.Rewriter{}
	conf.Rules = &stfsocks5.FireWallRuleSet{}

	// Store original dial function
	var originalDial func(ctx context.Context, net_, addr string) (net.Conn, error)
	if conf.Dial != nil {
		originalDial = conf.Dial
	} else {
		originalDial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial(network, addr)
		}
	}

	// Wrap the dial function to check stop flag
	conf.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if stop {
			return nil, fmt.Errorf("server is shutting down")
		}
		wg.Add(1)
		conn, err := originalDial(ctx, network, addr)
		if err != nil {
			wg.Done()
			return nil, err
		}
		// Keep timer alive
		if !*notimeout {
			select {
			case ch <- true:
			default:
			}
		}
		// Wrap connection to track when it closes
		return &trackedConn{Conn: conn, wg: wg}, nil
	}

	// Set up rate limiting
	if *socksInRateLimit != math.MaxInt64 {
		conf.InBucket = ratelimit.NewBucketWithRate(float64(*socksInRateLimit), *socksInRateLimit)
	}
	if *socksOutRateLimit != math.MaxInt64 {
		conf.OutBucket = ratelimit.NewBucketWithRate(float64(*socksOutRateLimit), *socksOutRateLimit)
	}

	// Set up proxy chaining if configured
	if *sockNext != "" {
		var a *proxy.Auth
		p := strings.Split(*sockNext, ":")
		if len(p) > 2 {
			a = new(proxy.Auth)
			a.User = p[2]
			a.Password = p[3]
			*sockNext = strings.Join(p[:2], ":")
		}
		dialer, err := proxy.SOCKS5("tcp", *sockNext, a, &net.Dialer{
			KeepAlive: 30 * time.Second,
		})
		if err != nil {
			log.Fatal("Failed to create SOCKS5 dialer:", err)
		}
		originalDial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	}

	// Set up authentication if configured
	if *sockAuth != "" {
		cred := stfsocks5.ParseSocks5Auth(*sockAuth)
		if len(cred) > 0 {
			cator := gosocks5.UserPassAuthenticator{Credentials: cred}
			conf.AuthMethods = []gosocks5.Authenticator{cator}
		}
	}

	// Set up finalizer for logging
	conf.Finalizer = &stfsocks5.LogFinalizer{
		Log: log.New(os.Stdout, "[socks5] ", log.LstdFlags),
	}

	// Disable resolver if requested
	if *sockNoResolver {
		conf.Resolver = nil
	}

	server, err := gosocks5.New(conf)
	if err != nil {
		log.Fatal("Failed to create SOCKS5 server:", err)
	}

	// Create listener
	ln, err := net.Listen("tcp", *fport)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}
	defer ln.Close()

	// Start accepting connections in a goroutine
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if stop {
					return
				}
				log.Printf("Accept error: %v", err)
				continue
			}

			// Check if we're stopping
			if stop {
				conn.Close()
				continue
			}

			// Keep timer alive
			if !*notimeout {
				select {
				case ch <- true:
				default:
				}
			}

			// Track connection
			wg.Add(1)
			go func() {
				defer wg.Done()
				server.ServeConn(conn)
			}()
		}
	}()

	// Wait for shutdown
	<-shutdownChan
}

func startHTTPTunnel() {
	proxy := stfhttp.NewProxy(*next)

	// Wrap the proxy handler to check stop flag
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if we're stopping
		if stop {
			http.Error(w, "Service Unavailable - Server is shutting down", http.StatusServiceUnavailable)
			return
		}

		// Track connection
		wg.Add(1)
		defer wg.Done()

		// Keep timer alive if not in notimeout mode
		if !*notimeout {
			select {
			case ch <- true:
			default:
			}
		}

		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    *fport,
		Handler: handler,
	}

	if !*notimeout {
		server.ReadTimeout = *timeout
		server.WriteTimeout = *timeout
	}

	// Start server in a goroutine so we can handle shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP tunnel failed:", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdownChan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP tunnel shutdown error: %v", err)
	}
}

func startTCPProxy() {
	log.Printf("TCP proxy mode not yet fully integrated from %s to %s", *fport, *upstream)
	log.Printf("Use HTTP tunnel mode instead with -tunnel flag")
	log.Fatal("TCP proxy mode disabled due to flag conflicts - use HTTP tunnel instead")
}

// getTLSCertDir returns the directory where TLS certificates are stored
func getTLSCertDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: Unable to get home directory, using current directory for TLS certs: %v", err)
		return ".stf"
	}
	return filepath.Join(homeDir, ".config", "stf")
}

// generateSelfSignedCert generates a self-signed certificate and private key
func generateSelfSignedCert(certPath, keyPath string) error {
	// Create certificate directory if it doesn't exist
	dir := filepath.Dir(certPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %v", err)
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	// Save certificate
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %v", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %v", err)
	}

	// Save private key
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	defer keyOut.Close()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	log.Printf("Generated self-signed certificate: %s", certPath)
	return nil
}

// getTLSCertificates returns the path to TLS certificate and key files
// If custom cert/key are provided via flags, use those; otherwise use auto-generated ones
func getTLSCertificates() (string, string, error) {
	// If custom certificates are specified, use them
	if *tlsCert != "" && *tlsKey != "" {
		// Validate that both files exist
		if _, err := os.Stat(*tlsCert); err != nil {
			return "", "", fmt.Errorf("custom certificate file not found: %v", err)
		}
		if _, err := os.Stat(*tlsKey); err != nil {
			return "", "", fmt.Errorf("custom key file not found: %v", err)
		}
		log.Printf("Using custom TLS certificate: %s", *tlsCert)
		return *tlsCert, *tlsKey, nil
	}

	// Use auto-generated certificates
	certDir := getTLSCertDir()
	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")

	// Check if certificate files exist and are valid
	needGenerate := false
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		needGenerate = true
	} else if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		needGenerate = true
	} else {
		// Check if certificate is expired or will expire soon
		certPEM, err := os.ReadFile(certPath)
		if err == nil {
			block, _ := pem.Decode(certPEM)
			if block != nil {
				cert, err := x509.ParseCertificate(block.Bytes)
				if err == nil {
					// Regenerate if expires within 30 days
					if time.Until(cert.NotAfter) < 30*24*time.Hour {
						needGenerate = true
					}
				} else {
					needGenerate = true
				}
			} else {
				needGenerate = true
			}
		} else {
			needGenerate = true
		}
	}

	if needGenerate {
		if err := generateSelfSignedCert(certPath, keyPath); err != nil {
			return "", "", err
		}
	} else {
		log.Printf("Using existing self-signed certificate: %s", certPath)
	}

	return certPath, keyPath, nil
}

func startHTTPServer() {
	// Initialize file system for serving files
	fs := stfhttp.Dir(*fdir)

	// Main handler that routes different request types
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if we're stopping
		if stop {
			http.Error(w, "Service Unavailable - Server is shutting down", http.StatusServiceUnavailable)
			return
		}

		// Track connection
		wg.Add(1)
		defer wg.Done()

		// Keep timer alive if not in notimeout mode
		if !*notimeout {
			select {
			case ch <- true:
			default:
			}
		}
		if *rt != -1 {
			// Debug mode: return specific status code
			w.WriteHeader(*rt)
			return
		}

		if len(*threexx) != 0 {
			// Handle redirects
			handleRedirect(w, r, *threexx)
			return
		}

		w.Header().Add("Connection", "Keep-Alive")

		if (r.Method == "GET" || r.Method == "HEAD") && !*uploadonly && !*testFile {
			// File serving
			if len(r.Header.Get("X-Cache-Control")) > 0 {
				w.Header().Add("Cache-Control", r.Header.Get("X-Cache-Control"))
			} else {
				w.Header().Add("Cache-Control", "no-cache")
			}
			fileHandler := stfhttp.FileServerWithOptions(fs, *noindex)
			fileHandler.ServeHTTP(w, r)
		} else if *testFile {
			// Test file generation
			stfhttp.TestFileHandler(w, r, *testFileCC)
		} else if r.Method == "POST" || r.Method == "PUT" {
			// File upload
			uploadHandler(w, r)
		}
	})

	// Apply middleware layers
	var h http.Handler = handler

	// Add HTTP authentication if configured
	if len(*httpAuthFlag) > 0 {
		h = auth.HTTPAuth(*httpAuthFlag)(h)
	}

	// Add gzip compression if not disabled
	if !*disableGzip {
		gzipHandler, err := gziphandler.GzipHandlerWithOpts(
			gziphandler.MinSize(512),
			gziphandler.CompressionLevel(-1),
		)
		if err != nil {
			log.Fatal("Failed to create gzip handler:", err)
		}
		h = gzipHandler(h)
	}

	// Add logging middleware
	h = middleware.LogHandlerWithColors(h, *usecolor)

	server := &http.Server{
		Addr:    *fport,
		Handler: h,
	}

	if !*notimeout {
		server.ReadTimeout = *timeout
		server.WriteTimeout = *timeout
	}

	// Determine server type and configure TLS if needed
	var protocol string = "http"
	var serverType string = "HTTP"

	if *tlsEnable {
		certPath, keyPath, err := getTLSCertificates()
		if err != nil {
			log.Fatalf("Failed to get TLS certificates: %v", err)
		}

		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			log.Fatalf("Failed to load TLS certificates: %v", err)
		}

		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		protocol = "https"
		serverType = "HTTPS"
	}

	// Print startup information
	addresses := getListeningAddresses(*fport)
	log.Printf("Starting %s file server:", serverType)
	for _, addr := range addresses {
		log.Printf("  %s://%s", protocol, addr)
	}
	log.Printf("Serving directory: %s", *fdir)

	// Start server in a goroutine so we can handle shutdown
	go func() {
		var err error
		if *tlsEnable {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("Server error: ", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdownChan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}

// handleRedirect handles HTTP redirects based on template
func handleRedirect(w http.ResponseWriter, r *http.Request, redirectTemplate string) {
	code := 302
	var urlTemplate string

	// Check if code is specified
	if len(redirectTemplate) >= 3 {
		if codeStr := redirectTemplate[:3]; strings.ContainsRune("0123456789", rune(codeStr[0])) {
			if c, err := strconv.Atoi(codeStr); err == nil && c >= 300 && c < 400 {
				code = c
				if len(redirectTemplate) > 4 {
					urlTemplate = redirectTemplate[4:]
				}
			} else {
				urlTemplate = redirectTemplate
			}
		} else {
			urlTemplate = redirectTemplate
		}
	}

	// Parse and execute template
	tmpl, err := template.New("redirect").Parse(urlTemplate)
	if err != nil {
		log.Printf("Error parsing redirect template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, r); err != nil {
		log.Printf("Error executing redirect template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, buf.String(), code)
}

// uploadHandler handles file uploads
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Clean the URL path to avoid issues with leading slashes
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	if urlPath == "" {
		urlPath = "index.html" // default file name for root path
	}
	p := filepath.Join(*fdir, urlPath)
	d, _ := filepath.Split(p)

	// Ensure directory exists (only if there's a directory component)
	if d != "" && d != "." {
		if err := os.MkdirAll(d, 0755); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create directory: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Create the file
	out, err := os.Create(p)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to create file: %v", err), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	// Copy request body to file
	_, err = io.Copy(out, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File uploaded successfully: %s\n", p)
}
