package api

import (
	"sync"
)

// HTTPClientPool manages a pool of HTTPClient instances for concurrent requests
type HTTPClientPool struct {
	clients     []*HTTPClient
	pool        chan *HTTPClient
	mu          sync.RWMutex
	size        int
	apiBase     string
	headers     string
	debug       bool
	reloginFunc ReloginFunc

	// Shared state across all clients
	bearer       string
	cryptoKeyHex string
}

// NewHTTPClientPool creates a new pool with the specified number of clients
func NewHTTPClientPool(size int) *HTTPClientPool {
	if size <= 0 {
		size = 3 // Default to 3 concurrent connections
	}

	p := &HTTPClientPool{
		clients: make([]*HTTPClient, size),
		pool:    make(chan *HTTPClient, size),
		size:    size,
	}

	// Initialize all clients
	for i := 0; i < size; i++ {
		p.clients[i] = NewHTTPClientWithEnv()
		p.pool <- p.clients[i]
	}

	return p
}

// Acquire gets a client from the pool (blocks if none available)
func (p *HTTPClientPool) Acquire() *HTTPClient {
	client := <-p.pool

	// Synchronize shared state to the acquired client
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.bearer != "" {
		client.SetBearerToken(p.bearer)
	}
	if p.cryptoKeyHex != "" {
		client.SetCryptoKeyHex(p.cryptoKeyHex)
	}
	if p.apiBase != "" {
		client.SetApiBase(p.apiBase)
	}
	if p.headers != "" {
		client.SetHeaders(p.headers)
	}
	client.SetDebug(p.debug)
	if p.reloginFunc != nil {
		client.SetReloginFunc(p.reloginFunc)
	}

	return client
}

// Release returns a client to the pool
func (p *HTTPClientPool) Release(client *HTTPClient) {
	// Sync any token updates back to the pool
	p.mu.Lock()
	newToken := client.GetBearerToken()
	if newToken != "" && newToken != p.bearer {
		p.bearer = newToken
	}
	p.mu.Unlock()

	p.pool <- client
}

// SetBearerToken updates the bearer token for all clients
func (p *HTTPClientPool) SetBearerToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.bearer = token
	// Update all clients in the pool
	for _, client := range p.clients {
		client.SetBearerToken(token)
	}
}

// GetBearerToken returns the current bearer token
func (p *HTTPClientPool) GetBearerToken() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bearer
}

// SetCryptoKeyHex updates the crypto key for all clients
func (p *HTTPClientPool) SetCryptoKeyHex(hex string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cryptoKeyHex = hex
	// Update all clients in the pool
	for _, client := range p.clients {
		client.SetCryptoKeyHex(hex)
	}
}

// GetCryptoKeyHex returns the current crypto key
func (p *HTTPClientPool) GetCryptoKeyHex() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cryptoKeyHex
}

// SetApiBase updates the API base URL for all clients
func (p *HTTPClientPool) SetApiBase(apiBase string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.apiBase = apiBase
	// Update all clients in the pool
	for _, client := range p.clients {
		client.SetApiBase(apiBase)
	}
}

// SetHeaders updates the headers for all clients
func (p *HTTPClientPool) SetHeaders(headers string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.headers = headers
	// Update all clients in the pool
	for _, client := range p.clients {
		client.SetHeaders(headers)
	}
}

// SetDebug updates the debug flag for all clients
func (p *HTTPClientPool) SetDebug(debug bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.debug = debug
	// Update all clients in the pool
	for _, client := range p.clients {
		client.SetDebug(debug)
	}
}

// SetReloginFunc updates the re-login function for all clients
func (p *HTTPClientPool) SetReloginFunc(fn ReloginFunc) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.reloginFunc = fn
	// Update all clients in the pool
	for _, client := range p.clients {
		client.SetReloginFunc(fn)
	}
}

// GetApiBase returns the current API base URL
func (p *HTTPClientPool) GetApiBase() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.apiBase
}

// GetHeaders returns the current headers
func (p *HTTPClientPool) GetHeaders() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.headers
}

// GetDebug returns the current debug flag
func (p *HTTPClientPool) GetDebug() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.debug
}

// WithClient executes a function with an acquired client and automatically releases it
func (p *HTTPClientPool) WithClient(fn func(*HTTPClient) error) error {
	client := p.Acquire()
	defer p.Release(client)
	return fn(client)
}
