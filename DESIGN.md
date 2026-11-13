**English** | [Bahasa Indonesia](DESIGN.id.md)

# Design Choices

This document explains the main design decisions behind the Flight Search & Aggregation System: what was chosen, why, and the trade-offs. For setup and usage, see the [README](README.md).

---

## Table of contents

- [Summary](#summary)
- [1. Providers: fetching is separated from understanding](#1-providers-fetching-is-separated-from-understanding)
- [2. Timestamps are the source of truth](#2-timestamps-are-the-source-of-truth)
- [3. Time zones](#3-time-zones)
- [4. Parallel aggregation with partial failure](#4-parallel-aggregation-with-partial-failure)
- [5. Retry with exponential backoff](#5-retry-with-exponential-backoff)
- [6. Rate limiting](#6-rate-limiting)
- [7. Caching](#7-caching)
- [8. Filtering and sorting](#8-filtering-and-sorting)
- [9. Best value scoring](#9-best-value-scoring)
- [10. Price comparison across providers](#10-price-comparison-across-providers)
- [11. Round trips](#11-round-trips)
- [12. Multi-city](#12-multi-city)
- [13. Response format](#13-response-format)
- [14. API-first with OpenAPI](#14-api-first-with-openapi)
- [Assumptions](#assumptions)
- [Data inconsistencies found and handled](#data-inconsistencies-found-and-handled)
- [Performance and complexity](#performance-and-complexity)
- [Known limitations and future work](#known-limitations-and-future-work)

---

## Summary

- **Providers** are split into a `Fetcher` (gets raw bytes) and a `Provider` (decodes and normalizes one airline's format). Retry and rate limiting wrap the fetcher as decorators, so switching from mocks to real HTTP APIs changes only the wiring.
- **Timestamps are the source of truth.** Durations are always computed from departure and arrival instants; provider-reported durations are only cross-checked, because several of them are wrong.
- **Times keep the airport's local offset** (WIB, WITA, WIT), and dates are matched in the origin's local time, so a 04:45 WIB departure is still a 15 Dec flight.
- **Providers are queried in parallel** with one overall timeout; a failing or slow provider only removes its own flights (partial failure), and panics are isolated.
- **Temporary failures are retried** with exponential backoff and jitter. **Calls are rate-limited per provider in Redis**, shared by every app instance.
- **Aggregated provider data is cached** in Redis for 2 minutes, before filtering, so different filters share one entry; partial results are not cached and concurrent identical misses are merged with singleflight.
- **Best value** ranks by price (50%), duration (30%) and stops (20%), measured relative to the best result after min-max scaling proved misleading on the real data.
- **The same flight from several providers** is merged, keeping the cheapest offer, after filtering.
- **Every search is a list of legs**: one-way (1), round trip (2) and multi-city (2–5, separate endpoint) share one parallel code path.
- **API-first**: the OpenAPI spec is the contract; server code is generated, and the build fails if an endpoint is not implemented.

---

## 1. Providers: fetching is separated from understanding

Each provider is split into two roles:

- **`Fetcher`** gets the raw response bytes. Today `MockFetcher` returns embedded JSON after a simulated delay; tomorrow an `HTTPFetcher` would call the real API.
- **`Provider`** (`garuda`, `lion`, `batik`, `airasia`) decodes its airline's format and normalizes every flight into `models.Flight`.

The mock returns raw JSON bytes rather than Go structs, so the real decoding path is always exercised. Retry and rate limiting are added as **decorators** around the fetcher:

```
RetryFetcher -> RateLimitedFetcher -> MockFetcher
```

Providers never know these layers exist. Switching to real APIs means writing one `HTTPFetcher` and changing `cmd/server/providers.go`; normalizers, aggregator, cache and search are untouched.

The mock behaves like a real HTTP client in the ways that matter: it respects context cancellation (it waits with `select`, not `time.Sleep`), returns the same error types (`ErrProviderUnavailable`), and its randomness is injectable for deterministic tests.

## 2. Timestamps are the source of truth

Several providers report durations that disagree with their own departure and arrival times (see [data inconsistencies](#data-inconsistencies-found-and-handled)). The rule is simple: **duration is always `arrival - departure`**, computed from absolute instants. Reported durations are only used for a cross-check, and a mismatch is logged with both values.

Derived values (`timestamp`, `"4h 20m"`, `"Rp 595.000"`, stop count) are never stored in the domain model. They are computed when the response is built, so they cannot drift out of sync. The provided `expected_result.json` shows why this matters: its `timestamp` disagrees with its own `datetime`.

## 3. Time zones

- Each time keeps its **airport's local offset** (`+07:00` WIB, `+08:00` WITA, `+09:00` WIT), so the API shows local times as travellers expect.
- Lion Air sends times without an offset plus an IANA zone name; they are parsed with `time.ParseInLocation`. Batik sends `+0700` (no colon), which needs its own layout.
- **Dates are matched in local time at the origin.** QZ520 departs at 04:45 WIB on 15 Dec, which is 21:45 UTC on the 14th. Comparing in UTC would drop it from a search for the 15th. A dedicated `Date` type (no time, no zone) avoids this.
- Time filters (`departureTimeFrom`, ...) also use local airport time.
- The IANA zone database is embedded (`time/tzdata`), so zone lookups work in the minimal distroless image.
- WIT is covered by a unit test (Jakarta → Jayapura): 05:30 WIB to 12:30 WIT is 5 hours, not the 7 hours suggested by the local clocks.

## 4. Parallel aggregation with partial failure

The aggregator starts one goroutine per provider and waits with a `sync.WaitGroup`. Search time is roughly the **slowest** provider (Batik, 200–400 ms) instead of the sum of all four (up to ~850 ms).

- **One overall timeout** (`PROVIDER_TIMEOUT`) bounds the whole search, retries included.
- **Partial failure**: a failing or slow provider only removes its own flights; the search still returns 200 with `providers_failed` reported.
- Each goroutine writes only to its own slot in a results slice, so no mutex is needed and the result order is deterministic.
- A panic inside a provider is recovered in its goroutine and counted as a failure. Without this, one buggy provider would crash the whole server.
- `errgroup.WithContext` was deliberately not used: it cancels every goroutine as soon as one fails, which is the opposite of what partial results need.

## 5. Retry with exponential backoff

Only `ErrProviderUnavailable` (a temporary failure) is retried, up to 3 attempts, with waits of about 50 ms → 100 ms → 200 ms plus **jitter** so that many clients do not retry in lockstep. Context errors and decoding errors are never retried. Waiting is cancellable, so retries stop as soon as the overall timeout is reached.

With AirAsia's 10% failure rate, 3 attempts reduce a visible failure to about 0.1 × 0.1 × 0.1 = 0.1%. The trade-off is slightly higher latency when a retry happens, still bounded by the overall timeout.

## 6. Rate limiting

The assignment asks for limits on calls **to** providers. A provider's quota applies to our whole system, however many instances run, so the counters live in **Redis** (GCRA algorithm via `redis_rate`, executed atomically in a Lua script).

- The limiter sits **inside** the retry decorator, so every retry attempt also uses quota.
- If the quota is used up, the fetcher **waits** when the next slot fits inside the request's deadline, and **fails fast** with `ErrRateLimited` when it does not.
- `ErrRateLimited` is not retried: retrying would only add pressure.
- If Redis is unreachable, calls are **allowed (fail open)** and a warning is logged. For flight search, briefly exceeding a quota is a better outcome than stopping every search.

## 7. Caching

- **What is cached**: the aggregated, normalized provider data **before** filtering and sorting. Searches for the same route and date share one entry whatever their filters.
- **Key**: `flights:v1:{origin}:{destination}:{date}:{cabin}`. Passenger count is excluded because it only affects the seat filter, which runs after the cache. `v1` lets every entry be invalidated by a version bump.
- **TTL 2 minutes**: prices change, so stale data must expire quickly, while popular searches still benefit.
- **Partial results are not cached**: otherwise one AirAsia failure would hide its flights for the whole TTL.
- **Runtime failures degrade, startup failures stop**: if Redis goes down while running, searches get slower but keep working. If Redis is unreachable at startup, the app exits with a clear error, because that is a configuration mistake.
- **Cache stampede protection**: `singleflight` merges identical searches that miss the cache at the same moment, so 20 concurrent requests trigger one provider fetch. The shared fetch uses `context.WithoutCancel`, so one client disconnecting does not fail the others.
- Filter and dedupe never modify their input, which makes sharing cached or singleflight results between requests safe.

## 8. Filtering and sorting

- Filters use pointers so "not set" differs from zero: `maxStops: 0` means direct only, which is not the same as no filter.
- Criteria (route, local date, cabin, seats ≥ passengers) are always re-checked, even though providers "should" only return matching flights.
- Time ranges where `from > to` wrap midnight (`22:00`–`05:00` = overnight).
- Sorting is stable with a deterministic tie-breaker (price, then departure time, then ID), so the same input always gives the same order.

## 9. Best value scoring

Lower score is better:

```
score = 0.5 × (price / cheapestPrice − 1)
      + 0.3 × (duration / fastestDuration − 1)
      + 0.2 × numberOfStops
```

Price and duration are measured **relative to the best result**: "20% more expensive than the cheapest" scores 0.2.

The first version used min-max normalization (scaling each factor between the minimum and maximum). Testing it on the real data showed two problems:

- In a narrow range it **magnifies** small differences. After filtering, durations ranged only from 100 to 110 minutes, so 5 minutes counted as half the scale and JT740 (Rp 950.000, 105 min) beat JT742 (Rp 890.000, 110 min).
- An outlier (GA315 at Rp 1.850.000) **compresses** differences between normal prices, and every flight with a stop ranked below every direct flight, even GA410 at Rp 1.450.000 above QZ7250 at Rp 485.000.

Relative scoring fixes both: JT742 now beats JT740, and the cheap QZ7250 with one stop now ranks above expensive direct flights while still below reasonably priced direct ones. The weights (price first, then duration, then stops) are a product decision, kept as named constants and locked by tests that check behaviour, not exact scores.

Scores are computed once before sorting (O(n)), never inside the comparator.

## 10. Price comparison across providers

The same physical flight can be sold by more than one provider. Duplicates are merged and the **cheapest offer** is kept. A flight is identified by:

- **flight number**, normalized (`"GA 400"` = `"GA400"`), because the number alone repeats every day,
- **departure instant** (Unix time), so different time formats still match,
- **cabin class**, because economy and business on the same plane are different products.

Deduplication runs **after filtering**. Example: offer A is cheaper but has 1 seat left, offer B has 10. For 2 passengers, deduplicating first would keep A, then the seat filter would remove it, and the flight would disappear even though B could serve the customer.

The assignment data contains no flight sold by two providers, so this behaviour is proven by unit tests.

## 11. Round trips

- The response has two lists, `flights` and `return_flights`, rather than every outbound × return combination (13 × 13 = 169 packages). The user picks one of each, the common pattern for one-way domestic fares.
- The return leg is simply a one-way search with origin and destination swapped (`SearchCriteria.ReturnLeg()`), so cache, singleflight, local-date filtering, dedupe and best value apply unchanged.
- Both legs run in parallel. Filters and sort apply to both legs.
- Metadata: provider counts are summed over both legs, `cache_hit` is true only if both legs hit, `total_results` counts both lists.
- One-way responses are unchanged: `return_flights` is omitted.

## 12. Multi-city

- **A separate endpoint** (`/api/v1/flights/search/multi-city`). Putting legs into the existing request would have made `origin`, `destination` and `departureDate` conditionally required, which OpenAPI and the generated types express poorly. A separate contract keeps both requests simple and leaves the one-way/round-trip API untouched.
- **Everything is a list of legs.** Internally the service has one function, `searchLegs`, that runs any number of legs in parallel and keeps their order. One-way is 1 leg, round trip is 2 legs, multi-city is 2–5. Round trip was refactored onto the same function, so there is one code path to test.
- Each leg goes through the normal pipeline, with its own cache key, so a leg searched before (by any endpoint) is served from the cache.
- **Validation**: 2–5 legs (fewer is not multi-city, more would fan out into too many provider calls), each leg valid, dates never going backwards. Legs may be disconnected (open-jaw), as in most flight search tools.
- Passengers, cabin, filters and sort are shared by all legs.
- Metadata follows the round-trip rules, summed over every leg.

## 13. Response format

- Field order matches `expected_result.json` (`x-order` in the OpenAPI spec; oapi-codegen would otherwise sort fields alphabetically).
- `aircraft` is `null` when unknown; `amenities` is `[]`, never `null`.
- `price.amount` stays a number for calculations; `price.formatted` (`"Rp 1.250.000"`) is for display.

## 14. API-first with OpenAPI

`api/openapi.yaml` is the contract. Server types and routing are generated with oapi-codegen (strict server on `net/http`, no framework). A compile-time check (`var _ oapi.StrictServerInterface = (*Server)(nil)`) makes the build fail if an endpoint in the spec is not implemented. Generated code is committed so the project builds without the generator.

## Assumptions

- Garuda baggage numbers (`"carry_on": 1, "checked": 2`) are treated as piece counts, since no unit is given.
- AirAsia prices are IDR (the field is named `price_idr`).
- Batik's fare code `Y` is economy (W premium economy, C/J business, F first).
- AirAsia sends no airline code; it is taken from the flight number (`QZ7250` → `QZ`).
- Batik's `totalPrice` (base price + taxes) is what the customer pays.

---

## Data inconsistencies found and handled

| Where | Problem | Handling |
|---|---|---|
| Garuda GA315 | Top level says it lands in **SUB** with `stops: 0`; its segments show CGK → SUB → DPS | Departure from the first segment, arrival from the last, layovers from the gaps between segments |
| Garuda GA315 | Second segment claims 90 min; SUB 17:15 WIB → DPS 18:45 WITA is really **30 min** | Total duration from timestamps: **225 min** (summing segment fields would give 285) |
| Batik ID7042 | `travelTime: "3h 5m"` (185 min), real duration **245 min** (time zone forgotten) | Timestamps used, mismatch logged |
| AirAsia | `duration_hours` is a rounded float (1.67 h = 100.2 min) | Timestamps used; a 1-minute difference is tolerated as rounding |
| Lion Air | Times without offset, zone in a separate field | `time.ParseInLocation` with cached zones |
| Batik Air | Offset without colon (`+0700`) | Dedicated time layout |
| Several | Missing city, aircraft, amenities, airline code | City lookup by airport, `null` / `[]` in the response, code from flight number |
| Lion / Batik / AirAsia | Stops described by 2–3 fields that could disagree | Fields cross-checked; inconsistent flights are skipped and logged |
| `expected_result.json` | `timestamp` 1734246900 is in **2024**, not 2025, and disagrees with its `datetime`; `total_results` is 15 although the data has 13 flights | Treated as a format example, not as exact values |

Invalid flights (unparseable time, arrival not after departure, unknown cabin, non-positive price, inconsistent stops) are **skipped one by one and logged**; they never fail the whole provider.

---

## Performance and complexity

| Step | Cost |
|---|---|
| Provider calls | Parallel: ≈ slowest provider (200–400 ms), not the sum |
| Cache hit | ~0 ms, providers skipped |
| Filter | O(n), airline filter uses a set (O(1) lookup) |
| Dedupe | O(n), one map lookup per flight |
| Best value | O(n), scores computed once before sorting |
| Sort | O(n log n) |
| Time zone lookup | Cached after first use |

---

## Known limitations and future work

- **Providers do not receive the search criteria.** The mock returns its whole file and the service filters afterwards. With real APIs, criteria would be passed down to the fetcher (`Fetch(ctx, criteria)`). The change is mechanical but touches every provider. For that reason, round-trip and multi-city searches currently fetch the same mock data once per leg.
- **Multi-city results are per leg**, not combined itineraries with a total price or connection checks between legs (for example, that leg 2 departs after leg 1 lands on the same day). That would be the next step for a booking flow.
- **Health check** does not check Redis. A readiness endpoint that pings Redis would help orchestrators.
- **Observability**: no per-request logging middleware, request IDs or metrics yet (cache hit rate, provider latency, failures per provider).
- **Error format for malformed JSON bodies** comes from the generated code's default handler. A custom `StrictHTTPServerOptions` handler would make it use the same JSON error shape as every other error.
- **Best value weights** are fixed constants. They could become configurable or be tuned with real booking data.

---

Back to the [README](README.md).
