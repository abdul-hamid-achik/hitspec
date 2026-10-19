package oauth2

import (
	"context"
	"errors"
	"sync"
)

// TokenCache provides thread-safe caching for OAuth2 tokens
type TokenCache struct {
	tokens  map[string]*Token
	flights map[string]*flight
	mutex   sync.RWMutex
}

// flight is one in-progress token request shared by concurrent callers.
type flight struct {
	done  chan struct{}
	token *Token
	err   error
}

// NewTokenCache creates a new token cache
func NewTokenCache() *TokenCache {
	return &TokenCache{
		tokens:  make(map[string]*Token),
		flights: make(map[string]*flight),
	}
}

// GetOrFetch returns the cached, unexpired token for key or calls fetch to get
// one and caches it. Concurrent callers for the same key share one fetch.
// fetched reports whether this call (or the call it waited on) hit the token
// endpoint rather than the cache. A waiter whose leader failed only because the
// leader's context ended retries with its own context.
func (c *TokenCache) GetOrFetch(ctx context.Context, key string, fetch func(context.Context) (*Token, error)) (token *Token, fetched bool, err error) {
	for {
		c.mutex.Lock()
		if t := c.tokens[key]; t != nil && !t.IsExpired() {
			c.mutex.Unlock()
			return t, false, nil
		}
		if f, ok := c.flights[key]; ok {
			c.mutex.Unlock()
			select {
			case <-f.done:
			case <-ctx.Done():
				return nil, false, ctx.Err()
			}
			if f.err != nil {
				if ctx.Err() == nil && (errors.Is(f.err, context.Canceled) || errors.Is(f.err, context.DeadlineExceeded)) {
					continue
				}
				return nil, true, f.err
			}
			return f.token, true, nil
		}
		f := &flight{done: make(chan struct{})}
		if c.flights == nil {
			c.flights = make(map[string]*flight)
		}
		c.flights[key] = f
		c.mutex.Unlock()

		f.token, f.err = fetch(ctx)

		c.mutex.Lock()
		delete(c.flights, key)
		if f.err == nil && f.token != nil {
			c.tokens[key] = f.token
		}
		c.mutex.Unlock()
		close(f.done)
		return f.token, true, f.err
	}
}

// Get retrieves a token from the cache
func (c *TokenCache) Get(key string) *Token {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.tokens[key]
}

// Set stores a token in the cache
func (c *TokenCache) Set(key string, token *Token) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.tokens[key] = token
}

// Delete removes a token from the cache
func (c *TokenCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.tokens, key)
}

// Clear removes all tokens from the cache
func (c *TokenCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.tokens = make(map[string]*Token)
}

// GlobalCache is a shared token cache for the application
var GlobalCache = NewTokenCache()
