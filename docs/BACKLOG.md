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
- [x] Workflow CI GitHub Actions
- [x] Run CI pertama lolos di GitHub, termasuk `-race`

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

- [x] Layar bayar publik
- [x] Solana Pay GET, POST, preflight, dan CORS
- [x] Penyusun transaksi split dengan `reference`
- [x] Worker konfirmasi, validasi, dan kedaluwarsa

Testing: unit isi transaksi, service dengan RPC tiruan, concurrency untuk konfirmasi ganda, kontrak, Devnet dengan wallet berisi SOL.

### B-06 Refund

Kebutuhan: FR-15, FR-16, FR-17.

- [x] Nilai refund per unit
- [x] Buat refund, signature, daftar, dan detail
- [x] Worker refund dan pencatatan potongan

Testing: unit, property, service dengan RPC tiruan, concurrency untuk refund ganda, kontrak.

### B-07 Dasbor

Kebutuhan: FR-16 sampai FR-20, NFR-22.

- [x] Penjualan, potongan, dan stok penitip
- [x] Ringkasan toko dengan cache

Testing: repository, handler, keamanan antarpenitip, kontrak.

### B-08 Penutup backend

Kebutuhan: FR-21, FR-24, NFR-02, NFR-15, NFR-25.

- [x] Seed data 3 penitip
- [x] Cara menjalankan backend di dokumentasi
- [x] Uji beban, `gosec`, dan `govulncheck`
- [x] Koleksi Postman dari kontrak, diuji lewat Newman
- [ ] Pembayaran nyata di Devnet. Transaksinya sudah lolos simulasi di Devnet, tetapi `requestAirdrop` dijawab `410` karena faucet RPC ditutup, dan faucet web memakai CAPTCHA. Uji ini butuh wallet Devnet yang sudah punya saldo
- [ ] Login dengan token Supabase asli
- [x] Kode pengiriman NFT sertifikat, lolos uji worker dan simulasi di program Token, Associated Token, dan Token Metadata yang asli di Devnet
- [ ] Pengiriman NFT sertifikat nyata di Devnet, menunggu `CERTIFICATE_SIGNER_SECRET` berisi wallet yang punya saldo. Satu sertifikat memakai sekitar 0,0163 SOL

Testing: beban, keamanan, Solana Devnet.

## Frontend

Frontend dimulai setelah kode backend selesai. Sisa uji nyata backend menyusul saat Supabase dan wallet Devnet siap.

### F-01 Fondasi dan login

Kebutuhan: FR-25, FR-27.

- [x] Next.js dan klien API dari kontrak
- [x] Login lokal untuk pengembangan
- [x] Layar penautan wallet
- [x] Tema amber di aplikasi
- [ ] Login Google lewat Supabase Auth. Kodenya sudah ada dan lolos Vitest, tetapi belum diuji dengan proyek Supabase asli

Testing: unit dan komponen dengan Vitest, serta uji browser yang mengirim tanda tangan ed25519 asli ke API Go. Phantom sungguhan diuji di F-05.

### F-02 Kasir

Kebutuhan: FR-01 sampai FR-06, FR-10.

- [x] Katalog, keranjang, rincian pembagian, QR, dan status

Testing: komponen dengan Vitest, serta Playwright terhadap API Go dengan Solana tiruan, termasuk 4 transfer dalam satu transaksi.

### F-03 Pembeli

Kebutuhan: FR-08, FR-13, FR-14.

- [x] Layar bayar, wallet adapter, dan bukti pembayaran
- [ ] Pembayaran sungguhan di Devnet lewat wallet

Testing: komponen dengan Vitest, serta Playwright dengan wallet tiruan lewat Wallet Standard. Pembayaran di Devnet butuh wallet berisi SOL.

### F-04 Dasbor dan refund

Kebutuhan: FR-15, FR-16, FR-18, FR-19, FR-20, FR-22, FR-23.

- [x] Dasbor penitip, dasbor toko, refund, barang, dan penitip
- [x] Pilihan sertifikat di form barang, dan status sertifikat di bukti pembayaran
- [x] Tampilan dasbor dan menu samping mengikuti referensi Datta Able, dengan warna landing page

