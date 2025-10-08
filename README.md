# APIGuard

APIGuard is a specialized API gateway and protection layer for the Czech National Corpus (CNC). It acts as a proxy between clients and various linguistic data backends, providing security, authentication, rate limiting, and caching capabilities.

It is an essential component of word profile aggregator [ WaG](https://github.com/czcorpus/wag), where it collects data from multiple backends and streams them to the portal page using server-sent events.

## What It Does

- **Backend Proxying**: Routes requests to various CNC linguistic services (Kontext, Treq, MQuery, and dictionary services)
- **Security & Protection**: Guards against abuse with IP-based blocking, rate limiting, and session validation
- **Caching**: Improves performance with multi-backend caching support (Redis, file-based)
- **Monitoring**: Tracks usage patterns and detects anomalous behavior
- **Authentication**: Manages CNC user sessions and token-based API access
- **Data streaming**: collect data from different backends and stream them via an EventStream.

## Build

```bash
make build
```

## Usage

```bash
# Start the gateway service
./apiguard start

# Check status
./apiguard status

# Generate authentication token
./apiguard generate-token
```

