**English** | [Bahasa Indonesia](README.id.md)

# Flight Search & Aggregation System

A Go service that searches four airline providers (Garuda Indonesia, Lion Air, Batik Air, AirAsia) in parallel, normalizes their very different response formats into one model, and returns filtered, sorted and ranked results through a single HTTP API.

The providers are simulated in-process with realistic latency and failure rates, as required by the assignment. The design keeps the switch to real HTTP providers to a single wiring change.

> **Design choices** (why things are built this way, trade-offs, data issues found, limitations) are documented separately in **[DESIGN.md](DESIGN.md)**.

---

## Table of contents

- [Features](#features)
- [Quick start](#quick-start)
- [Using the API](#using-the-api)
- [Configuration](#configuration)
- [Project structure](#project-structure)
- [How a search works](#how-a-search-works)
- [Simulating provider behaviour](#simulating-provider-behaviour)
- [Additional mock data](#additional-mock-data)
- [Testing](#testing)
- [Verifying Redis (cache and rate limiter)](#verifying-redis-cache-and-rate-limiter)
- [Design choices](#design-choices)

---

## Features

### Core requirements

| Requirement | Status |
|---|---|
| Aggregate and normalize data from 4 providers | ✅ |
| Search by origin, destination, date | ✅ |
| Filter by price range, stops, departure/arrival time, airlines, duration | ✅ |
| Sort by price, duration, departure time, arrival time | ✅ |
| Compare prices across providers for the same flight | ✅ |
| Total trip duration including layovers | ✅ |
| "Best value" ranking | ✅ |
| Handle time formats, time zones, missing fields, invalid data | ✅ |

### Bonus

| Bonus | Status |
|---|---|
| Best value scoring algorithm | ✅ |
| Round-trip searches | ✅ |
| Time zone conversion (WIB, WITA, WIT) | ✅ |
| Rate limiting for provider APIs | ✅ Redis-backed, shared across instances |
| Retry with exponential backoff | ✅ With jitter |
| IDR formatting with thousands separator | ✅ |
| Parallel provider queries with timeout | ✅ |
| Multi-city searches | ✅ Separate endpoint, 2–5 legs searched in parallel |

---

## Quick start

### Requirements

- Go 1.22 or newer
- Docker and Docker Compose (for Redis, or to run everything in containers)

### Option A: everything in Docker

```bash
cp .env.example .env
make docker-up
```

### Option B: app locally, Redis in Docker

```bash
cp .env.example .env
docker compose up -d redis
make run
```

### Option C: no Redis at all

The cache and the rate limiter fall back to in-process memory:

```bash
STORE_DRIVER=memory make run
```

Then open **Swagger UI at http://localhost:8080/docs**.

### Make targets

| Target | What it does |
|---|---|
| `make run` | Run the server locally (reads `.env`) |
| `make build` | Build the binary into `bin/` |
| `make test` | Run unit tests |
| `make generate` | Regenerate `internal/oapi` from `api/openapi.yaml` |
| `make docker-up` | Build and start app + Redis |
| `make docker-down` | Stop the containers |

Run the tests with the race detector (recommended, since the code is highly concurrent):

```bash
go test -race ./...
```

---

## Using the API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/flights/search` | One-way and round-trip search |
| `POST` | `/api/v1/flights/search/multi-city` | Multi-city search (2–5 legs) |
| `GET` | `/health` | Liveness check |
| `GET` | `/docs` | Swagger UI |
| `GET` | `/openapi.yaml` | Raw OpenAPI 3.0 specification |

### One-way search

```bash
curl -X POST http://localhost:8080/api/v1/flights/search \
  -H 'Content-Type: application/json' \
  -d '{
    "origin": "CGK",
    "destination": "DPS",
    "departureDate": "2025-12-15",
    "passengers": 1,
    "cabinClass": "economy"
  }'
```

Returns all 13 flights from the assignment data, ranked by best value.

### With filters and sorting

```bash
curl -X POST http://localhost:8080/api/v1/flights/search \
  -H 'Content-Type: application/json' \
  -d '{
    "origin": "CGK",
    "destination": "DPS",
    "departureDate": "2025-12-15",
    "passengers": 1,
    "cabinClass": "economy",
    "filters": { "maxPrice": 1000000, "maxStops": 0 },
    "sort": { "by": "price", "order": "asc" }
  }'
```

Returns 5 direct flights under Rp 1.000.000: QZ532, QZ520, QZ524, JT742, JT740.

### Round trip

```bash
curl -X POST http://localhost:8080/api/v1/flights/search \
  -H 'Content-Type: application/json' \
  -d '{
    "origin": "CGK",
    "destination": "DPS",
    "departureDate": "2025-12-15",
    "returnDate": "2025-12-20",
    "passengers": 1,
    "cabinClass": "economy"
  }'
```

Returns 13 outbound flights in `flights` and 7 return flights in `return_flights`.

### Multi-city

```bash
curl -X POST http://localhost:8080/api/v1/flights/search/multi-city \
  -H 'Content-Type: application/json' \
  -d '{
    "legs": [
      { "origin": "CGK", "destination": "DPS", "departureDate": "2025-12-15" },
      { "origin": "DPS", "destination": "SUB", "departureDate": "2025-12-18" },
      { "origin": "SUB", "destination": "CGK", "departureDate": "2025-12-20" }
    ],
    "passengers": 1,
    "cabinClass": "economy"
  }'
```

Returns one list per leg, in request order: 13 flights for CGK → DPS, 2 for DPS → SUB, 2 for SUB → CGK. `filters` and `sort` are accepted too and apply to every leg.

```json
{
  "search_criteria": { "passengers": 1, "cabin_class": "economy" },
  "metadata": { "total_results": 17, "providers_queried": 12, "providers_succeeded": 12, "providers_failed": 0, "search_time_ms": 382, "cache_hit": false },
  "legs": [
    { "origin": "CGK", "destination": "DPS", "departure_date": "2025-12-15", "flights": [ ... ] },
    { "origin": "DPS", "destination": "SUB", "departure_date": "2025-12-18", "flights": [ ... ] },
    { "origin": "SUB", "destination": "CGK", "departure_date": "2025-12-20", "flights": [ ... ] }
  ]
}
```

Rules: 2 to 5 legs, each leg valid on its own, dates must not go backwards. Legs do not have to connect, so open-jaw trips (for example CGK → DPS, then SUB → CGK) are allowed.

### Request fields

| Field | Required | Notes |
|---|---|---|
| `origin`, `destination` | yes | 3-letter IATA codes, must differ |
| `departureDate` | yes | `YYYY-MM-DD`, matched against the **local date at the origin airport** |
| `returnDate` | no | Makes it a round trip; must not be before `departureDate` |
| `passengers` | yes | 1–9; flights with fewer seats are excluded |
| `cabinClass` | yes | `economy`, `premium_economy`, `business`, `first` |
| `filters` | no | See below; every filter is optional |
| `sort` | no | Defaults to `best_value`, `asc` |

**Filters**

| Filter | Meaning |
|---|---|
| `minPrice`, `maxPrice` | Price range in IDR |
| `maxStops` | `0` = direct flights only |
| `departureTimeFrom`, `departureTimeTo` | `HH:MM`, local time at the origin. A range like `22:00`–`05:00` wraps midnight |
| `arrivalTimeFrom`, `arrivalTimeTo` | `HH:MM`, local time at the destination |
| `airlines` | IATA codes, e.g. `["GA", "JT"]` |
| `maxDurationMinutes` | Total trip time including layovers |

**Sort**: `by` is one of `best_value`, `price`, `duration`, `departure_time`, `arrival_time`; `order` is `asc` or `desc`.

### Response

```json
{
  "search_criteria": {
    "origin": "CGK",
    "destination": "DPS",
    "departure_date": "2025-12-15",
    "return_date": null,
    "passengers": 1,
    "cabin_class": "economy"
  },
  "metadata": {
    "total_results": 13,
    "providers_queried": 4,
    "providers_succeeded": 4,
    "providers_failed": 0,
    "search_time_ms": 252,
    "cache_hit": false
  },
  "flights": [
    {
      "id": "QZ532_AirAsia",
      "provider": "AirAsia",
      "airline": { "name": "AirAsia", "code": "QZ" },
      "flight_number": "QZ532",
      "departure": {
        "airport": "CGK",
        "city": "Jakarta",
        "datetime": "2025-12-15T19:30:00+07:00",
        "timestamp": 1765801800
      },
      "arrival": {
        "airport": "DPS",
        "city": "Denpasar",
        "datetime": "2025-12-15T22:10:00+08:00",
        "timestamp": 1765807800
      },
      "duration": { "total_minutes": 100, "formatted": "1h 40m" },
      "stops": 0,
      "price": { "amount": 595000, "currency": "IDR", "formatted": "Rp 595.000" },
      "available_seats": 72,
      "cabin_class": "economy",
      "aircraft": null,
      "amenities": [],
      "baggage": { "carry_on": "Cabin baggage only", "checked": "Additional fee" }
    }
  ]
}
```

`return_flights` appears only for round trips (and is `[]` if no return flight matches).

### Errors

Every error uses the same shape:

```json
{ "error": { "code": "invalid_request", "message": "invalid request: destination must differ from origin" } }
```

`400` for invalid input, `500` for unexpected errors (internal details are logged, never returned).

---

## Configuration

Environment variables (see `.env.example`):

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | HTTP port |
| `REDIS_PORT` | `6379` | Redis port exposed on the host (Docker Compose only) |
| `STORE_DRIVER` | `redis` | Backend for cache and rate limiter: `redis` or `memory` |
| `REDIS_ADDR` | `localhost:6379` | Redis address (Docker Compose uses `redis:6379`) |
| `CACHE_TTL` | `2m` | How long aggregated provider data is reused |
| `PROVIDER_TIMEOUT` | `2s` | Total time budget for all providers in one search |
| `PROVIDER_RATE_LIMIT` | `10` | Max calls per second to each provider |

Invalid values (for example `PROVIDER_TIMEOUT=2detik`) stop the app at startup with a clear message instead of silently falling back to a default. The effective configuration is logged at startup.

---

## Project structure

```
api/
  openapi.yaml            API contract (source of truth)
  oapi-codegen.yaml       code generator config
  embed.go                embeds the spec for /openapi.yaml and Swagger UI
cmd/server/
  main.go                 composition root: wires every dependency
  providers.go            builds each provider and its fetch chain
  stores.go               builds cache + rate limiter (Redis or memory)
internal/
  oapi/                   GENERATED by oapi-codegen, do not edit
  handlers/               HTTP layer (server.go wires the routes)
    health/ flight/ docs/ health check, flight search + multi-city, Swagger UI
  services/               search pipeline: cache -> aggregate -> filter -> dedupe -> sort
  aggregator/             calls all providers in parallel with a timeout
  providers/              Provider/Fetcher interfaces, mock, retry, rate limiting, time zones
    garuda/ lion/ batik/ airasia/   one normalizer per airline
    mockdata/             the provider JSON files (embedded)
  pkg/
    cache/                Redis and in-memory cache
    ratelimit/            Redis and in-memory rate limiter
    helper/
      search/             filter, sort, best value scoring, dedupe (pure functions)
      money/              IDR formatting
  validation/             business rules for requests
  models/                 domain types
  config/                 environment configuration
```

Dependencies are injected through small interfaces declared by the code that uses them (`handlers/flight.SearchService`, `services.FlightAggregator`, `services.Cache`, `providers.Fetcher`, `providers.Limiter`). Only `cmd/server` knows the concrete types, which is what makes every layer testable with fakes.

---

## How a search works

```
POST /api/v1/flights/search
  │
  ├─ handler        map JSON -> domain request
  ├─ validation     business rules (400 on failure)
  ├─ cache          hit? -> skip providers
  │    miss ─► singleflight (identical concurrent searches share one fetch)
  │             └─ aggregator: 4 goroutines, one overall timeout
  │                  └─ provider: Fetch -> decode -> normalize each flight
  │                       └─ RetryFetcher -> RateLimitedFetcher -> MockFetcher
  ├─ filter         route, local date, cabin, seats + user filters
  ├─ dedupe         same flight from several providers -> cheapest offer
  ├─ sort           best value (default) or the requested field
  └─ handler        map domain -> JSON (timestamps, "4h 20m", "Rp 595.000")
```

Every search is a list of **legs**, and each leg runs this pipeline in parallel: one leg for one-way, two for a round trip (the second is the route reversed), and 2–5 for multi-city.

---

## Simulating provider behaviour

Providers are simulated by `MockFetcher` with the latency and reliability required by the assignment. The values are set in `cmd/server/providers.go`:

| Provider | Delay | Failure rate |
|---|---|---|
| Garuda Indonesia | 50–100 ms | 0% |
| Lion Air | 100–200 ms | 0% |
| Batik Air | 200–400 ms | 0% |
| AirAsia | 50–150 ms | 10% |

The delay is random within the range on every call, and failures return `ErrProviderUnavailable`, exactly like a real provider answering with a 5xx error.

Every scenario below starts from an empty cache, otherwise results come from Redis and providers are not called at all:

```bash
docker compose exec redis redis-cli flushall
```

**Parallel calls.** Search a few times and look at `search_time_ms`: it follows the slowest provider (around 200–400 ms because of Batik), not the sum of all four.

**Provider failure and retry.** In `cmd/server/providers.go`, raise AirAsia's `FailureRate` and restart:

- `0.5`: the log shows `fetch failed, retrying` with the attempt number and backoff; most searches still end with 4 providers succeeding.
- `1.0`: AirAsia always fails after 3 attempts. The search still returns 200 with the other providers' flights and `providers_failed: 1` (partial failure).

Set it back to `0.1` afterwards.

**Timeout.** Set `PROVIDER_TIMEOUT=150ms` in `.env` and restart. Batik (200–400 ms) can never answer in time, so it fails with `context deadline exceeded`, the others still answer, and `search_time_ms` never goes far above 150. Set it back to `2s`.

**Rate limiting.** See [Verifying Redis](#verifying-redis-cache-and-rate-limiter): with `PROVIDER_RATE_LIMIT=1`, parallel searches compete for the same quota.

**Cache and singleflight.** Repeat the same search: the second answer has `cache_hit: true` and takes ~0 ms. Sending many identical searches at once on an empty cache calls the providers only once.

**Deterministic simulation in tests.** Tests do not depend on luck: `MockFetcher` accepts a seeded random generator, so a failure rate of `1.0` always fails, `0.0` never fails, and anything in between gives the same sequence on every run.

---

## Additional mock data

The assignment data only contains CGK → DPS flights on 15 December, so round-trip and multi-city searches would always return empty lists after the first leg. To demonstrate them, **11 flights** were appended to the provider files:

| Route and date | Used by | Flights |
|---|---|---|
| DPS → CGK, 20 Dec | Round trip | GA401, GA421 (Garuda), JT741, JT743 with 1 stop via SUB (Lion), ID6515 (Batik), QZ521, QZ535 (AirAsia) |
| DPS → SUB, 18 Dec | Multi-city | JT693 (Lion), QZ641 (AirAsia) |
| SUB → CGK, 20 Dec | Multi-city | GA313 (Garuda), ID6583 (Batik) |

- The original 13 flights are unchanged. One-way CGK → DPS searches still return exactly 13 flights.
- Each entry follows its provider's own format, so it also exercises the normalizers.
- The westbound flights test the opposite time-zone case: QZ535 leaves at 20:00 WITA and lands at 20:40 WIB, which looks like 40 minutes but is really 100; JT693 (DPS → SUB) looks like 5 minutes but is really 65.
- The added data is intentionally consistent: it adds no new traps, so every inconsistency listed in [DESIGN.md](DESIGN.md#data-inconsistencies-found-and-handled) comes from the original assignment files.

---

## Testing

```bash
go test -race ./...
```

Unit tests cover:

- **Normalizers**: GA315 multi-segment handling, WIB → WIT conversion, embedded mock files are valid JSON.
- **Mock fetcher**: delay, failure rate, cancellation, concurrent use (race detector).
- **Retry**: succeeds after temporary failures, gives up after max attempts, never retries other errors, stops on cancellation, backoff growth and cap.
- **Rate limiting**: waits for a near slot, fails fast for a far slot, fails open when Redis is down, per-provider quotas.
- **Search**: every filter, local-date matching, overnight time ranges, sorting and tie-breaking, best value behaviour, deduplication rules, inputs never modified.
- **Service**: cache hit on the second call, partial results not cached, Redis outage does not fail the search, 50 concurrent misses call providers once (singleflight), round trip legs, multi-city leg order and validation.
- **Cache, rate limiter, money formatting.**

Tests use fakes injected through interfaces and a fake clock, so they are fast and deterministic.

---

## Verifying Redis (cache and rate limiter)

Commands below assume Docker Compose (`docker compose exec redis redis-cli ...`).

**Is Redis up and is the app using it?**

```bash
docker compose ps                              # redis: running (healthy)
docker compose exec redis redis-cli ping       # PONG
docker compose logs app | grep "config loaded" # store_driver: redis
docker compose logs app | grep "using redis"
```

**Cache**

```bash
# entries and their remaining lifetime
docker compose exec redis redis-cli keys 'flights:*'
docker compose exec redis redis-cli ttl "flights:v1:CGK:DPS:2025-12-15:economy"

# the cached JSON itself
docker compose exec redis redis-cli get "flights:v1:CGK:DPS:2025-12-15:economy"

# watch commands live while searching (development only)
docker compose exec redis redis-cli monitor

# hit / miss counters
docker compose exec redis redis-cli info stats | grep keyspace

# force a miss
docker compose exec redis redis-cli del "flights:v1:CGK:DPS:2025-12-15:economy"
```

The first search returns `cache_hit: false` (~200–400 ms), the second `cache_hit: true` (~0 ms). `keys` is fine for development; use `scan` on a large production instance.

**Singleflight**: with an empty cache, 20 parallel identical searches produce 20 `GET` but only 1 `SET`:

```bash
docker compose exec redis redis-cli flushall
docker compose exec redis redis-cli config resetstat
for i in $(seq 1 20); do
  curl -s -o /dev/null -X POST localhost:8080/api/v1/flights/search \
    -H 'Content-Type: application/json' \
    -d '{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}' &
done; wait
docker compose exec redis redis-cli info commandstats | grep -E "cmdstat_(get|set):"
```

**Rate limiter**: rate limit keys expire within seconds, so `keys 'rate:*'` is usually empty. Check the script executions instead. To see requests being limited, set `PROVIDER_RATE_LIMIT=1` in `.env`, recreate the container, and send 4 searches with different dates in parallel:

```bash
docker compose up -d --build app
docker compose exec redis redis-cli flushall
docker compose exec redis redis-cli config resetstat

for d in 15 16 17 18; do
  (
    curl -s -X POST localhost:8080/api/v1/flights/search \
      -H 'Content-Type: application/json' \
      -d "{\"origin\":\"CGK\",\"destination\":\"DPS\",\"departureDate\":\"2025-12-$d\",\"passengers\":1,\"cabinClass\":\"economy\"}" \
      | grep -o '"providers_[a-z]*":[0-9]*' | tr '\n' ' '
    echo
  ) &
done
wait

docker compose exec redis redis-cli info commandstats | grep evalsha
docker compose logs app | grep "rate limit reached"
```

With 1 call per second and a 2-second budget, each provider serves 2 of the 4 searches; the rest fail with `provider rate limit reached`. Restore `PROVIDER_RATE_LIMIT=10` afterwards.

---

## Design choices

The reasoning behind the implementation is in **[DESIGN.md](DESIGN.md)**: a one-page summary first, then the details for each decision:

- provider abstraction (`Fetcher` / `Provider`) and the retry and rate-limit decorators
- timestamps as the source of truth, and time zone handling (WIB, WITA, WIT)
- parallel aggregation, timeouts and partial failure
- retry with exponential backoff, rate limiting, caching and singleflight
- filtering, sorting, best value scoring and price comparison across providers
- round trip and multi-city as lists of legs
- data inconsistencies found in the provider files, assumptions, performance, and known limitations
