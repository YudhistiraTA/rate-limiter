package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

//go:embed tokenbucket.lua
var luaScript string

// Rate limiting is done using Redis via a Lua script
// This is done to ensure atomicity instead of trying
// to do it via multiple Redis commands in application code
var tokenBucketScript = redis.NewScript(luaScript)

func main() {
	// Load environments and required variables
	err := godotenv.Load()
	if err != nil {
		log.Panicf("Failed to load .env file: %v", err)
	}
	targetUrlStr := os.Getenv("TARGET_URL")
	if targetUrlStr == "" {
		log.Panic("TARGET_URL environment variable is required")
	}
	targetUrl, err := url.Parse(targetUrlStr)
	if err != nil {
		log.Panicf("Invalid TARGET_URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	ctx := context.Background()
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Password: redisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Panicf("Failed to connect to Redis: %v", err)
	}
	capStr := os.Getenv("RATE_LIMITER_CAPACITY")
	if capStr == "" {
		log.Panic("RATE_LIMITER_CAPACITY environment variable is required")
	}
	cap, err := strconv.Atoi(capStr)
	if err != nil {
		log.Panicf("Invalid RATE_LIMITER_CAPACITY: %v", err)
	}
	refillStr := os.Getenv("RATE_LIMITER_REFILL_RATE")
	if refillStr == "" {
		log.Panic("RATE_LIMITER_REFILL_RATE environment variable is required")
	}
	refill, err := strconv.Atoi(refillStr)
	if err != nil {
		log.Panicf("Invalid RATE_LIMITER_REFILL_RATE: %v", err)
	}
	ttlStr := os.Getenv("RATE_LIMITER_TTL")
	if ttlStr == "" {
		log.Panic("RATE_LIMITER_TTL environment variable is required")
	}
	ttl, err := strconv.Atoi(ttlStr)
	if err != nil {
		log.Panicf("Invalid RATE_LIMITER_TTL: %v", err)
	}
	identifierType := os.Getenv("RATE_LIMITER_IDENTIFIER")
	if identifierType == "" {
		identifierType = "IP"
		log.Println("RATE_LIMITER_IDENTIFIER not set, defaulting to IP")
	}
	apiKeyHeader := os.Getenv("API_KEY_HEADER")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Println("PORT not set, defaulting to 8080")
	}

	// Define HTTP handler
	handler := createHandler(
		rdb,
		proxy,
		cap,
		refill,
		ttl,
		identifierType,
		apiKeyHeader,
		tokenBucketScript,
	)

	// Start HTTP server
	log.Printf("Starting server on port %s\n", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func createHandler(rdb *redis.Client, proxy *httputil.ReverseProxy, cap, refill, ttl int, identifierType, apiKeyHeader string, script *redis.Script) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define identifier based on configuration
		var identifier string
		switch identifierType {
		case "IP":
			identifier = r.RemoteAddr
		case "API_KEY":
			identifier = r.Header.Get(apiKeyHeader)
			if identifier == "" {
				http.Error(w, "Missing API Key", http.StatusUnauthorized)
				return
			}
		default:
			// Default to IP if not configured properly
			identifier = r.RemoteAddr
		}

		// Execute Lua script for token bucket algorithm
		res, err := script.Run(r.Context(), rdb, []string{identifier}, cap, refill, ttl).Result()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if res.(int64) == 0 {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Forward request to target server
		proxy.ServeHTTP(w, r)
	})
}
