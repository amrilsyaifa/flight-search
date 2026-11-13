[English](DESIGN.md) | **Bahasa Indonesia**

# Keputusan Desain

Dokumen ini menjelaskan keputusan desain utama di balik Flight Search & Aggregation System: apa yang dipilih, alasannya, dan trade-off-nya. Untuk setup dan cara pakai, lihat [README](README.id.md).

---

## Daftar isi

- [Ringkasan](#ringkasan)
- [1. Provider: mengambil data dipisah dari memahami data](#1-provider-mengambil-data-dipisah-dari-memahami-data)
- [2. Timestamp adalah sumber kebenaran](#2-timestamp-adalah-sumber-kebenaran)
- [3. Zona waktu](#3-zona-waktu)
- [4. Agregasi paralel dengan partial failure](#4-agregasi-paralel-dengan-partial-failure)
- [5. Retry dengan exponential backoff](#5-retry-dengan-exponential-backoff)
- [6. Rate limiting](#6-rate-limiting)
- [7. Caching](#7-caching)
- [8. Filter dan sort](#8-filter-dan-sort)
- [9. Best value scoring](#9-best-value-scoring)
- [10. Perbandingan harga antar-provider](#10-perbandingan-harga-antar-provider)
- [11. Round-trip](#11-round-trip)
- [12. Multi-city](#12-multi-city)
- [13. Format response](#13-format-response)
- [14. API-first dengan OpenAPI](#14-api-first-dengan-openapi)
- [Asumsi](#asumsi)
- [Inkonsistensi data yang ditemukan dan ditangani](#inkonsistensi-data-yang-ditemukan-dan-ditangani)
- [Performa dan kompleksitas](#performa-dan-kompleksitas)
- [Keterbatasan dan pengembangan selanjutnya](#keterbatasan-dan-pengembangan-selanjutnya)

---

## Ringkasan

- **Provider** dibagi menjadi `Fetcher` (mengambil byte mentah) dan `Provider` (men-decode dan menormalisasi format satu maskapai). Retry dan rate limiting membungkus fetcher sebagai decorator, sehingga peralihan dari mock ke API HTTP sungguhan hanya mengubah bagian wiring.
- **Timestamp adalah sumber kebenaran.** Durasi selalu dihitung dari waktu berangkat dan tiba; durasi dari provider hanya dicek silang, karena beberapa di antaranya salah.
- **Waktu menyimpan offset lokal bandara** (WIB, WITA, WIT), dan tanggal dicocokkan dalam waktu lokal bandara asal, sehingga keberangkatan 04:45 WIB tetap terhitung penerbangan tanggal 15 Desember.
- **Provider dipanggil secara paralel** dengan satu timeout keseluruhan; provider yang gagal atau lambat hanya menghilangkan flight miliknya sendiri (partial failure), dan panic diisolasi.
- **Kegagalan sementara di-retry** dengan exponential backoff dan jitter. **Panggilan dibatasi per provider di Redis**, dipakai bersama oleh semua instance aplikasi.
- **Data agregasi provider di-cache** di Redis selama 2 menit, sebelum filter, sehingga filter yang berbeda memakai satu entri yang sama; hasil parsial tidak di-cache, dan cache miss identik yang bersamaan digabung dengan singleflight.
- **Best value** memeringkat berdasarkan harga (50%), durasi (30%), dan transit (20%), diukur relatif terhadap hasil terbaik setelah normalisasi min-max terbukti menyesatkan dengan data nyata.
- **Penerbangan yang sama dari beberapa provider** digabung dengan mempertahankan penawaran termurah, setelah filter.
- **Setiap pencarian adalah daftar segmen**: satu arah (1), round-trip (2), dan multi-city (2–5, endpoint terpisah) memakai satu jalur kode paralel yang sama.
- **API-first**: spec OpenAPI adalah kontraknya; kode server di-generate, dan build gagal kalau ada endpoint yang belum diimplementasikan.

---

## 1. Provider: mengambil data dipisah dari memahami data

Setiap provider dibagi menjadi dua peran:

- **`Fetcher`** mengambil byte response mentah. Saat ini `MockFetcher` mengembalikan JSON yang di-embed setelah delay simulasi; nantinya `HTTPFetcher` akan memanggil API sungguhan.
- **`Provider`** (`garuda`, `lion`, `batik`, `airasia`) men-decode format maskapainya dan menormalisasi setiap flight menjadi `models.Flight`.

Mock mengembalikan byte JSON mentah, bukan struct Go, sehingga jalur decoding yang sesungguhnya selalu teruji. Retry dan rate limiting ditambahkan sebagai **decorator** di sekitar fetcher:

```
RetryFetcher -> RateLimitedFetcher -> MockFetcher
```

Provider tidak pernah tahu lapisan-lapisan ini ada. Beralih ke API sungguhan berarti menulis satu `HTTPFetcher` dan mengubah `cmd/server/providers.go`; normalizer, aggregator, cache, dan search tidak tersentuh.

Mock berperilaku seperti HTTP client sungguhan dalam hal-hal yang penting: menghormati pembatalan context (menunggu dengan `select`, bukan `time.Sleep`), mengembalikan jenis error yang sama (`ErrProviderUnavailable`), dan sumber randomness-nya bisa di-inject untuk test yang deterministik.

## 2. Timestamp adalah sumber kebenaran

Beberapa provider melaporkan durasi yang tidak sesuai dengan waktu berangkat dan tiba milik mereka sendiri (lihat [inkonsistensi data](#inkonsistensi-data-yang-ditemukan-dan-ditangani)). Aturannya sederhana: **durasi selalu `tiba - berangkat`**, dihitung dari momen absolut. Durasi dari provider hanya dipakai untuk pengecekan silang, dan selisihnya dicatat di log beserta kedua nilainya.

Nilai turunan (`timestamp`, `"4h 20m"`, `"Rp 595.000"`, jumlah stop) tidak pernah disimpan di model domain. Semuanya dihitung saat response dibentuk, sehingga tidak mungkin tidak sinkron. File `expected_result.json` dari soal menunjukkan kenapa ini penting: `timestamp`-nya tidak sesuai dengan `datetime`-nya sendiri.

## 3. Zona waktu

- Setiap waktu menyimpan **offset lokal bandaranya** (`+07:00` WIB, `+08:00` WITA, `+09:00` WIT), sehingga API menampilkan waktu lokal seperti yang diharapkan penumpang.
- Lion Air mengirim waktu tanpa offset ditambah nama zona IANA; waktu ini di-parse dengan `time.ParseInLocation`. Batik mengirim `+0700` (tanpa titik dua), yang butuh layout tersendiri.
- **Tanggal dicocokkan dalam waktu lokal bandara asal.** QZ520 berangkat 04:45 WIB tanggal 15 Desember, yang dalam UTC adalah 21:45 tanggal 14. Kalau dibandingkan dalam UTC, flight ini akan hilang dari pencarian tanggal 15. Tipe `Date` khusus (tanpa jam, tanpa zona) mencegah hal ini.
- Filter jam (`departureTimeFrom`, dan lainnya) juga memakai waktu lokal bandara.
- Database zona IANA di-embed (`time/tzdata`), sehingga lookup zona tetap bekerja di image distroless yang minimal.
- WIT dibuktikan lewat unit test (Jakarta → Jayapura): 05:30 WIB sampai 12:30 WIT adalah 5 jam, bukan 7 jam seperti yang terlihat dari jam lokal.

## 4. Agregasi paralel dengan partial failure

Aggregator menjalankan satu goroutine per provider dan menunggu dengan `sync.WaitGroup`. Waktu pencarian kira-kira sama dengan provider **paling lambat** (Batik, 200–400 ms), bukan jumlah keempatnya (sampai sekitar 850 ms).

- **Satu timeout keseluruhan** (`PROVIDER_TIMEOUT`) membatasi seluruh pencarian, termasuk retry.
- **Partial failure**: provider yang gagal atau lambat hanya menghilangkan flight miliknya sendiri; pencarian tetap mengembalikan 200 dengan `providers_failed` dilaporkan.
- Setiap goroutine hanya menulis ke slot miliknya sendiri di slice hasil, sehingga tidak perlu mutex dan urutan hasil deterministik.
- Panic di dalam provider di-recover di goroutine-nya dan dihitung sebagai kegagalan. Tanpa ini, satu provider yang bermasalah bisa menjatuhkan seluruh server.
- `errgroup.WithContext` sengaja tidak dipakai: ia membatalkan semua goroutine begitu satu gagal, kebalikan dari yang dibutuhkan partial result.

## 5. Retry dengan exponential backoff

Hanya `ErrProviderUnavailable` (kegagalan sementara) yang di-retry, maksimal 3 percobaan, dengan jeda sekitar 50 ms → 100 ms → 200 ms ditambah **jitter** supaya banyak client tidak retry di saat yang sama. Error context dan error decoding tidak pernah di-retry. Penantiannya bisa dibatalkan, sehingga retry langsung berhenti begitu timeout keseluruhan tercapai.

Dengan tingkat kegagalan AirAsia 10%, 3 percobaan menurunkan kegagalan yang terlihat menjadi sekitar 0,1 × 0,1 × 0,1 = 0,1%. Trade-off-nya adalah latensi sedikit lebih tinggi saat retry terjadi, yang tetap dibatasi oleh timeout keseluruhan.

## 6. Rate limiting

Assignment meminta pembatasan panggilan **ke** provider. Kuota provider berlaku untuk seluruh sistem kita, berapa pun instance yang berjalan, sehingga counter-nya disimpan di **Redis** (algoritma GCRA lewat `redis_rate`, dijalankan secara atomik dalam Lua script).

- Limiter berada **di dalam** decorator retry, sehingga setiap percobaan ulang juga memakai kuota.
- Kalau kuota habis, fetcher **menunggu** jika slot berikutnya masih muat dalam batas waktu request, dan **langsung menyerah** dengan `ErrRateLimited` jika tidak.
- `ErrRateLimited` tidak di-retry: retry hanya akan menambah tekanan.
- Kalau Redis tidak bisa dihubungi, panggilan **tetap diizinkan (fail open)** dan warning dicatat. Untuk pencarian tiket, melewati kuota sebentar lebih bisa diterima daripada menghentikan semua pencarian.

## 7. Caching

- **Yang di-cache**: data provider yang sudah diagregasi dan dinormalisasi, **sebelum** filter dan sort. Pencarian untuk rute dan tanggal yang sama memakai satu entri, apa pun filternya.
- **Key**: `flights:v1:{origin}:{destination}:{date}:{cabin}`. Jumlah penumpang tidak dimasukkan karena hanya memengaruhi filter kursi, yang berjalan setelah cache. `v1` memungkinkan semua entri dibatalkan cukup dengan menaikkan versi.
- **TTL 2 menit**: harga berubah, jadi data basi harus cepat kedaluwarsa, sementara pencarian populer tetap mendapat manfaat cache.
- **Hasil parsial tidak di-cache**: kalau tidak, satu kegagalan AirAsia akan menyembunyikan flight-nya selama TTL.
- **Kegagalan saat berjalan menurunkan performa, kegagalan saat start menghentikan aplikasi**: kalau Redis mati saat aplikasi berjalan, pencarian menjadi lebih lambat tapi tetap bekerja. Kalau Redis tidak bisa dihubungi saat start, aplikasi berhenti dengan pesan yang jelas, karena itu kesalahan konfigurasi.
- **Perlindungan cache stampede**: `singleflight` menggabungkan pencarian identik yang sama-sama cache miss di saat yang sama, sehingga 20 request bersamaan hanya memicu satu pengambilan ke provider. Pengambilan bersama itu memakai `context.WithoutCancel`, sehingga satu client yang memutus koneksi tidak menggagalkan client lain.
- Filter dan dedupe tidak pernah mengubah input-nya, sehingga hasil cache atau singleflight aman dipakai bersama oleh banyak request.

## 8. Filter dan sort

- Filter memakai pointer supaya "tidak diisi" berbeda dari nol: `maxStops: 0` berarti hanya penerbangan langsung, yang tidak sama dengan tanpa filter.
- Kriteria (rute, tanggal lokal, cabin, kursi ≥ penumpang) selalu dicek ulang, walaupun provider "seharusnya" hanya mengembalikan penerbangan yang cocok.
- Rentang jam dengan `from > to` melewati tengah malam (`22:00`–`05:00` = penerbangan malam).
- Sort bersifat stabil dengan tie-breaker yang deterministik (harga, lalu jam berangkat, lalu ID), sehingga input yang sama selalu menghasilkan urutan yang sama.

## 9. Best value scoring

Skor lebih kecil berarti lebih baik:

```
skor = 0,5 × (harga / hargaTermurah − 1)
     + 0,3 × (durasi / durasiTercepat − 1)
     + 0,2 × jumlahTransit
```

Harga dan durasi diukur **relatif terhadap hasil terbaik**: "20% lebih mahal dari yang termurah" bernilai 0,2.

Versi pertama memakai normalisasi min-max (setiap faktor diskalakan antara nilai minimum dan maksimum). Saat diuji dengan data nyata, muncul dua masalah:

- Pada rentang yang sempit, min-max **membesarkan** selisih kecil. Setelah difilter, durasi hanya berkisar 100–110 menit, sehingga 5 menit dihitung sebagai setengah skala, dan JT740 (Rp 950.000, 105 menit) mengalahkan JT742 (Rp 890.000, 110 menit).
- Outlier (GA315 seharga Rp 1.850.000) **menekan** selisih di antara harga normal, dan semua penerbangan transit berada di bawah semua penerbangan langsung, bahkan GA410 seharga Rp 1.450.000 berada di atas QZ7250 seharga Rp 485.000.

Skor relatif menyelesaikan keduanya: JT742 kini mengalahkan JT740, dan QZ7250 yang murah dengan satu transit kini berada di atas penerbangan langsung yang mahal, tapi tetap di bawah penerbangan langsung dengan harga wajar. Bobot (harga paling utama, lalu durasi, lalu transit) adalah keputusan produk, disimpan sebagai konstanta bernama dan dikunci oleh test yang memeriksa perilaku, bukan angka skor yang persis.

Skor dihitung sekali sebelum sort (O(n)), tidak pernah di dalam fungsi pembanding.

## 10. Perbandingan harga antar-provider

Penerbangan yang sama bisa dijual oleh lebih dari satu provider. Duplikat digabung dan **penawaran termurah** dipertahankan. Sebuah penerbangan dikenali dari:

- **nomor penerbangan**, dinormalisasi (`"GA 400"` = `"GA400"`), karena nomor saja berulang setiap hari,
- **waktu keberangkatan** (Unix time), sehingga format waktu yang berbeda tetap cocok,
- **cabin class**, karena economy dan business di pesawat yang sama adalah produk berbeda.

Dedupe dijalankan **setelah filter**. Contoh: penawaran A lebih murah tapi sisa 1 kursi, penawaran B sisa 10 kursi. Untuk 2 penumpang, kalau dedupe dijalankan lebih dulu, A yang dipertahankan, lalu filter kursi membuangnya, dan penerbangan itu hilang padahal B bisa melayani pengguna.

Data assignment tidak berisi penerbangan yang dijual dua provider, sehingga perilaku ini dibuktikan lewat unit test.

## 11. Round-trip

- Response berisi dua daftar, `flights` dan `return_flights`, bukan setiap kombinasi pergi × pulang (13 × 13 = 169 paket). Pengguna memilih satu dari masing-masing, pola yang umum untuk tiket domestik satu arah.
- Arah pulang hanyalah pencarian satu arah dengan origin dan destination ditukar (`SearchCriteria.ReturnLeg()`), sehingga cache, singleflight, filter tanggal lokal, dedupe, dan best value berlaku tanpa perubahan.
- Kedua arah berjalan paralel. Filter dan sort berlaku untuk kedua arah.
- Metadata: jumlah provider dijumlahkan dari kedua arah, `cache_hit` bernilai true hanya kalau kedua arah hit, `total_results` menghitung kedua daftar.
- Response satu arah tidak berubah: `return_flights` tidak dicantumkan.

## 12. Multi-city

- **Endpoint terpisah** (`/api/v1/flights/search/multi-city`). Menaruh segmen di request yang sudah ada akan membuat `origin`, `destination`, dan `departureDate` menjadi wajib secara bersyarat, yang sulit diekspresikan dengan baik oleh OpenAPI dan tipe hasil generate. Kontrak terpisah membuat kedua request tetap sederhana dan API satu arah/round-trip tidak berubah.
- **Semuanya adalah daftar segmen.** Di dalam service ada satu fungsi, `searchLegs`, yang menjalankan berapa pun segmen secara paralel dan menjaga urutannya. Satu arah adalah 1 segmen, round-trip 2 segmen, multi-city 2–5. Round-trip di-refactor memakai fungsi yang sama, sehingga hanya ada satu jalur kode untuk dites.
- Setiap segmen melewati pipeline biasa dengan cache key-nya sendiri, sehingga segmen yang pernah dicari (lewat endpoint mana pun) diambil dari cache.
- **Validasi**: 2–5 segmen (kurang dari itu bukan multi-city, lebih dari itu memicu terlalu banyak panggilan ke provider), setiap segmen valid, dan tanggal tidak pernah mundur. Segmen boleh tidak bersambung (open-jaw), seperti di kebanyakan aplikasi pencarian tiket.
- Jumlah penumpang, cabin, filter, dan sort berlaku untuk semua segmen.
- Metadata mengikuti aturan round-trip, dijumlahkan dari semua segmen.

## 13. Format response

- Urutan field sesuai `expected_result.json` (`x-order` di spec OpenAPI; tanpa itu oapi-codegen mengurutkan field secara alfabetis).
- `aircraft` bernilai `null` kalau tidak diketahui; `amenities` bernilai `[]`, tidak pernah `null`.
- `price.amount` tetap berupa angka untuk perhitungan; `price.formatted` (`"Rp 1.250.000"`) untuk ditampilkan.

## 14. API-first dengan OpenAPI

`api/openapi.yaml` adalah kontraknya. Tipe server dan routing di-generate dengan oapi-codegen (strict server di atas `net/http`, tanpa framework). Pengecekan saat compile (`var _ oapi.StrictServerInterface = (*Server)(nil)`) membuat build gagal kalau ada endpoint di spec yang belum diimplementasikan. Kode hasil generate ikut di-commit supaya project bisa di-build tanpa generator.

## Asumsi

- Angka bagasi Garuda (`"carry_on": 1, "checked": 2`) dianggap sebagai jumlah koli, karena tidak ada satuannya.
- Harga AirAsia dalam IDR (nama field-nya `price_idr`).
- Kode fare Batik `Y` adalah economy (W premium economy, C/J business, F first).
- AirAsia tidak mengirim kode maskapai; kode diambil dari nomor penerbangan (`QZ7250` → `QZ`).
- `totalPrice` Batik (harga dasar + pajak) adalah harga yang dibayar pelanggan.

---

## Inkonsistensi data yang ditemukan dan ditangani

| Lokasi | Masalah | Penanganan |
|---|---|---|
| Garuda GA315 | Top-level menyatakan tiba di **SUB** dengan `stops: 0`; segmennya menunjukkan CGK → SUB → DPS | Keberangkatan dari segmen pertama, kedatangan dari segmen terakhir, layover dari jeda antar-segmen |
| Garuda GA315 | Segmen kedua mengaku 90 menit; SUB 17:15 WIB → DPS 18:45 WITA sebenarnya **30 menit** | Durasi total dari timestamp: **225 menit** (menjumlahkan field segmen menghasilkan 285) |
| Batik ID7042 | `travelTime: "3h 5m"` (185 menit), durasi sebenarnya **245 menit** (zona waktu terlupakan) | Memakai timestamp, selisih dicatat di log |
| AirAsia | `duration_hours` berupa float yang dibulatkan (1,67 jam = 100,2 menit) | Memakai timestamp; selisih 1 menit dianggap pembulatan |
| Lion Air | Waktu tanpa offset, zona di field terpisah | `time.ParseInLocation` dengan zona yang di-cache |
| Batik Air | Offset tanpa titik dua (`+0700`) | Layout waktu tersendiri |
| Beberapa provider | Kota, aircraft, amenities, kode maskapai tidak ada | Lookup kota dari bandara, `null` / `[]` di response, kode dari nomor penerbangan |
| Lion / Batik / AirAsia | Transit dijelaskan oleh 2–3 field yang bisa saling bertentangan | Field dicek silang; flight yang tidak konsisten dilewati dan dicatat |
| `expected_result.json` | `timestamp` 1734246900 jatuh di tahun **2024**, bukan 2025, dan tidak sesuai dengan `datetime`-nya; `total_results` bernilai 15 padahal data hanya berisi 13 penerbangan | Diperlakukan sebagai contoh format, bukan nilai yang harus sama persis |

Flight yang tidak valid (waktu tidak bisa di-parse, waktu tiba tidak setelah berangkat, cabin tidak dikenal, harga tidak positif, transit tidak konsisten) **dilewati satu per satu dan dicatat di log**; flight tersebut tidak pernah menggagalkan seluruh provider.

---

## Performa dan kompleksitas

| Langkah | Biaya |
|---|---|
| Panggilan provider | Paralel: ≈ provider paling lambat (200–400 ms), bukan jumlahnya |
| Cache hit | ~0 ms, provider dilewati |
| Filter | O(n), filter maskapai memakai set (lookup O(1)) |
| Dedupe | O(n), satu lookup map per flight |
| Best value | O(n), skor dihitung sekali sebelum sort |
| Sort | O(n log n) |
| Lookup zona waktu | Di-cache setelah pemakaian pertama |

---

## Keterbatasan dan pengembangan selanjutnya

- **Provider belum menerima kriteria pencarian.** Mock mengembalikan seluruh isi file-nya dan service memfilter setelahnya. Dengan API sungguhan, kriteria akan diteruskan sampai ke fetcher (`Fetch(ctx, criteria)`). Perubahannya mekanis tapi menyentuh setiap provider. Karena itu pula, pencarian round-trip dan multi-city saat ini mengambil data mock yang sama sekali per segmen.
- **Hasil multi-city per segmen**, bukan itinerary gabungan dengan total harga atau pengecekan sambungan antar-segmen (misalnya segmen 2 berangkat setelah segmen 1 mendarat di hari yang sama). Itu langkah berikutnya untuk alur pemesanan.
- **Health check** belum memeriksa Redis. Endpoint readiness yang melakukan ping ke Redis akan membantu orchestrator.
- **Observability**: belum ada middleware logging per request, request ID, maupun metrik (hit rate cache, latensi provider, kegagalan per provider).
- **Format error untuk body JSON yang rusak** masih berasal dari handler default kode hasil generate. Handler `StrictHTTPServerOptions` kustom akan membuatnya memakai bentuk error JSON yang sama dengan error lainnya.
- **Bobot best value** berupa konstanta tetap. Bobot ini bisa dibuat dapat dikonfigurasi atau disetel dengan data pemesanan nyata.

---

Kembali ke [README](README.id.md).