Testing: komponen dengan Vitest, serta Playwright untuk refund, potongan di dasbor penitip, hak akses tiap peran, wallet yang salah, dan menu samping di desktop dan HP.

### F-05 Uji akhir

Kebutuhan: NFR-05, NFR-10, NFR-11.

- [x] Vitest, Testing Library, dan Playwright
- [x] Pemeriksaan web di lokal oleh pilot, 6 November 2026
- [x] Pengujian menyeluruh di lokal, 6 November 2026. Hasil dan kendalanya ada di bagian Kendala dari pengujian
- [ ] Uji Phantom di HP dan alur demo 5 kali
- [ ] Uji landing page langsung di Firefox

### F-06 Landing page

Kebutuhan: belum ada di PRD. Rencana dan keputusannya ada di [FRONTEND.md](FRONTEND.md).

- [x] Landing page dari template Linea di `/`
- [ ] Tombol utama mengarah ke video demo
- [x] Halaman login mengikuti template Linea

Testing: Playwright di lebar 375 dan 1440 px, dengan reduced motion, dan rewrite `/` di `next start`. Halaman login juga diuji begitu, termasuk kembali ke `/`. Firefox belum diuji langsung.

### F-07 Performa

Kebutuhan: belum ada di PRD. Angkanya diukur dari build produksi lokal pada 6 November 2026, sebelum dan sesudah perbaikan.

- [x] Kasir membaca saldo tanpa `@solana/web3.js`, jadi JavaScript-nya turun dari 278 KB ke 192 KB setelah gzip
- [x] Font Inter di landing dikirim dari server sendiri, bukan dari Google Fonts
- [x] Foto hero landing dan latar login punya versi kecil untuk HP, jadi unduhan fotonya turun dari 188 KB ke 81 KB
- [x] Font monospace tidak lagi diunduh di awal setiap halaman
- [x] Layar bayar hanya memuat `@solana/web3.js` saat pembeli menekan "Bayar dengan wallet", jadi pembeli yang memindai QR tidak mengunduhnya. Memuatnya otomatis setelah layar tampil malah menurunkan nilainya ke 81, jadi cara itu dibatalkan. Belum diuji dengan Phantom asli
- [x] Kalau berkas wallet gagal dimuat, layar bayar memberi pesan dan QR tetap bisa dipakai
- [x] Uji tab yang masih terbuka saat ada deploy baru, dengan dua build berbeda. Tab lama tetap login, tetap bisa pindah layar, dan kasir tetap bisa membuat QR. Layar yang belum di-prefetch dimuat ulang penuh oleh Next.js
- [ ] Foto latar login menjadi LCP di 2,6 detik, dan belum dimuat lebih awal
- [ ] Foto hero landing masih dari Unsplash. Menyimpannya di server sendiri butuh izin mengunduh berkas
- [ ] Ukur PageSpeed beberapa kali setelah web online, lalu ambil nilai tengahnya
- [ ] Ukur latensi API setelah deploy, karena angka lokal belum memuat jarak ke Render dan Supabase. Render gratis juga tidur kalau tidak ada lalu lintas, jadi cron `/healthz` harus jalan sebelum demo

Kesimpulan: web ini kencang untuk HP, selama diukur di lokal. Tidak ada halaman yang berat kecuali refund dan wallet, karena keduanya memuat `@solana/web3.js`.

Lighthouse mode HP di build produksi lokal, nilai tengah 3 kali uji:

| Halaman | Performa | LCP | Bobot |
| --- | --- | --- | --- |
| Landing | 89 ke 99 | 3,2 ke 2,0 detik | 258 ke 155 KB |
| Login | 92 sampai 97 ke 96 sampai 97 | 2,7 sampai 3,3 ke 2,6 detik | 425 ke 286 KB |
| Bayar, dengan order asli | 91 ke 94 | 3,3 ke 2,8 detik | 390 ke 253 KB |

Nilai aksesibilitas Lighthouse: login dan bayar 100, landing 96. Sisa temuan landing berasal dari teks animasi yang belum terlihat sebelum di-scroll, dan axe bersih saat animasi dimatikan.

