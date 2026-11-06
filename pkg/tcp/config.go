package tcp

import "sync"

// ProxyConfig holds configuration for TCP proxy
type ProxyConfig struct {
	LocalAddr  string
	RemoteAddr string
	Verbose    bool
	Stop       bool
	mutex      sync.RWMutex
}

// NewProxyConfig creates a new proxy configuration
func NewProxyConfig(local, remote string) *ProxyConfig {
	return &ProxyConfig{
		LocalAddr:  local,
		RemoteAddr: remote,
		Verbose:    false,
		Stop:       false,
	}
}

// SetStop sets the stop flag
func (p *ProxyConfig) SetStop(stop bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.Stop = stop
}

// IsStop returns the stop flag
func (p *ProxyConfig) IsStop() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.Stop
}
