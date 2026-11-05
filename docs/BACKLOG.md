# Backlog FastConsignee

Pekerjaan dicicil per fitur, dan backend dikerjakan lebih dulu. Satu item baru selesai kalau testing-nya lolos. Sumber kebutuhannya ada di [PRD](../README.md), dan kontraknya ada di [api/openapi.yaml](../api/openapi.yaml).

## Aturan selesai

- Kode mengikuti layer handler, service, dan repository di dalam folder fiturnya.
- Query berurutan memakai satu transaksi database, dan transaksi tidak terbuka selama RPC Solana.
- Jawaban API divalidasi otomatis terhadap kontrak OpenAPI di test.
- `go vet`, `go test`, dan lint kontrak lolos sebelum item berikutnya dimulai.

## Backend

### B-00 Fondasi

Kebutuhan: NFR-08, NFR-16 sampai NFR-20, NFR-23, NFR-25.

- [x] Module Go, konfigurasi environment, dan `.env.example`
- [x] Koneksi Postgres, helper transaksi, dan migrasi semua tabel serta indeks
- [x] Error RFC 9457, request ID, log, recover, dan CORS
- [x] `GET /healthz` dan graceful shutdown
- [x] Test harness: database per test, klien API, dan validator kontrak
- [ ] CI GitHub Actions

Testing: unit, repository untuk constraint dan indeks, handler, kontrak.

### B-01 Auth

Kebutuhan: FR-25, FR-26, FR-27, NFR-24.

- [x] Verifikasi JWT Supabase, lewat HS256 atau JWKS
- [x] Pencocokan akun dan `GET /api/v1/me`
- [x] Penjaga peran dan penjaga wallet
- [x] Nonce dan penautan wallet dengan tanda tangan ed25519
- [x] Rate limit per akun, IP, dan order

Testing: unit, handler, keamanan untuk JWT palsu dan akses antarperan, kontrak.

### B-02 Barang

Kebutuhan: FR-01, FR-02, FR-22, NFR-22.

- [x] Daftar barang dengan pencarian, filter, dan cursor
- [x] Detail, tambah, dan ubah barang
- [x] Stok tertahan dan stok tersedia
- [x] Cache katalog

Testing: repository, service, handler, kontrak.

### B-03 Penitip

Kebutuhan: FR-23.

- [x] Daftar, detail, tambah, dan ubah penitip
- [x] Validasi alamat wallet dan status wallet

Testing: repository, handler, keamanan antartoko, kontrak.

### B-04 Order

Kebutuhan: FR-02 sampai FR-08, FR-11, FR-17, FR-28, FR-29, NFR-03, NFR-07.

- [x] Fungsi murni pembagian, diskon, potongan, dan konversi lamport
- [x] Penyedia kurs dengan cache dan nilai cadangan
- [x] Rincian pembagian
- [x] Buat order dengan reservasi stok, idempotency, dan pelunasan potongan
- [x] Daftar order, detail, dan status

Testing: unit, property dan fuzz, repository, concurrency untuk stok terakhir, handler, kontrak.

### B-05 Pembayaran

Kebutuhan: FR-07 sampai FR-14, NFR-01, NFR-04, NFR-12 sampai NFR-14, NFR-21.

- [ ] Layar bayar publik
- [ ] Solana Pay GET, POST, preflight, dan CORS
- [ ] Penyusun transaksi split dengan `reference`
- [ ] Worker konfirmasi, validasi, dan kedaluwarsa

Testing: unit isi transaksi, service dengan RPC tiruan, concurrency untuk konfirmasi ganda, kontrak, Devnet kalau airdrop tersedia.

### B-06 Refund

Kebutuhan: FR-15, FR-16, FR-17.

- [ ] Nilai refund per unit
- [ ] Buat refund, signature, daftar, dan detail
- [ ] Worker refund dan pencatatan potongan

Testing: unit, property, service dengan RPC tiruan, concurrency untuk refund ganda, kontrak.

### B-07 Dasbor

Kebutuhan: FR-16 sampai FR-20, NFR-22.

- [ ] Penjualan, potongan, dan stok penitip
- [ ] Ringkasan toko dengan cache

Testing: repository, handler, keamanan antarpenitip, kontrak.

### B-08 Penutup backend

Kebutuhan: FR-21, FR-24, NFR-02, NFR-15, NFR-25.

- [ ] Seed data 3 penitip
- [ ] Cara menjalankan backend di dokumentasi
- [ ] Uji beban, `gosec`, dan `govulncheck`
- [ ] Pembayaran nyata di Devnet
- [ ] Pengiriman NFT sertifikat

Testing: beban, keamanan, Solana Devnet.

## Frontend

Frontend dimulai setelah backend selesai.

### F-01 Fondasi dan login

Kebutuhan: FR-25, FR-27.

- [ ] Next.js, klien API dari kontrak, dan Supabase Auth
- [ ] Layar penautan wallet

### F-02 Kasir

Kebutuhan: FR-01 sampai FR-06, FR-10.

- [ ] Katalog, keranjang, rincian pembagian, QR, dan status

### F-03 Pembeli

Kebutuhan: FR-08, FR-13, FR-14.

- [ ] Layar bayar, wallet adapter, dan bukti pembayaran

### F-04 Dasbor dan refund

Kebutuhan: FR-15, FR-16, FR-18, FR-19, FR-20, FR-22, FR-23.

- [ ] Dasbor penitip, dasbor toko, refund, barang, dan penitip

### F-05 Uji akhir

Kebutuhan: NFR-05, NFR-10, NFR-11.

- [ ] Vitest, Testing Library, dan Playwright
- [ ] Uji Phantom di HP dan alur demo 5 kali

## Deploy

Bagian ini butuh akun pemilik proyek.

- [ ] Proyek Supabase, Google OAuth, dan migrasi
- [ ] Layanan Render, environment, dan cron `/healthz`
- [ ] Proyek Vercel
- [ ] Repo dibuat publik sebelum submit Devpost
