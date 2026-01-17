// main_test.go
package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHandler_IPIdentifier(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	cap := 2
	refill := 1
	ttl := 60
	identifierType := "IP"
	apiKeyHeader := ""

	handler := createHandler(rdb, proxy, cap, refill, ttl, identifierType, apiKeyHeader, tokenBucketScript)

	// Test allowed requests
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Test rate limit exceeded
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "Rate limit exceeded")

	// Advance time to refill
	mr.FastForward(1 * time.Second)
	mr.SetTime(time.Now().Add(1 * time.Second))

	// Test refilled
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Test different IP not affected
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.2:12345"
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestCreateHandler_APIKeyIdentifier(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	cap := 1
	refill := 1
	ttl := 60
	identifierType := "API_KEY"
	apiKeyHeader := "X-API-Key"

	handler := createHandler(rdb, proxy, cap, refill, ttl, identifierType, apiKeyHeader, tokenBucketScript)

	// Test missing API key
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Missing API Key")

	// Test allowed with API key
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "testkey")
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())

	// Test exceeded with same key
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "testkey")
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "Rate limit exceeded")

	// Test different key not affected
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "otherkey")
	w = httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestCreateHandler_DefaultIdentifier(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	cap := 1
	refill := 1
	ttl := 60
	identifierType := "UNKNOWN" // Should default to IP
	apiKeyHeader := ""

	handler := createHandler(rdb, proxy, cap, refill, ttl, identifierType, apiKeyHeader, tokenBucketScript)

	// Test allowed request
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestCreateHandler_InvalidRemoteAddr(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	cap := 1
	refill := 1
	ttl := 60
	identifierType := "IP"
	apiKeyHeader := ""

	handler := createHandler(rdb, proxy, cap, refill, ttl, identifierType, apiKeyHeader, tokenBucketScript)

	// Test invalid RemoteAddr
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "invalid-addr"
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid remote address")
}

func TestCreateHandler_ScriptError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// Use invalid script to force error
	invalidScript := redis.NewScript("invalid lua")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()
	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	cap := 1
	refill := 1
	ttl := 60
	identifierType := "IP"
	apiKeyHeader := ""

	handler := createHandler(rdb, proxy, cap, refill, ttl, identifierType, apiKeyHeader, invalidScript)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	handler(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal Server Error")
}
