# Backlog FastConsignee

Pekerjaan dicicil per fitur, dan backend dikerjakan lebih dulu. Satu item baru selesai kalau testing-nya lolos. Sumber kebutuhannya ada di [PRD](../README.md), dan kontraknya ada di [api/openapi.yaml](../api/openapi.yaml).

## Sisa pekerjaan

Semua fitur sudah dibuat. Yang tersisa adalah uji nyata, deploy, dan submit. Rincian tiap item ada di bagiannya masing-masing. Deploy didahulukan karena paling banyak membuka item lain.

| Urutan | Pekerjaan | Butuh | Item yang ikut terbuka |
| --- | --- | --- | --- |
| 1 | ~~Merge mockup HP ke `main`~~ | Selesai 8 November 2026 | Tidak ada |
| 2 | ~~Google OAuth~~ | Selesai 8 November 2026, login Google pertama berhasil | Tidak ada |
| 3 | Bukti cron. Render, Vercel, `FRONTEND_URL`, dan URL Supabase sudah jalan | Akun pemilik proyek | PageSpeed layar bayar, latensi API |
| 4 | Saldo Devnet di akun Phantom Pembeli dan Toko | SOL Devnet gratis dari [faucet](https://faucet.solana.com), maksimal 2 permintaan tiap 8 jam. Login GitHub menaikkan batasnya. Pembeli sudah berisi 1 SOL sejak 8 November 10:00 WIB dan tampil di Phantom, sedangkan Toko belum | Demo dengan Phantom |
| 5 | Uji Phantom di HP, demo 5 kali, dan Firefox | HP dan Firefox pemilik proyek | Tidak ada |
| 6 | Video demo 1 sampai 2 menit | Pemilik proyek | Tombol utama landing |
| 7 | Repo publik, pernyataan di halaman submit, lalu submit | Akun pemilik proyek | Tidak ada |

Uji pembayaran, refund, dan NFT nyata di Devnet sudah lolos memakai wallet uji. Saldo di Phantom tetap dibutuhkan untuk demo: wallet pembeli untuk membayar, wallet toko untuk refund, dan wallet khusus NFT untuk sertifikat. Login, deploy, uji di HP, video, dan submit dikerjakan terpisah.

Satu item menunggu pihak luar, yaitu GO-2026-5932 di `golang.org/x/crypto`.

### Jadwal lomba

Sumbernya [halaman event MLH](https://events.mlh.com/events/15200-solana-hackathon), dicek 7 November 2026. Lomba sudah berjalan, dan pengumpulan memakai [halaman submit MLH](https://www.mlh.com/events/solana-hackathon/submissions/new), bukan Devpost.

| Acara | ET | WIB |
| --- | --- | --- |
| Hacking mulai | Jumat 12:00 | Sabtu 7 November 00:00 |
| Initial submission | Minggu 10:00 | Minggu 8 November 22:00 |
| Hacking selesai dan final submission | Minggu 11:45 | Minggu 8 November 23:45 |

Halaman submit MLH menulis batas 13:00 EST, atau Senin 01:00 WIB, jadi berbeda dari jadwal di atas. Panduan lokal menulis 21:00 dan 22:45 WIB. Yang dipakai tetap batas paling awal, yaitu Minggu 21:00 WIB.

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
- [x] Pembayaran dan refund nyata di Devnet, 8 November 2026. `TestDevnetPaymentAndRefund` lolos, dengan [transaksi bayar](https://explorer.solana.com/tx/5D8Q88vm6te8LsSaTRjSGGKvDuTioK5uqpHDLsa2jWDCiCMM2C1G2aGoDvLQRQgCTAF2cKQbBbjZMZj68bMSvAiY?cluster=devnet) dan [transaksi refund](https://explorer.solana.com/tx/5BNLCkgXnRbcD7wEDTkLtS4WPJ3ikYFxueERcvp2VngBmSktBUBCoRE3atSjPJpgxTiu71JWxuyRddYUMXp2uBW5?cluster=devnet)
- [x] Login dengan token Supabase asli. Terbukti 8 November 2026 pukul 11.31 WIB: pemilik proyek login dengan Google di https://fast-consignee.vercel.app, Supabase mencatat satu akun Google, dan akun pemilik toko di database tertaut ke subject Google itu
- [x] Kode pengiriman NFT sertifikat, lolos uji worker dan simulasi di program Token, Associated Token, dan Token Metadata yang asli di Devnet
- [x] NFT sertifikat nyata di Devnet, 8 November 2026. `TestDevnetCertificateMints` lolos: order dibayar, lalu NFT-nya sampai di wallet pembeli dan punya akun metadata. [Lihat NFT di Explorer](https://explorer.solana.com/address/CBaJANZpLCHZeBkLpzuqv3KVyodmHvxBoVg94dgiioU8?cluster=devnet)

Testing: beban, keamanan, Solana Devnet.

## Frontend

Frontend dimulai setelah kode backend selesai. Sisa uji nyata backend menyusul saat Supabase dan wallet Devnet siap.

### F-01 Fondasi dan login

Kebutuhan: FR-25, FR-27.

- [x] Next.js dan klien API dari kontrak
- [x] Login lokal untuk pengembangan
- [x] Layar penautan wallet
- [x] Tema amber di aplikasi
- [x] Login Google lewat Supabase Auth. Terbukti 8 November 2026 pukul 11.31 WIB: pemilik proyek login dengan Google di https://fast-consignee.vercel.app, Supabase mencatat satu akun Google, dan akun pemilik toko di database tertaut ke subject Google itu

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
- [x] Mockup HP di landing menyalin layar kasir dan bayar yang asli. Sebelumnya berbentuk chat berbahasa Inggris dengan tab bawah yang tidak ada di aplikasi, dan "Waiting for payment" tampil bersama "Confirmed"
- [x] Kepala layar kasir di lebar 320 px. Nama FastConsignee menimpa label Devnet sejauh 35 px. Sekarang logonya disembunyikan di bawah 360 px, jaraknya jadi 7 px, dan uji Playwright menjaganya

Testing: Playwright di lebar 375 dan 1440 px, dengan reduced motion, dan rewrite `/` di `next start`. Halaman login juga diuji begitu, termasuk kembali ke `/`. Firefox belum diuji langsung.

Mockup HP diuji di lebar 320, 390, 768, dan 1440 px, tanpa scroll samping dan tanpa teks bertumpuk di kepala layar. `landing-phone.test.ts` gagal kalau teks layar asli berubah tetapi mockup tertinggal. Test itu hanya memeriksa teks, bukan tata letak. Uji landing yang baru dijalankan di Edge, belum di WebKit.

Catatan mockup: QR-nya hanya berisi teks, jadi tidak membuka pembayaran. Kurs dan saldonya data uji lokal. Chip kecil di bagian *three steps*, seperti "QR for Rp 180,000", tetap berbahasa Inggris karena hiasan, bukan salinan layar.

### F-07 Performa

Kebutuhan: belum ada di PRD. Angkanya diukur dari build produksi lokal pada 6 November 2026, sebelum dan sesudah perbaikan.

- [x] Kasir membaca saldo tanpa `@solana/web3.js`, jadi JavaScript-nya turun dari 278 KB ke 192 KB setelah gzip
- [x] Font Inter di landing dikirim dari server sendiri, bukan dari Google Fonts
- [x] Foto hero landing dan latar login punya versi kecil untuk HP, jadi unduhan fotonya turun dari 188 KB ke 81 KB
- [x] Font monospace tidak lagi diunduh di awal setiap halaman
- [x] Layar bayar hanya memuat `@solana/web3.js` saat pembeli menekan "Bayar dengan wallet", jadi pembeli yang memindai QR tidak mengunduhnya. Memuatnya otomatis setelah layar tampil malah menurunkan nilainya ke 81, jadi cara itu dibatalkan. Belum diuji dengan Phantom asli
- [x] Kalau berkas wallet gagal dimuat, layar bayar memberi pesan dan QR tetap bisa dipakai
- [x] Uji tab yang masih terbuka saat ada deploy baru, dengan dua build berbeda. Tab lama tetap login, tetap bisa pindah layar, dan kasir tetap bisa membuat QR. Layar yang belum di-prefetch dimuat ulang penuh oleh Next.js
- [x] Foto latar login sekarang berupa `img`, jadi browser memintanya bersamaan dengan CSS. Nilai Lighthouse tidak ikut naik, yaitu 96 dengan LCP 2,7 detik, karena unduhan foto hanya 6% dari LCP di simulasi
- [x] Foto hero landing dan login disimpan di server sendiri, dalam AVIF dengan cadangan WebP. Lima foto landing lain masih dari Unsplash, dan semuanya di bawah layar pertama
- [x] Ukur PageSpeed setelah web online, 8 November 2026. Lighthouse 12.8.2 mode HP ke https://fast-consignee.vercel.app, nilai tengah 3 kali uji: landing performa 100 (100, 99, 100) dengan LCP 1,7 detik, bobot 159 KB, dan aksesibilitas 96, lalu login 98 (98, 98, 98) dengan LCP 2,3 detik, bobot 286 KB, dan aksesibilitas 100. PageSpeed Insights resmi untuk landing di HP: performa 100, aksesibilitas 96, praktik terbaik 100, SEO 100, dan LCP 1,5 detik. PageSpeed resmi untuk login belum keluar karena analisis di situs Google macet, dan kuota API tanpa kunci sudah habis hari itu
- [ ] Ukur layar bayar dengan order asli di produksi, setelah order pertama dibuat
- [ ] Ukur latensi API setelah deploy, karena angka lokal belum memuat jarak ke Render dan Supabase. Render gratis juga tidur kalau tidak ada lalu lintas, jadi cron `/healthz` harus jalan sebelum demo. Angka awal dari jaringan lokal di Indonesia ke Render Singapore: `/healthz` dengan ping database 0,14 sampai 0,17 detik, dan `/api/v1/me` yang ditolak 0,13 sampai 0,16 detik. Endpoint yang butuh login diukur setelah login Google pertama

Kesimpulan: web ini kencang untuk HP, selama diukur di lokal. Tidak ada halaman yang berat kecuali refund dan wallet, karena keduanya memuat `@solana/web3.js`.

Lighthouse mode HP di build produksi lokal, nilai tengah 3 kali uji:

| Halaman | Performa | LCP | Bobot |
| --- | --- | --- | --- |
| Landing | 89 ke 100 | 3,2 ke 1,7 detik | 258 ke 158 KB |
| Login | 92 sampai 97 ke 96 | 2,7 sampai 3,3 ke 2,7 detik | 425 ke 287 KB |
| Bayar, dengan order asli | 91 ke 94 | 3,3 ke 2,8 detik | 390 ke 253 KB |

Nilai aksesibilitas Lighthouse: login dan bayar 100, landing 96. Sisa temuan landing berasal dari teks animasi yang belum terlihat sebelum di-scroll, dan axe bersih saat animasi dimatikan.

Waktu sampai isi layar tampil, memakai HP lambat tiruan dengan CPU 4 kali lebih lambat dan jaringan setara Slow 4G:

| Layar | Kunjungan pertama | Kunjungan ulang | Unduhan pertama | JavaScript gzip |
| --- | --- | --- | --- | --- |
| Kasir | 2,7 detik | 1,2 detik | 225 KB | 192 KB |
| Dasbor | 2,2 detik | 0,6 detik | 213 KB | 187 KB |
| Barang | 2,7 detik | 1,2 detik | 230 KB | 184 KB |
| Refund | 2,9 detik | 0,7 detik | 364 KB | 324 KB |
| Bayar | 2,2 detik | 0,7 detik | 213 KB | 153 KB |

Wallet memuat 319 KB, penitip 185 KB, dan login 177 KB JavaScript setelah gzip. Tanpa pembatas, semua layar tampil dalam 0,2 detik. JavaScript bayar dihitung dari unduhan browser sebelum tombol wallet ditekan, sedangkan layar lain dihitung dari berkas hasil build.

API lokal menjawab endpoint baca dengan p95 di bawah 7 ms, dari 200 panggilan per endpoint. Checkout menangani 103 order per detik dengan p95 233 ms. Angka ini bukan pengganti PageSpeed dan latensi setelah deploy.

## Kendala dari pengujian

Pengujian menyeluruh dijalankan di lokal pada 6 November 2026, memakai database uji terpisah dan Solana tiruan. Tabel ini memuat hasil setelah perbaikan.

| Uji | Hasil |
| --- | --- |
| `gofmt`, `go vet`, `go build`, `go test` | Lolos, cakupan 84,7%, diulang 8 November 2026. Di checkout Windows `gofmt -l` menyebut 59 file hanya karena akhir baris CRLF dari Git, dan isinya rapi setelah CRLF dibuang |
| `go test -race` | Lolos di image Docker Go, karena Windows ini tidak punya gcc |
| Fuzz 3 fungsi, masing-masing 10 detik | Lolos |
| Uji beban 1.000 order dengan 20 pekerja | Lolos, p95 284 ms saat uji lain ikut berjalan. Uji sebelumnya tanpa beban lain: 103 order per detik, p95 233 ms. Diulang 8 November 2026: p95 246 ms |
| `gosec` dan `govulncheck` | Lolos, tanpa celah yang terpanggil kode |
| Uji baca Devnet | Lolos |
| Simulasi transaksi bayar dan NFT di Devnet | Lolos di program asli, tanpa memakai SOL. NFT berukuran 777 byte dan memakai 16.299.040 lamport |
| Tab lama saat deploy baru, dua build berbeda | 6 dari 6 lolos, tanpa error halaman |
| Lint kontrak dan sinkron tipe API serta Postman | Lolos |
| Uji endpoint hidup, 28 dari 29 operasi | 443 dari 444 pemeriksaan lolos, dan semua jawaban sesuai kontrak. Yang gagal hanya ringkasan yang masih tersimpan 30 detik di cache. Metadata sertifikat diuji lewat `go test` dan Newman |
| Uji batas permintaan dan order kedaluwarsa | 12 dari 12 lolos |
| Newman, 29 request | 87 dari 87 assertion lolos |
| Verifikasi JWKS dengan ES256 dan RS256 | Lolos, sekarang juga di `go test` |
| ESLint, typecheck, Vitest 160 test, build produksi, `npm audit` | Lolos, 8 November 2026, audit 0 celah |
| Playwright Edge, 26 uji | Lolos di database baru, 8 November 2026 |
| Playwright WebKit, 26 uji | 25 lolos di database baru, 8 November 2026. Uji pergeseran tata letak dilewati karena WebKit belum punya Layout Instability API |
| Playwright landing di Edge dan WebKit, 14 uji | Lolos, termasuk uji sticky tanpa induk pemotong, kartu Every rupiah, label gelombang, dan scrollbar login |
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
- [x] Landing page terasa bergetar saat discroll di HP, 8 November 2026. Lighthouse produksi mencatat 48 animasi yang tidak bisa dijalankan GPU, termasuk efek ikut scroll yang menggerakkan elemen lewat custom property (`--listen-p`, `--rooms-p`), `clip-path`, `filter`, dan `stroke-dashoffset`. Di HP layar sentuh, gerakan itu tertinggal dari jari lalu menyusul. Sekarang efek ikut scroll hanya menyala di perangkat dengan mouse atau touchpad (`hover: hover` dan `pointer: fine`), sedangkan HP dan tablet mendapat versi diam yang sudah ada untuk *reduced motion*. Diuji di HP tiruan 412, 390, 360, dan 320 px: 0 animasi ikut scroll, 0 pergeseran tata letak dan 0 lompatan posisi selama scroll penuh, serta halaman tidak bisa digeser ke samping. Setelah `main` dimajukan, Vercel memuat versi ini dalam satu menit, dan uji yang sama ke https://fast-consignee.vercel.app memberi hasil yang sama di keempat ukuran. Cara ini diganti di item berikutnya, karena HP jadi kehilangan animasinya
- [x] Animasi landing page di HP menyala lagi, 8 November 2026. Getarnya diperbaiki di sumbernya: efek ikut scroll yang menggeser elemen sekarang hanya memakai translate, scale, atau opacity, dengan satu animasi per properti di tiap elemen, jadi GPU menjalankannya seiring jari. Yang diubah: rel foto, foto, dan keterangan di bagian *rooms* (sebelumnya digerakkan custom property `--rooms-p` di main thread tiap frame), gelombang suara di bagian *listen* (`--listen-p`, sekarang keyframes per batang tiap 0,025), 25 kata *read-along* (warna dari `--motion-ink`, sekarang opacity), kartu yang terbang dari HP ke bagian *tune* (dua animasi `scale` digabung dan `visibility` dibuang), serta lima langkah di bagian kasir (tiga animasi opacity per langkah digabung jadi satu). Di desktop, posisi dan opacity tiap elemen sama dengan versi lama di 240 titik scroll: rel foto dan langkah kasir selisih 0 px, dan gelombang paling banyak 1 px. Di HP tiruan 412, 390, 360, dan 320 px, animasi menyala dengan 0 pergeseran tata letak, 0 lompatan scroll, dan halaman tidak bisa digeser ke samping. Trace Chromium dari halaman dibuka sampai scroll penuh tidak menemukan animasi yang bertumpuk atau timing yang ditolak GPU. Sisanya yang masih di main thread adalah `clip-path`, `stroke-dashoffset`, `font-variation-settings`, warna, dan chip hero yang bergerak dengan waktu, bukan dengan scroll. Uji Playwright baru memeriksa ini di layar sentuh dan desktop. Rasa di HP asli belum dicek
- [x] Di HP, mockup di bagian *One screen at the counter* menutupi teks langkah berikutnya, yang mengintip di kiri dan kanan mockup, dan bagian bawahnya terpotong layar. Sekarang ada pita gelap selebar layar di belakang mockup, dan tinggi mockup ikut tinggi layar (`min(0.66rem, 1.3vh)` per em), jadi di 360 x 640 px mockup setinggi 268 px dan muat utuh. Di desktop yang dipersempit, animasi langkah tetap berjalan di atas pita itu
- [x] Ikon web masih huruf F di kotak kuning. Sekarang memakai logo gelombang FastConsignee di kotak gelap, di tiga tempat: favicon aplikasi (`src/app/icon.svg`), favicon landing page, dan `public/icon.png` 256 px yang dipakai wallet sebagai ikon toko di Solana Pay. Dicek di ukuran 16, 32, dan 64 px di latar terang dan gelap, dan uji landing di Edge lolos 10 dari 10
- [x] Layar aplikasi meloncat sekali saat data selesai dimuat, karena teks "Memuat..." diganti isi yang lebih tinggi. Diukur di 390 x 780 px: dasbor penitip 0,60, kasir 0,29, dasbor toko 0,27, refund 0,08, dan barang 0,08. Sekarang selama memuat, tiap layar menampilkan kerangka isi dengan susunan dan tinggi yang sama dengan isi aslinya: enam kartu angka dan kartu 7 hari di kedua dasbor (188, 209, dan 450 px, sama persis dengan kartu asli), enam kotak di katalog kasir, dan baris kosong di daftar barang, penitip, order, dan riwayat refund. Pembaca layar tetap mendengar "Memuat...". Daftar order di layar refund sekarang setinggi tiga setengah baris dengan scroll sendiri, jadi rincian dan riwayat di bawahnya tidak bergeser berapa pun jumlah ordernya. Hasilnya 0 di dasbor penitip, kasir, dasbor toko, dan barang, 0,0005 di penitip, dan 0,0014 di refund. Uji Playwright baru menjaganya di Chromium dengan batas 0,05, dan gagal dengan angka 0,29 kalau kerangkanya dilepas. WebKit melewati uji ini karena belum punya Layout Instability API
- [x] Landing masih bergetar di iPhone pemilik proyek, 8 November 2026, terutama di bagian *The buyer pays once* dan foto-foto *One shop, many consignors*. Penyebabnya bukan animasinya: tiga panggung sticky (`listen`, `calls`, `rooms`) berada di dalam section ber-`overflow-x: clip`, dan Safari iPhone menggetarkan elemen sticky di dalam induk yang memotong overflow saat toolbar mengecil atau membesar. Sampai lebar 88rem, saat wrap selebar layar, clip pindah ke panggung sticky itu sendiri dan ke kepala `listen`, jadi tepi potongnya tetap tepi layar. Di atas 88rem tidak ada toolbar yang berubah, jadi tampilannya tetap. Tumpukan animasi juga diperiksa dengan aturan WebKit untuk animasi ber-thread, dan tidak ada yang menahan GPU. Diuji di WebKit 27 ukuran iPhone dan Edge: tidak ada elemen sticky di dalam induk yang memotong, dan tanpa scroll samping di 320 sampai 1920 px, termasuk dengan scrollbar klasik. Rasa di HP asli belum bisa diuji dari sini
- [x] Pita gelap di belakang mockup HP memakai `width: 100vw`. Setelah clip `calls` dipindah, pita itu meluber 8 px ke samping di browser desktop sempit yang punya scrollbar klasik, karena `100vw` ikut menghitung scrollbar. Sekarang pita menjangkau tepi lewat gutter wrap, dan luberannya 0 dengan atau tanpa scrollbar
- [x] Kartu *Every rupiah, accounted for* langsung hilang saat di-scroll balik ke atas. Kartu itu mengecil ke nol dalam 1,4vh terakhir sebelum kembali ke layar mockup HP, sekitar 12 px scroll. Sekarang kartu mulai dari ukuran layar HP dan memudar sepanjang 21vh, jadi saat di-scroll balik ia mengecil ke HP sambil memudar perlahan. Uji Playwright memeriksa opacity di tiga titik, saat turun dan saat naik
- [x] Di HP, gelombang suara menimpa tulisan *The buyer pays once*, karena tulisannya lebih panjang dari label template. Sekarang tulisan itu turun ke dua baris sebelum mencapai gelombang, dengan celah 8 px di lebar 320 sampai 767 px
- [x] Reveal judul landing memakai easing `linear()`. Safari menjalankan animasi waktu ber-`linear()` di thread utama, jadi reveal yang terpicu saat scroll bisa tersendat di iPhone. Sekarang memakai `cubic-bezier(0.16, 1, 0.3, 1)` dengan kurva hampir sama, dan uji Playwright menolak `linear()` atau `steps()` di animasi waktu
- [x] Halaman login memunculkan scrollbar sebentar lalu hilang saat dibuka, sehingga isinya bergeser. Foto latar yang membesar 1,08 kali saat pembukaan melewati tepi bawah sampai 40 px selama sekitar 1 detik. Sekarang `.login` memotong kedua sumbu, dan uji Playwright mengambil sampel tiap frame selama 1,5 detik di 1920 x 990 dan 1366 x 768 px
- [x] Judul penutup *Your next sale can split itself.* menganimasikan custom property `--start-bar` di tiap kata. Custom property yang diwariskan membuat semua kata dihitung ulang di thread utama setiap frame scroll. Uji GPU tidak menangkapnya karena judul baru dipecah per kata saat mendekati layar. Sekarang kata menganimasikan `color` dan garis putus-putusnya `opacity` serta `scale`, dengan nilai yang sama persis dengan versi lama di enam posisi scroll, di Edge dan WebKit. Uji GPU kini mendatangi semua judul dulu, dan juga gagal di kode lama

### Uji dan dependency

- [x] Uji "refund menolak wallet selain wallet toko" gagal di database baru. Uji ini sekarang membuat order lunasnya sendiri
- [x] Di WebKit, 2 sampai 3 uji gagal di pemeriksaan console dengan pesan "due to access control checks". Penyebabnya fetch yang terputus karena uji pindah halaman, bukan CORS, karena API mencatat `context canceled` di waktu yang sama. Uji sekarang menunggu permintaan selesai. WebKit yang dipakai masih build 2370, sedangkan Playwright 1.65 meminta 2388
- [ ] Firefox tidak bisa dijalankan, karena Windows menolak `firefox.exe` versi tes dan Firefox biasa tidak terpasang
- [x] Koleksi Postman belum punya assertion. Tiap request sekarang memeriksa status terhadap kontrak, tidak ada `5xx`, dan ada `X-Request-Id`
- [x] `NewJWKSVerifier` belum tersentuh `go test`. Sekarang ada uji kunci ES256 dan RS256, serta token palsu
- [x] `npm audit` frontend menemukan 18 celah moderate. Sekarang 0, setelah `jayson` dan `minimatch` dinaikkan lewat `overrides`
- [x] `npm audit` folder `api` menemukan 4 high dan 2 moderate. Sekarang 0, karena `openapi-to-postmanv2` diganti generator sendiri di `api/scripts/postman.mjs`. Isi koleksinya sama, kecuali petunjuk pilihan nilai yang sekarang berbahasa Indonesia, teks kontak di deskripsi yang hilang, dan variabel baru `certificate_id`
- [x] Playwright penuh diulang setelah mockup HP diganti dan kepala layar 320 px diperbaiki. Edge dan WebKit sama-sama lolos 19 dari 19
- [x] Stub Solana untuk uji mati kalau menerima request kosong, misalnya dari pemeriksa kesiapan server, sehingga semua langkah pembayaran gagal `503`. Sekarang request kosong dijawab `400` dengan error JSON-RPC, dan stub tetap hidup
- [x] Playwright penuh diulang setelah landing page diam di layar sentuh, 8 November 2026. Edge dan WebKit sama-sama lolos 21 dari 21, termasuk uji layar sentuh dan uji mockup HP yang baru
- [x] Playwright penuh diulang setelah kerangka isi dan animasi landing di HP, 8 November 2026. Edge lolos 22 dari 22, sedangkan WebKit lolos 21 dan melewati 1 uji pergeseran tata letak. Vitest 156 dari 156, tsc, dan ESLint juga bersih
- [x] Semua cara uji dijalankan ulang setelah perbaikan getar iPhone, 8 November 2026. Backend: `gofmt`, `go vet`, `go test` dengan cakupan 84,7%, fuzz 3 fungsi, uji beban (p95 246 ms), `gosec`, `govulncheck`, `go test -race` di Docker, dan lint kontrak, semuanya lolos. Frontend: sinkron tipe API, ESLint, typecheck, Vitest 160 uji, build produksi, dan `npm audit` 0 celah. Playwright penuh di database baru: Edge lolos 25 dari 25, WebKit lolos 25 dan melewati 1 uji pergeseran tata letak. Setelah uji login dan uji GPU ditambah, 14 uji landing lolos di Edge dan WebKit. Lima uji baru dijalankan juga di kode lama dan semuanya gagal, jadi uji itu benar menangkap bugnya
- [ ] `govulncheck` mencatat GO-2026-5932 di `golang.org/x/crypto`. Kode tidak memanggilnya, dan perbaikannya baru ada di versi pseudo

## Deploy

Bagian ini butuh akun pemilik proyek.

- [x] Merge mockup HP ke `main`, 8 November 2026. `main` dimajukan ke `frontend` tanpa commit merge, jadi isinya sama persis. Sebelumnya Vitest 156 uji lolos dan tsc bersih. Hari yang sama `main` dimajukan lagi supaya Render ikut membawa migrasi `00002`. Pukul 12.48 WIB `main` dimajukan ke `4237ce5`, membawa skeleton layar aplikasi, animasi landing di HP, dan kode login Google langsung. Vercel menyajikan versi baru sekitar 20 detik kemudian. Dicek di produksi dengan empat HP tiruan: animasi menyala dengan 120 animasi ikut scroll, tanpa pergeseran layout, tanpa loncatan, dan tanpa scroll samping. Playwright landing lolos 9 dari 10 uji di produksi. Satu uji gagal karena mencari tombol akun demo, yang memang disembunyikan di produksi
- [x] Proyek Supabase di Singapore dengan paket gratis, 8 November 2026. Migrasi `00001` dan `00002` sudah jalan, dan seed membuat 1 toko, 5 akun, 3 penitip, dan 9 barang. Pemilik toko dan penitip Sari Anyaman memakai email Google pemilik proyek, sedangkan penitip lain dan kasir memakai email `.test`. Seed kedua tidak membuat data baru. Proyek tidak disambungkan ke GitHub karena migrasi dijalankan goose dari API Go, dan Data API dianjurkan tetap mati
- [x] Akses tabel lewat Data API Supabase ditutup, 8 November 2026. Bawaan Supabase memberi peran anon dan authenticated hak penuh atas 13 tabel di schema `public`, termasuk menulis dan menghapus, padahal anon key ikut terkirim ke browser. Migrasi `00002` mencabut hak itu, juga untuk tabel baru, lalu menyalakan RLS tanpa policy. Diuji di Supabase: hak anon dan authenticated tinggal 0 dari 13 tabel, RLS menyala di 13 tabel, `SET ROLE anon` lalu membaca `users` ditolak "permission denied", dan pemilik tabel tetap bisa menulis dan membaca. `go test` penuh lolos lokal dengan uji RLS baru
- [x] Google OAuth di Supabase dan Google Cloud. Provider Google sudah menyala, dan pada 8 November 2026 Google menerima Client ID serta Callback URL-nya. Client Secret terbukti saat login pertama. Terbukti 8 November 2026 pukul 11.31 WIB: pemilik proyek login dengan Google di https://fast-consignee.vercel.app, Supabase mencatat satu akun Google, dan akun pemilik toko di database tertaut ke subject Google itu. Aplikasi Google tetap di mode Testing dengan dua email pemilik proyek sebagai test user
- [x] Layanan Render `fastconsignee-api` di Singapore dengan paket gratis, Live di https://fastconsignee-api.onrender.com pada 8 November 2026. JWKS proyek Supabase memakai kunci ES256, jadi Render cukup diberi `SUPABASE_JWKS_URL` tanpa `SUPABASE_JWT_SECRET`. Diuji dari luar: `/healthz` menjawab `200` dengan ping database, `/api/v1/me` tanpa token dan dengan token palsu menjawab `401` beserta header rate limit, order publik acak menjawab `404` dari tabel `orders`, preflight CORS dari `https://fastconsignee.vercel.app` diterima, dan origin lain tidak mendapat header izin
- [ ] Cron `/healthz` tiap 10 menit di cron-job.org, supaya Render gratis tidak tidur. Cronjob `fastconsignee-cron` sudah dibuat pada 8 November 2026, dengan notifikasi setelah 2 kegagalan berturut-turut. Dicentang setelah terbukti: Render tetap menjawab cepat walaupun 20 menit lebih tidak ada permintaan selain dari cron. Pukul 14.06 WIB `/healthz` butuh 11,3 detik lalu 0,16 detik, tanda Render sempat tidur, jadi cron belum terbukti menjaganya. Pukul 14.28 WIB, setelah 22 menit tanpa permintaan dari pengujian, `/healthz` menjawab 0,25 detik, jadi Render tetap bangun. Belum dicentang karena buktinya campuran: riwayat eksekusi di cron-job.org perlu dicek pemilik proyek untuk melihat kenapa Render sempat tidur sebelum 14.06
- [x] Proyek Vercel Live di https://fast-consignee.vercel.app pada 8 November 2026, bukan `fastconsignee.vercel.app` seperti rencana awal. Diuji dari luar: `/`, `/landing.html`, `/login`, `/kasir`, `/bayar/...`, dan `/icon.png` menjawab `200`. JS hasil build memuat alamat API Render, Project URL Supabase, dan publishable key, tanpa `localhost:8080`, tanpa `sb_secret_`, dan tanpa `service_role`. Halaman login menampilkan tombol "Masuk dengan Google", jadi login lokal tidak ikut terpasang
- [x] `FRONTEND_URL` di Render diganti ke `https://fast-consignee.vercel.app`, 8 November 2026. Sebelum diganti, preflight CORS dari alamat itu tidak mendapat header izin. Sesudahnya alamat itu diterima, alamat lama `fastconsignee.vercel.app` tidak lagi diberi izin, dan `/healthz` tetap `200`
- [x] Site URL dan Redirect URLs Supabase diisi `https://fast-consignee.vercel.app` dan `https://fast-consignee.vercel.app/login`. Sudah diisi pemilik proyek pada 8 November 2026, tetapi belum bisa dicek dari luar karena `state` OAuth Supabase sekarang berupa ID acak. Buktinya login Google pertama kembali ke `/login` di Vercel. Terbukti 8 November 2026 pukul 11.31 WIB: pemilik proyek login dengan Google di https://fast-consignee.vercel.app, Supabase mencatat satu akun Google, dan akun pemilik toko di database tertaut ke subject Google itu
- [ ] Layar izin Google menyebut `gmbuhjjdepqxquwzqeej.supabase.co`, bukan FastConsignee. Kodenya siap 8 November 2026: kalau `NEXT_PUBLIC_GOOGLE_CLIENT_ID` diisi di Vercel, Google kembali langsung ke `https://fast-consignee.vercel.app/login` dan frontend menukar ID token-nya dengan sesi Supabase, jadi yang tampil alamat situs ini. Tanpa variabel itu, login tetap lewat Supabase seperti sebelumnya. Lolos Vitest 160 uji, termasuk uji nonce, state, dan alur cadangan, serta Playwright login. Kodenya sudah di produksi sejak `main` dimajukan ke `4237ce5`, tetapi JS di Vercel belum memuat Client ID, jadi login masih lewat Supabase. Butuh pemilik proyek: tambah redirect URI `https://fast-consignee.vercel.app/login` di client Google, isi `NEXT_PUBLIC_GOOGLE_CLIENT_ID` di Vercel untuk Production, lalu Redeploy di Vercel. Tulisan "FastConsignee" sendiri baru tampil setelah merek diverifikasi Google, yang butuh halaman kebijakan privasi dan verifikasi domain di Search Console. Domain khusus Supabase tidak dipakai karena berbayar
- [x] Wallet uji Devnet diisi 1 SOL dari faucet. Dua uji nyata memakai sekitar 0,16 SOL, jadi sisanya 0,84 SOL. Kuncinya ada di luar repo
- [ ] `CERTIFICATE_SIGNER_SECRET` di Render diisi kunci wallet khusus NFT yang punya saldo. Kalau kosong, sertifikat tidak dikirim
- [ ] Video demo 1 sampai 2 menit. Syarat ini baru tertulis di panduan lokal, dan halaman resmi MLH yang terbuka tanpa login tidak menyebutnya. Kepastiannya ada di form submit, yang butuh login
- [x] Check-in lomba di halaman MLH. Register dan Check In sudah tercentang pada 7 November 2026, dan tahap berikutnya Submit Project
- [x] Riwayat git dipindai sebelum repo dibuka, 8 November 2026. Isinya hanya nilai test, contoh di `.env.example`, dan hash lockfile, tanpa kunci atau password asli
- [ ] Repo dibuat publik sebelum submit
- [x] Pernyataan template di README
- [x] Draf pernyataan AI di README. Pembuat proyek perlu membacanya sebelum submit
- [ ] Pernyataan AI dan template di halaman submit MLH
- [ ] Initial submission di halaman MLH dengan tautan repo dan video demo
- [ ] Final submission di halaman MLH