Waktu sampai isi layar tampil, memakai HP lambat tiruan dengan CPU 4 kali lebih lambat dan jaringan setara Slow 4G:

| Layar | Kunjungan pertama | Kunjungan ulang | Unduhan pertama | JavaScript gzip |
| --- | --- | --- | --- | --- |
| Kasir | 2,7 detik | 1,2 detik | 225 KB | 192 KB |
| Dasbor | 2,2 detik | 0,6 detik | 213 KB | 187 KB |
| Barang | 2,7 detik | 1,2 detik | 230 KB | 184 KB |
| Refund | 2,9 detik | 0,7 detik | 364 KB | 324 KB |
| Bayar | 2,9 detik | 0,7 detik | 350 KB | 290 KB |

Wallet memuat 319 KB, penitip 185 KB, dan login 177 KB JavaScript setelah gzip. Tanpa pembatas, semua layar tampil dalam 0,2 detik. Baris bayar di tabel diukur sebelum kode wallet dipisah, jadi angkanya belum diperbarui.

API lokal menjawab endpoint baca dengan p95 di bawah 7 ms, dari 200 panggilan per endpoint. Checkout menangani 103 order per detik dengan p95 233 ms. Angka ini bukan pengganti PageSpeed dan latensi setelah deploy.

## Kendala dari pengujian

Pengujian menyeluruh dijalankan di lokal pada 6 November 2026, memakai database uji terpisah dan Solana tiruan. Tabel ini memuat hasil setelah perbaikan.

| Uji | Hasil |
| --- | --- |
| `go vet`, `go build`, `go test` | Lolos, cakupan 84,7% |
| `go test -race` | Lolos di image Docker Go, karena Windows ini tidak punya gcc |
| Fuzz 3 fungsi, masing-masing 10 detik | Lolos |
| Uji beban 1.000 order dengan 20 pekerja | Lolos, p95 284 ms saat uji lain ikut berjalan. Uji sebelumnya tanpa beban lain: 103 order per detik, p95 233 ms |
| `gosec` dan `govulncheck` | Lolos, tanpa celah yang terpanggil kode |
| Uji baca Devnet | Lolos |
| Simulasi transaksi bayar dan NFT di Devnet | Lolos di program asli, tanpa memakai SOL. NFT berukuran 777 byte dan memakai 16.299.040 lamport |
| Tab lama saat deploy baru, dua build berbeda | 6 dari 6 lolos, tanpa error halaman |
| Lint kontrak dan sinkron tipe API serta Postman | Lolos |
| Uji endpoint hidup, 28 dari 29 operasi | 443 dari 444 pemeriksaan lolos, dan semua jawaban sesuai kontrak. Yang gagal hanya ringkasan yang masih tersimpan 30 detik di cache. Metadata sertifikat diuji lewat `go test` dan Newman |
| Uji batas permintaan dan order kedaluwarsa | 12 dari 12 lolos |
| Newman, 29 request | 87 dari 87 assertion lolos |
| Verifikasi JWKS dengan ES256 dan RS256 | Lolos, sekarang juga di `go test` |
| ESLint, typecheck, Vitest 148 test, build produksi | Lolos |
| Playwright Edge, 17 uji | Lolos di database baru |
| Playwright WebKit, 17 uji | Lolos di database baru |
| axe di 12 layar | Semua bersih |
| Layar saat API mati, `500`, `429`, dan sesi habis | Lolos, tanpa error halaman |

### Sebelum demo

- [x] `STORE_ICON_URL` bawaan menunjuk `/icon.png`, padahal berkas itu tidak ada. Sekarang `public/icon.png` berukuran 256 px dibuat dari `icon.svg`, dan menjawab `200`
- [x] RPC Devnet publik sempat butuh 11 detik di koneksi pertama, melewati batas 8 detik di `rpc.go`. Batasnya sekarang 15 detik, dan uji baca Devnet lolos
- [x] CoinGecko tidak terjangkau dari jaringan lokal saat diuji, dan rincian pertama tiap 45 detik tertahan 3 detik. Sekarang percobaan berikutnya ditunda 5 menit, dan kurs pasar terakhir lebih diutamakan daripada `FIXED_IDR_PER_SOL`. Nilai ini tetap harus diisi di Render
- [x] Akun yang pernah login lokal terikat ke subject `local:...`, jadi email yang sama ditolak saat login Google. Sekarang login Google mengambil alih subject lokal itu, dan subject Google tetap tidak bisa diambil alih

