# Frontend FastConsignee

Aplikasi Next.js untuk kasir, pemilik toko, dan penitip. Tipe dan klien API dibuat dari [api/openapi.yaml](../api/openapi.yaml).

Peta layar, aturan kode, keputusan, dan catatan landing page ada di [docs/FRONTEND.md](../docs/FRONTEND.md).

## Menjalankan di komputer

Selama proyek Supabase belum ada, login memakai token lokal. Buat satu secret acak, lalu pakai nilai yang sama di kedua file.

Di `backend/.env`, hapus `SUPABASE_JWKS_URL` dan isi:

```
SUPABASE_JWT_SECRET=secret-acak-minimal-32-karakter
SUPABASE_JWT_ISSUER=fastconsignee-local
```

Salin [.env.example](.env.example) menjadi `.env.local`, lalu isi `LOCAL_JWT_SECRET` dengan secret yang sama.

Isi data demo dan jalankan API dari folder `backend`:

```bash
SEED_OWNER_EMAIL=pemilik@fastconsignee.test go run ./cmd/seed
```

```bash
go run ./cmd/api
```

Jalankan frontend:

```bash
npm install
```

```bash
npm run dev
```

Buka `http://localhost:3000`, lalu pilih akun. Kalau API tidak berjalan di port 8080, samakan `PORT` dan `PUBLIC_API_URL` di backend dengan `NEXT_PUBLIC_API_URL` di frontend.

Login lokal hanya ada di `next dev`. Di produksi route-nya menjawab 404, dan API yang memakai JWKS tetap menolak token lokal.

## Testing

```bash
npm test
```

```bash
npm run lint
```

```bash
npm run typecheck
```

## Uji browser

Playwright menjalankan alur demo di Edge terhadap API Go dan Postgres lokal. Solana dan wallet-nya tiruan, jadi uji ini tidak butuh SOL dan tidak berjalan di CI. Node tiruannya memeriksa tanda tangan dan mengubah saldo, tetapi tidak menggantikan uji di Devnet.

Jalankan API dari folder `backend` dengan RPC tiruan:

```bash
SOLANA_RPC_URL=http://127.0.0.1:8899 go run ./cmd/api
```

Jalankan `npm run dev`, lalu dari folder `frontend`:

```bash
npm run e2e
```

Node tiruan dinyalakan otomatis di port 8899. Alamat lain diatur lewat `E2E_BASE_URL`, dan browser lain lewat `E2E_BROWSER_CHANNEL`.

Uji ini menulis order, refund, dan penautan wallet ke database lokal. Jangan jalankan terhadap database yang datanya ingin dipertahankan.

Tiap putaran mengurangi stok seed. Stok `Syal Batik` habis setelah sekitar 15 putaran, jadi buat ulang database dan jalankan seed lagi sebelum itu.

## Landing page

Landing page ada di `public/landing.html`, dan `next.config.ts` menyajikannya di `/`. File ini memakai [template Linea dari 8B](https://8b.com/templates/linea/) dengan lisensi MIT. Lisensi dan kredit fotonya ada di `public/landing/`.

Halaman `/login` memakai gaya dari template yang sama. Gayanya ada di `src/features/auth/login.css`.

## Saat kontrak berubah

```bash
npm run api:types
```

Perintah ini membuat ulang `src/lib/api/schema.d.ts`. CI gagal kalau file ini tidak sama dengan kontrak.
