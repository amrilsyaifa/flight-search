[English](README.md) | **Bahasa Indonesia**

# Flight Search & Aggregation System

Service Go yang mencari penerbangan dari empat provider maskapai (Garuda Indonesia, Lion Air, Batik Air, AirAsia) secara paralel, menormalisasi format response mereka yang sangat berbeda ke dalam satu model, lalu mengembalikan hasil yang sudah difilter, diurutkan, dan diperingkat melalui satu HTTP API.

Provider disimulasikan di dalam proses dengan latensi dan tingkat kegagalan yang realistis, sesuai permintaan assignment. Desainnya dibuat supaya peralihan ke provider HTTP sungguhan cukup dengan satu perubahan di bagian wiring.

> **Keputusan desain** (alasan di balik implementasi, trade-off, masalah data yang ditemukan, keterbatasan) didokumentasikan terpisah di **[DESIGN.id.md](DESIGN.id.md)**.

---

## Daftar isi

- [Fitur](#fitur)
- [Memulai](#memulai)
- [Menggunakan API](#menggunakan-api)
- [Konfigurasi](#konfigurasi)
- [Struktur project](#struktur-project)
- [Alur sebuah pencarian](#alur-sebuah-pencarian)
- [Simulasi perilaku provider](#simulasi-perilaku-provider)
- [Data mock tambahan](#data-mock-tambahan)
- [Testing](#testing)
- [Memverifikasi Redis (cache dan rate limiter)](#memverifikasi-redis-cache-dan-rate-limiter)
- [Keputusan desain](#keputusan-desain)

---

## Fitur

### Core requirement

| Requirement | Status |
|---|---|
| Agregasi dan normalisasi data dari 4 provider | ✅ |
| Pencarian berdasarkan origin, destination, tanggal | ✅ |
| Filter harga, jumlah transit, jam berangkat/tiba, maskapai, durasi | ✅ |
| Sort berdasarkan harga, durasi, jam berangkat, jam tiba | ✅ |
| Perbandingan harga antar-provider untuk penerbangan yang sama | ✅ |
| Durasi total termasuk layover | ✅ |
| Peringkat "best value" | ✅ |
| Penanganan format waktu, zona waktu, field kosong, data tidak valid | ✅ |

### Bonus

| Bonus | Status |
|---|---|
| Algoritma best value scoring | ✅ |
| Pencarian round-trip | ✅ |
| Konversi zona waktu (WIB, WITA, WIT) | ✅ |
| Rate limiting ke provider | ✅ Berbasis Redis, dipakai bersama antar-instance |
| Retry dengan exponential backoff | ✅ Dengan jitter |
| Format IDR dengan pemisah ribuan | ✅ |
| Query provider paralel dengan timeout | ✅ |
| Pencarian multi-city | ✅ Endpoint terpisah, 2–5 segmen dicari secara paralel |

---

## Memulai

### Kebutuhan

- Go 1.22 atau lebih baru
- Docker dan Docker Compose (untuk Redis, atau untuk menjalankan semuanya di container)

### Opsi A: semuanya di Docker

```bash
cp .env.example .env
make docker-up
```

### Opsi B: aplikasi di lokal, Redis di Docker

```bash
cp .env.example .env
docker compose up -d redis
make run
```

### Opsi C: tanpa Redis sama sekali

Cache dan rate limiter beralih ke memory di dalam proses:

```bash
STORE_DRIVER=memory make run
```

Lalu buka **Swagger UI di http://localhost:8080/docs**.

### Target Makefile

| Target | Fungsi |
|---|---|
| `make run` | Menjalankan server di lokal (membaca `.env`) |
| `make build` | Build binary ke `bin/` |
| `make test` | Menjalankan unit test |
| `make generate` | Generate ulang `internal/oapi` dari `api/openapi.yaml` |
| `make docker-up` | Build dan menjalankan aplikasi + Redis |
| `make docker-down` | Menghentikan container |

Jalankan test dengan race detector (disarankan, karena kode ini banyak memakai concurrency):

```bash
go test -race ./...
```

---

## Menggunakan API

| Method | Path | Keterangan |
|---|---|---|
| `POST` | `/api/v1/flights/search` | Pencarian satu arah dan round-trip |
| `POST` | `/api/v1/flights/search/multi-city` | Pencarian multi-city (2–5 segmen) |
| `GET` | `/health` | Pengecekan liveness |
| `GET` | `/docs` | Swagger UI |
| `GET` | `/openapi.yaml` | Spesifikasi OpenAPI 3.0 mentah |

### Pencarian satu arah

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

Mengembalikan ke-13 penerbangan dari data assignment, diurutkan berdasarkan best value.

### Dengan filter dan sort

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

Mengembalikan 5 penerbangan langsung di bawah Rp 1.000.000: QZ532, QZ520, QZ524, JT742, JT740.

### Round-trip

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

Mengembalikan 13 penerbangan pergi di `flights` dan 7 penerbangan pulang di `return_flights`.

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

Mengembalikan satu daftar per segmen, sesuai urutan request: 13 penerbangan untuk CGK → DPS, 2 untuk DPS → SUB, 2 untuk SUB → CGK. `filters` dan `sort` juga diterima dan berlaku untuk setiap segmen.

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

Aturannya: 2 sampai 5 segmen, setiap segmen valid, dan tanggal tidak boleh mundur. Segmen tidak harus bersambung, jadi perjalanan open-jaw (misalnya CGK → DPS, lalu SUB → CGK) diizinkan.

### Field request

| Field | Wajib | Catatan |
|---|---|---|
| `origin`, `destination` | ya | Kode IATA 3 huruf, harus berbeda |
| `departureDate` | ya | `YYYY-MM-DD`, dicocokkan dengan **tanggal lokal di bandara asal** |
| `returnDate` | tidak | Menjadikan pencarian round-trip; tidak boleh sebelum `departureDate` |
| `passengers` | ya | 1–9; penerbangan dengan kursi kurang akan dikeluarkan |
| `cabinClass` | ya | `economy`, `premium_economy`, `business`, `first` |
| `filters` | tidak | Lihat di bawah; setiap filter opsional |
| `sort` | tidak | Default `best_value`, `asc` |

**Filter**

| Filter | Arti |
|---|---|
| `minPrice`, `maxPrice` | Rentang harga dalam IDR |
| `maxStops` | `0` = hanya penerbangan langsung |
| `departureTimeFrom`, `departureTimeTo` | `HH:MM`, waktu lokal bandara asal. Rentang seperti `22:00`–`05:00` melewati tengah malam |
| `arrivalTimeFrom`, `arrivalTimeTo` | `HH:MM`, waktu lokal bandara tujuan |
| `airlines` | Kode IATA, misalnya `["GA", "JT"]` |
| `maxDurationMinutes` | Durasi total perjalanan termasuk layover |

**Sort**: `by` salah satu dari `best_value`, `price`, `duration`, `departure_time`, `arrival_time`; `order` bernilai `asc` atau `desc`.

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

`return_flights` hanya muncul untuk round-trip (dan bernilai `[]` kalau tidak ada penerbangan pulang yang cocok).

### Error

Semua error memakai bentuk yang sama:

```json
{ "error": { "code": "invalid_request", "message": "invalid request: destination must differ from origin" } }
```

`400` untuk input tidak valid, `500` untuk error tak terduga (detail internal hanya dicatat di log, tidak pernah dikirim ke client).

---

## Konfigurasi

Environment variable (lihat `.env.example`):

| Variable | Default | Keterangan |
|---|---|---|
| `HTTP_PORT` | `8080` | Port HTTP |
| `REDIS_PORT` | `6379` | Port Redis yang dibuka di host (hanya Docker Compose) |
| `STORE_DRIVER` | `redis` | Backend cache dan rate limiter: `redis` atau `memory` |
| `REDIS_ADDR` | `localhost:6379` | Alamat Redis (Docker Compose memakai `redis:6379`) |
| `CACHE_TTL` | `2m` | Berapa lama data agregasi provider dipakai ulang |
| `PROVIDER_TIMEOUT` | `2s` | Batas waktu total untuk semua provider dalam satu pencarian |
| `PROVIDER_RATE_LIMIT` | `10` | Maksimal panggilan per detik ke setiap provider |

Nilai yang tidak valid (misalnya `PROVIDER_TIMEOUT=2detik`) menghentikan aplikasi saat start dengan pesan yang jelas, bukan diam-diam memakai nilai default. Konfigurasi yang benar-benar dipakai dicatat di log saat start.

---

## Struktur project

```
api/
  openapi.yaml            kontrak API (sumber kebenaran)
  oapi-codegen.yaml       konfigurasi code generator
  embed.go                meng-embed spec untuk /openapi.yaml dan Swagger UI
cmd/server/
  main.go                 composition root: merangkai semua dependency
  providers.go            membangun setiap provider beserta rantai fetch-nya
  stores.go               membangun cache + rate limiter (Redis atau memory)
internal/
  oapi/                   HASIL GENERATE oapi-codegen, jangan diedit
  handlers/               layer HTTP (server.go merangkai route)
    health/ flight/ docs/ health check, flight search + multi-city, Swagger UI
  services/               pipeline pencarian: cache -> agregasi -> filter -> dedupe -> sort
  aggregator/             memanggil semua provider secara paralel dengan timeout
  providers/              interface Provider/Fetcher, mock, retry, rate limiting, zona waktu
    garuda/ lion/ batik/ airasia/   satu normalizer per maskapai
    mockdata/             file JSON provider (di-embed)
  pkg/
    cache/                cache Redis dan in-memory
    ratelimit/            rate limiter Redis dan in-memory
    helper/
      search/             filter, sort, best value scoring, dedupe (pure function)
      money/              format IDR
  validation/             aturan bisnis untuk request
  models/                 tipe domain
  config/                 konfigurasi dari environment
```

Dependency di-inject lewat interface kecil yang didefinisikan oleh kode pemakainya (`handlers/flight.SearchService`, `services.FlightAggregator`, `services.Cache`, `providers.Fetcher`, `providers.Limiter`). Hanya `cmd/server` yang mengenal tipe konkretnya, dan itulah yang membuat setiap layer bisa dites dengan fake.

---

## Alur sebuah pencarian

```
POST /api/v1/flights/search
  │
  ├─ handler        JSON -> request domain
  ├─ validation     aturan bisnis (400 kalau gagal)
  ├─ cache          hit? -> provider dilewati
  │    miss ─► singleflight (pencarian identik yang bersamaan berbagi satu fetch)
  │             └─ aggregator: 4 goroutine, satu timeout keseluruhan
  │                  └─ provider: Fetch -> decode -> normalisasi setiap flight
  │                       └─ RetryFetcher -> RateLimitedFetcher -> MockFetcher
  ├─ filter         rute, tanggal lokal, cabin, kursi + filter dari pengguna
  ├─ dedupe         penerbangan yang sama dari beberapa provider -> penawaran termurah
  ├─ sort           best value (default) atau field yang diminta
  └─ handler        domain -> JSON (timestamp, "4h 20m", "Rp 595.000")
```

Setiap pencarian adalah daftar **segmen**, dan setiap segmen menjalankan pipeline ini secara paralel: satu segmen untuk satu arah, dua untuk round-trip (segmen kedua adalah rute yang dibalik), dan 2–5 untuk multi-city.

---

## Simulasi perilaku provider

Provider disimulasikan oleh `MockFetcher` dengan latensi dan tingkat keandalan sesuai permintaan assignment. Nilainya diatur di `cmd/server/providers.go`:

| Provider | Delay | Tingkat kegagalan |
|---|---|---|
| Garuda Indonesia | 50–100 ms | 0% |
| Lion Air | 100–200 ms | 0% |
| Batik Air | 200–400 ms | 0% |
| AirAsia | 50–150 ms | 10% |

Delay diacak dalam rentang tersebut di setiap panggilan, dan kegagalan mengembalikan `ErrProviderUnavailable`, persis seperti provider sungguhan yang menjawab dengan error 5xx.

Setiap skenario di bawah dimulai dari cache kosong, karena kalau tidak, hasil diambil dari Redis dan provider tidak dipanggil sama sekali:

```bash
docker compose exec redis redis-cli flushall
```

**Panggilan paralel.** Lakukan pencarian beberapa kali dan perhatikan `search_time_ms`: nilainya mengikuti provider paling lambat (sekitar 200–400 ms karena Batik), bukan jumlah keempatnya.

**Kegagalan provider dan retry.** Di `cmd/server/providers.go`, naikkan `FailureRate` AirAsia lalu restart:

- `0.5`: log menampilkan `fetch failed, retrying` beserta nomor percobaan dan backoff-nya; sebagian besar pencarian tetap berakhir dengan 4 provider sukses.
- `1.0`: AirAsia selalu gagal setelah 3 percobaan. Pencarian tetap mengembalikan 200 dengan flight dari provider lain dan `providers_failed: 1` (partial failure).

Kembalikan ke `0.1` setelahnya.

**Timeout.** Set `PROVIDER_TIMEOUT=150ms` di `.env` lalu restart. Batik (200–400 ms) tidak akan pernah sempat menjawab, sehingga gagal dengan `context deadline exceeded`, provider lain tetap menjawab, dan `search_time_ms` tidak pernah jauh di atas 150. Kembalikan ke `2s`.

**Rate limiting.** Lihat [Memverifikasi Redis](#memverifikasi-redis-cache-dan-rate-limiter): dengan `PROVIDER_RATE_LIMIT=1`, pencarian yang paralel berebut kuota yang sama.

**Cache dan singleflight.** Ulangi pencarian yang sama: jawaban kedua memiliki `cache_hit: true` dan selesai dalam ~0 ms. Mengirim banyak pencarian identik sekaligus saat cache kosong hanya memanggil provider sekali.

**Simulasi yang deterministik di test.** Test tidak bergantung pada keberuntungan: `MockFetcher` menerima random generator dengan seed tetap, sehingga tingkat kegagalan `1.0` selalu gagal, `0.0` tidak pernah gagal, dan nilai di antaranya menghasilkan urutan yang sama di setiap run.

---

## Data mock tambahan

Data dari assignment hanya berisi penerbangan CGK → DPS tanggal 15 Desember, sehingga pencarian round-trip dan multi-city akan selalu mengembalikan daftar kosong setelah segmen pertama. Untuk mendemokannya, **11 penerbangan** ditambahkan ke file provider:

| Rute dan tanggal | Dipakai oleh | Penerbangan |
|---|---|---|
| DPS → CGK, 20 Des | Round-trip | GA401, GA421 (Garuda), JT741, JT743 dengan 1 transit di SUB (Lion), ID6515 (Batik), QZ521, QZ535 (AirAsia) |
| DPS → SUB, 18 Des | Multi-city | JT693 (Lion), QZ641 (AirAsia) |
| SUB → CGK, 20 Des | Multi-city | GA313 (Garuda), ID6583 (Batik) |

- Ke-13 penerbangan asli tidak diubah. Pencarian satu arah CGK → DPS tetap mengembalikan tepat 13 penerbangan.
- Setiap entri mengikuti format provider-nya masing-masing, sehingga juga menguji normalizer.
- Penerbangan ke arah barat menguji kasus zona waktu yang berlawanan: QZ535 berangkat 20:00 WITA dan tiba 20:40 WIB, terlihat seperti 40 menit padahal sebenarnya 100 menit; JT693 (DPS → SUB) terlihat seperti 5 menit padahal sebenarnya 65 menit.
- Data tambahan ini sengaja dibuat konsisten: tidak menambah jebakan baru, sehingga setiap inkonsistensi yang tercantum di [DESIGN.id.md](DESIGN.id.md#inkonsistensi-data-yang-ditemukan-dan-ditangani) berasal dari file assignment asli.

---

## Testing

```bash
go test -race ./...
```

Unit test mencakup:

- **Normalizer**: penanganan multi-segmen GA315, konversi WIB → WIT, file mock yang di-embed berupa JSON valid.
- **Mock fetcher**: delay, tingkat kegagalan, pembatalan, penggunaan bersamaan (race detector).
- **Retry**: berhasil setelah kegagalan sementara, menyerah setelah percobaan maksimal, tidak pernah me-retry error lain, berhenti saat dibatalkan, pertumbuhan dan batas backoff.
- **Rate limiting**: menunggu slot yang dekat, langsung menyerah untuk slot yang jauh, fail open saat Redis mati, kuota per provider.
- **Search**: setiap filter, pencocokan tanggal lokal, rentang jam lewat tengah malam, sort dan tie-breaker, perilaku best value, aturan dedupe, input tidak pernah diubah.
- **Service**: cache hit di panggilan kedua, hasil parsial tidak di-cache, Redis mati tidak menggagalkan pencarian, 50 cache miss bersamaan hanya memanggil provider sekali (singleflight), kedua arah round-trip, urutan segmen dan validasi multi-city.
- **Cache, rate limiter, format uang.**

Test memakai fake yang di-inject lewat interface serta jam palsu, sehingga cepat dan deterministik.

---

## Memverifikasi Redis (cache dan rate limiter)

Perintah di bawah mengasumsikan Docker Compose (`docker compose exec redis redis-cli ...`).

**Apakah Redis hidup dan dipakai aplikasi?**

```bash
docker compose ps                              # redis: running (healthy)
docker compose exec redis redis-cli ping       # PONG
docker compose logs app | grep "config loaded" # store_driver: redis
docker compose logs app | grep "using redis"
```

**Cache**

```bash
# entri dan sisa umurnya
docker compose exec redis redis-cli keys 'flights:*'
docker compose exec redis redis-cli ttl "flights:v1:CGK:DPS:2025-12-15:economy"

# isi JSON yang di-cache
docker compose exec redis redis-cli get "flights:v1:CGK:DPS:2025-12-15:economy"

# memantau perintah secara langsung saat mencari (hanya untuk development)
docker compose exec redis redis-cli monitor

# counter hit / miss
docker compose exec redis redis-cli info stats | grep keyspace

# memaksa cache miss
docker compose exec redis redis-cli del "flights:v1:CGK:DPS:2025-12-15:economy"
```

Pencarian pertama mengembalikan `cache_hit: false` (~200–400 ms), pencarian kedua `cache_hit: true` (~0 ms). `keys` cukup untuk development; gunakan `scan` di instance produksi yang besar.

**Singleflight**: dengan cache kosong, 20 pencarian identik secara paralel menghasilkan 20 `GET` tapi hanya 1 `SET`:

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

**Rate limiter**: key rate limit kedaluwarsa dalam hitungan detik, sehingga `keys 'rate:*'` biasanya kosong. Periksa jumlah eksekusi script-nya. Untuk melihat request yang dibatasi, set `PROVIDER_RATE_LIMIT=1` di `.env`, buat ulang container, lalu kirim 4 pencarian dengan tanggal berbeda secara paralel:

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

Dengan 1 panggilan per detik dan batas waktu 2 detik, setiap provider melayani 2 dari 4 pencarian; sisanya gagal dengan `provider rate limit reached`. Kembalikan `PROVIDER_RATE_LIMIT=10` setelahnya.

---

## Keputusan desain

Alasan di balik implementasi ada di **[DESIGN.id.md](DESIGN.id.md)**: dimulai dengan ringkasan satu halaman, lalu detail setiap keputusan:

- abstraksi provider (`Fetcher` / `Provider`) serta decorator retry dan rate limit
- timestamp sebagai sumber kebenaran, dan penanganan zona waktu (WIB, WITA, WIT)
- agregasi paralel, timeout, dan partial failure
- retry dengan exponential backoff, rate limiting, caching, dan singleflight
- filter, sort, best value scoring, dan perbandingan harga antar-provider
- round-trip dan multi-city sebagai daftar segmen
- inkonsistensi data yang ditemukan di file provider, asumsi, performa, dan keterbatasan
