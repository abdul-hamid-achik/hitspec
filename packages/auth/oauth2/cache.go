package oauth2

import (
	"sync"
)

// TokenCache provides thread-safe caching for OAuth2 tokens
type TokenCache struct {
	tokens map[string]*Token
	mutex  sync.RWMutex
}

// NewTokenCache creates a new token cache
func NewTokenCache() *TokenCache {
	return &TokenCache{
		tokens: make(map[string]*Token),
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
