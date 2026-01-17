# Running Instruction

The repository already provides a `.env` file with all the required configurations.

## Docker

Just do docker compose up
```sh
docker compose up --build
```

The docker-compose file will run the rate limiter as a reverse proxy layer and the actual backend with a `/v1/health` route available.

The example backend's `/v1/health` will return the caller's IP address.

Use the following bash script to test the rate limiter. By default, the rate limiter should have a 5 request capacity with a refill rate of 1 request/second, so the following script must fail from the 6th request, and then work again after the user has waited for at least 5 seconds.

```bash
for i in {1..11}; do           
curl http://localhost:8080/v1/health    
done
```

## Manual Run

Just do `go run .` on both root and example dir. Make sure `.env` exists for root and a Redis server is running.

# Design Overview

This rate limiter is meant to be used as a reverse proxy layer, similar to a gateway.

![image](https://bytebytego.com/_next/image?url=%2Fimages%2Fcourses%2Fsystem-design-interview%2Fdesign-a-rate-limiter%2Ffigure-4-3-KLFLUVLJ.png&w=1080&q=75)

Amusingly, the bulk of the business logic is written in Lua inside of `tokenbucket.lua` because this service relies on Redis's atomicity instead of trying to wrangle mutexes and threads manually. This ensures that the service rate limiter service can be autoscaled horizontally on high traffic and does not ironically need to have a rate limiter itself.

The Go code is actually just a convenient wrapper for the Redis client. The Lua script is embedded into the built binary for easy deployment and zero IO interrupt for disk read on runtime.

## Rate limiter algorithm

It's just a simple leaky bucket algorithm. For each identifier (IP or x-api-key header), it will be given a `capacity` amount of tokens. Each request will consume a token. For every second that pass, the `capacity` for a given identifier is refilled by `refill` amount.

This algorithm is chosen over a simple time window to mitigate possible bursts, such as 100 requests at the 10th second and then another 100 at the 11th second, effectively doubling their alloted capacity due to using the next window.

# Assumptions

This service assumes the availability of Redis or other **atomic** in memory cache, because there is literally zero logic in the Go code itself. The atomicity being key here.

# Library Used

Just the standard godotenv and Redis client. The Go service uses raw HTTP from the standard library because it is simple enough to not need any framework like Gin or Fiber.

# Testing

Testing is available using the standard `go test -cover`

The tests provided only cover 27.8% of the code because the rest of it is just bootstrapping env variables in main function, and there's not much to meaningfully test there.

# Limitation

When rate limiter is set to API key, malicious client can easily just generate a new API key for every requests, effectively getting an entire `capacity` amount of requests each time and bypassing the rate limiter. A database call or JWT parsing may be necessary to validate new API keys. API keys that already exists inside the Redis cache is assumed as already verified and don't need to make DB call or parsing, or something like that.