### Bug API

- [x] API menjawab `500`, bukan `400`, untuk byte nol di teks, serta untuk `q=%00` dan `q=%FF` di `GET /items`. Validator teks sekarang menolak karakter yang ditolak Postgres
- [x] UTF-8 rusak di body JSON diterima `201` dan tersimpan sebagai karakter pengganti. Sekarang ditolak `400`
- [x] Metode HTTP yang salah menjawab `404`. Sekarang menjawab `405 METHOD_NOT_ALLOWED` dengan header `Allow`, dan kontraknya sudah diperbarui

### Frontend

- [x] Struktur `dl` di kartu dasbor toko dan penitip salah, karena `dt` dan `dd` terbungkus dua lapis `div`
- [x] Kontras `--muted-foreground` di atas `#f3f4f6` hanya 4,39:1. Nilainya sekarang `#656d7a`, dengan kontras 4,74:1
- [x] Layar kasir tidak punya `h1`
- [x] Frontend belum mengirim header keamanan. Sekarang semua halaman mengirim `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, dan `Content-Security-Policy`. CSP sengaja tanpa `script-src`, supaya wallet asli tidak terblokir
- [x] Halaman `404` masih bawaan Next.js berbahasa Inggris
- [x] Saat API tidak terjangkau ketika halaman dimuat ulang, pengguna dilempar ke `/login`. Sekarang tokennya tetap tersimpan, dan layar "Belum tersambung" memberi tombol coba lagi
- [x] `/bayar/bukan-uuid` menampilkan "Harus berupa UUID." ke pembeli. Sekarang tautan itu menuju halaman 404

### Uji dan dependency

- [x] Uji "refund menolak wallet selain wallet toko" gagal di database baru. Uji ini sekarang membuat order lunasnya sendiri
- [x] Di WebKit, 2 sampai 3 uji gagal di pemeriksaan console dengan pesan "due to access control checks". Penyebabnya fetch yang terputus karena uji pindah halaman, bukan CORS, karena API mencatat `context canceled` di waktu yang sama. Uji sekarang menunggu permintaan selesai. WebKit yang dipakai masih build 2370, sedangkan Playwright 1.65 meminta 2388
- [ ] Firefox tidak bisa dijalankan, karena Windows menolak `firefox.exe` versi tes dan Firefox biasa tidak terpasang
- [x] Koleksi Postman belum punya assertion. Tiap request sekarang memeriksa status terhadap kontrak, tidak ada `5xx`, dan ada `X-Request-Id`
- [x] `NewJWKSVerifier` belum tersentuh `go test`. Sekarang ada uji kunci ES256 dan RS256, serta token palsu
- [x] `npm audit` frontend menemukan 18 celah moderate. Sekarang 0, setelah `jayson` dan `minimatch` dinaikkan lewat `overrides`
- [ ] `npm audit` folder `api` tinggal 3 high, semuanya dari `@faker-js/faker` 5.5.3 lewat `openapi-to-postmanv2`. Versi faker baru membuat alat itu gagal, dan belum ada perbaikan. Paket ini hanya alat pengembangan, jadi tidak ikut ke produksi
- [ ] `govulncheck` mencatat GO-2026-5932 di `golang.org/x/crypto`. Kode tidak memanggilnya, dan perbaikannya baru ada di versi pseudo

## Deploy

Bagian ini butuh akun pemilik proyek.

- [ ] Proyek Supabase, Google OAuth, dan migrasi
- [ ] Layanan Render, environment termasuk `FIXED_IDR_PER_SOL`, dan cron `/healthz`
- [ ] Proyek Vercel
- [ ] Repo dibuat publik sebelum submit Devpost
- [x] Pernyataan template di README
- [ ] Pernyataan AI di README, serta pernyataan AI dan template di Devpost
