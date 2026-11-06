# Backend FastConsignee

API Go untuk kasir konsinyasi. Kontraknya ada di [api/openapi.yaml](../api/openapi.yaml).

## Yang dibutuhkan

- Go sesuai versi di `go.mod`
- PostgreSQL 16, atau proyek Supabase
- Proyek Supabase dengan login Google

## Menjalankan di komputer

Semua variabel ada di [.env.example](.env.example). Backend membaca environment, bukan file `.env`, jadi muat dulu ke shell:

```bash
set -a; source .env; set +a
```

Isi data demo. Email pemilik harus email Google yang kamu pakai untuk login, karena toko tidak punya pendaftaran mandiri:

```bash
SEED_OWNER_EMAIL=kamu@gmail.com go run ./cmd/seed
```

Seed membuat satu toko, satu kasir, tiga penitip, dan sembilan barang. Seed aman dijalankan berulang dan tidak menimpa data yang sudah ada.

Jalankan API:

```bash
go run ./cmd/api
```

Migrasi berjalan otomatis saat `AUTO_MIGRATE=true`. Cek dengan `GET /healthz`.

## Menyiapkan wallet Devnet

1. Buka Phantom, lalu aktifkan Testnet Mode dan pilih Solana Devnet.
2. Isi wallet pembeli dengan SOL Devnet dari faucet web. Faucet lewat RPC publik sudah ditutup dan menjawab `410 Gone`.
3. Login sebagai pemilik, lalu tautkan wallet toko. Order ditolak selama wallet toko belum tertaut.
4. Seed membuat wallet penitip acak yang kunci privatnya dibuang. Untuk melihat saldo penitip di Phantom, isi `SEED_CONSIGNOR_WALLETS` dengan tiga alamat milikmu.
5. Penitip yang ingin membuka dasbor harus login dan membuktikan wallet-nya dengan tanda tangan.

## Mencoba API di Postman

Import [api/postman/FastConsignee.postman_collection.json](../api/postman/FastConsignee.postman_collection.json). Koleksi ini dibuat dari kontrak. Kalau kontrak berubah, jalankan `npm run postman` di folder `api`, lalu import lagi.

1. Pastikan `http://localhost:3000` ada di Redirect URLs Supabase.
2. Buka alamat ini di browser, ganti `PROJECT` dengan proyek Supabase-mu, lalu login Google:
   `https://PROJECT.supabase.co/auth/v1/authorize?provider=google&redirect_to=http://localhost:3000`
3. Salin nilai `access_token` dari address bar ke variabel `bearerToken` di koleksi. Kalau API menjawab `401`, ambil token baru.
4. Jalankan request dari atas ke bawah. ID hasil `POST` tersimpan otomatis, dan `Idempotency-Key` selalu baru di tiap kirim.

Peran yang boleh memanggil tiap request tertulis di deskripsinya. Akun pemilik menjawab `403 WALLET_NOT_LINKED` sampai wallet toko tertaut. Postman tidak bisa menandatangani dengan wallet, jadi `wallet_signature` dan `refund_signature` harus diisi dari hasil tanda tangan wallet.

## Testing

Test butuh Postgres dengan akun yang boleh membuat database, karena tiap test memakai database sendiri:

```bash
export TEST_DATABASE_URL="postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable"
```

```bash
go test ./...
```

Race detector butuh CGO, dan berjalan di CI:

```bash
go test -race ./...
```

Fuzz untuk perhitungan uang:

```bash
go test -fuzz=FuzzComputeLine -fuzztime=30s ./internal/order
```

```bash
go test -fuzz=FuzzBuildSplit -fuzztime=30s ./internal/order
```

```bash
go test -fuzz=FuzzUnitValues -fuzztime=30s ./internal/refund
```

Uji beban checkout, memakai RPC palsu:

```bash
go test -tags load -run TestCheckoutLatencyUnderLoad -v ./internal/order
```

Uji jaringan Devnet. Yang pertama hanya membaca dan tidak butuh saldo:

```bash
go test -tags devnet -run TestDevnetReadsRealNetwork -v ./internal/payment
```

Yang kedua mensimulasikan transaksi bayar dan NFT di program asli, jadi juga tidak butuh saldo:

```bash
go test -tags devnet -run 'TestDevnet(Certificate|Payment)Simulates' -v ./internal/payment
```

Uji pembayaran, refund, dan NFT sungguhan butuh wallet buangan berisi sekitar 0,3 SOL Devnet. Jangan pernah memakai kunci wallet utama:

```bash
DEVNET_PAYER_SECRET=kunci_base58 go test -tags devnet -run 'TestDevnetPaymentAndRefund|TestDevnetCertificateMints' -v ./internal/payment
```

## Deploy ke Render

- Build command: `go build -o bin/api ./cmd/api`
- Start command: `./bin/api`
- Root directory: `backend`
- `DATABASE_URL` memakai Session Pooler Supabase, karena koneksi langsung Supabase hanya IPv6.
- `TRUSTED_PROXY_HOPS=1`
- `PUBLIC_API_URL` dan `FRONTEND_URL` diisi alamat asli, tanpa garis miring di akhir.
- `CERTIFICATE_SIGNER_SECRET` diisi kunci wallet Devnet khusus untuk NFT sertifikat. Satu sertifikat memakai sekitar 0,0163 SOL (hasil simulasi di Devnet). Kalau kosong, sertifikat ditandai `failed` setelah `CERTIFICATE_DEADLINE`.
- `FIXED_IDR_PER_SOL` wajib diisi. Tanpa nilai ini, API menjawab `503 RATE_UNAVAILABLE` saat CoinGecko tidak terjangkau dan belum ada kurs pasar terakhir.
- Akun yang pernah login lokal terikat ke subject `local:...`. Login Google dengan email terverifikasi yang sama mengambil alih akun itu.
- Layanan cron eksternal memanggil `/healthz` tiap 10 menit agar instance gratis tidak tidur.

RPC publik Devnet membatasi `getTransaction`. Worker mencoba lagi pada tick berikutnya saat kena batas ini, jadi konfirmasi hanya melambat. Untuk order yang lebih ramai, isi `SOLANA_RPC_URL` dengan endpoint Devnet gratis dari penyedia RPC.

## Struktur

```text
cmd/api          titik masuk API
cmd/seed         data demo
migrations       skema database
internal/app     perakitan rute dan worker
internal/<fitur> auth, item, consignor, order, payment, refund, dashboard
internal/seed    isi data demo
internal/testkit alat test, termasuk validasi kontrak
internal/platform api, apperr, cache, config, database, middleware, solana
```

Tiap fitur memakai lapisan handler, service, dan repository.
