# Frontend FastConsignee

Aplikasi Next.js untuk kasir, pemilik toko, dan penitip. Tipe dan klien API dibuat dari [api/openapi.yaml](../api/openapi.yaml).

Peta layar, aturan kode, keputusan yang menunggu, dan rencana landing page ada di [docs/FRONTEND.md](../docs/FRONTEND.md).

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

## Saat kontrak berubah

```bash
npm run api:types
```

Perintah ini membuat ulang `src/lib/api/schema.d.ts`. CI gagal kalau file ini tidak sama dengan kontrak.
