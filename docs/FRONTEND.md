# Frontend FastConsignee

Dokumen kerja frontend. Pilot memakainya untuk memutuskan arah, dan co-pilot memakainya untuk mengerjakan layar dengan aturan yang sama.

| Butuh apa | Lihat di mana |
| --- | --- |
| Kebutuhan produk | [PRD](../README.md) |
| Status tiap item | [BACKLOG.md](BACKLOG.md) |
| Bentuk API | [api/openapi.yaml](../api/openapi.yaml) |
| Cara menjalankan | [frontend/README.md](../frontend/README.md) |

## Cara kerja

- Pilot memutuskan urutan kerja, desain, dan kapan commit atau push.
- Co-pilot boleh mengusulkan apa saja, tetapi baru mengerjakan setelah pilot bilang jalan.
- Satu item selesai kalau testing-nya lolos. Setelah itu backlog langsung diperbarui.
- Commit hanya atas nama pilot, tanpa baris atribusi AI.
- Penggunaan AI dan template pihak ketiga ditulis di README dan Devpost, sesuai aturan MLH.
- Biaya harus nol. Kalau sebuah pilihan butuh biaya atau akun baru, pilot yang memutuskan.

## Keputusan yang menunggu pilot

| No | Keputusan | Usulan co-pilot | Alasan |
| --- | --- | --- | --- |
| 1 | Landing page atau F-02 Kasir dulu? | F-02 Kasir dulu | Video demo bergantung pada alur kasir, dan landing page belum ada di PRD. |
| 2 | Bahasa landing page | Inggris, aplikasinya tetap Indonesia | Juri MLH membaca Inggris, dan PRD memosisikan produk untuk pasar global. |
| 3 | Tujuan tombol utama | "Tonton demo", ditambah "Masuk" untuk staf | Belum ada pendaftaran mandiri. Akun Google yang belum terdaftar akan menerima `403 ROLE_NOT_ASSIGNED`. |
| 4 | Pengganti bagian testimoni | Kasus nyata dari PRD, tanpa nama toko, dengan tautan sumber | Belum ada pengguna, jadi ulasan tidak boleh dikarang. Sebagian berita berisi tuduhan terhadap toko yang bisa dikenali. |
| 5 | Bagian kontak | Hapus bagian dan formulirnya, cukup tautan GitHub di footer | Formulirnya tidak mengirim apa pun, dan layanan formulir menambah satu akun lagi. |
| 7 | Foto | Foto toko sungguhan kalau ada, kalau tidak dari Unsplash | Foto template bertema telepon dan klinik. |
| 8 | Langkah demo 3, saldo semua pihak naik bersamaan | Frontend membaca saldo Devnet dari alamat `recipients` | API tidak punya endpoint saldo. Pilihan lainnya mengubah langkah itu menjadi tampilan Phantom atau Explorer. |
| 9 | Kapan F-01 di-commit | Sekarang, di branch `frontend` | Semua kode F-01 masih berupa perubahan lokal. |
| 10 | Mode gelap | Token `.dark` disimpan, tombol ganti tema ditunda | PRD tidak memintanya, dan tiap layar harus diuji dua kali kalau tombolnya ada. |
| 11 | Nilai tema yang kontrasnya rendah | Terima usulan di bagian [Penyesuaian kontras](#penyesuaian-kontras) | Teks error dan fokus keyboard sulit terbaca dengan nilai aslinya. |
| 12 | Warna untuk status berhasil | Tambah token hijau `--success`, khusus untuk status berhasil | Tema tidak punya warna sukses, jadi "Wallet tertaut" kini terlihat seperti peringatan. Status order di F-02 butuh warna yang jelas berbeda. |

Keputusan 6 sudah diambil: tema amber dipasang sebelum F-02, dan sekarang sudah terpasang di aplikasi.

## Status sekarang

F-01 selesai kecuali login Google. Sudah ada Next.js, klien API dari kontrak, login lokal, penjaga peran, layar wallet, tema amber, dan job `frontend` di CI.

Hasil uji terakhir:

- 39 test Vitest, lint, typecheck, dan build lolos.
- Uji browser 17 dari 17 lolos, termasuk tanda tangan ed25519 asli yang diterima API Go.
- Layar login dan wallet muat di lebar 375 px.

Yang belum:

- Uji dengan Phantom sungguhan, dijadwalkan di F-05.
- Login Google, menunggu proyek Supabase.
- `/kasir` dan `/dasbor` masih placeholder.

## Peta layar

Nama route yang bertanda usulan belum dibuat dan masih bisa diganti pilot.

| Route | Pengguna | Isi | Endpoint | Item |
| --- | --- | --- | --- | --- |
| `/` | Publik | Landing page. Saat ini hanya pengalihan. | Tidak ada | F-06 |
| `/login` | Publik | Login lokal, nanti Google | `GET /me` | F-01, selesai |
| `/wallet` | Pemilik toko, penitip | Penautan wallet | `POST /wallet/nonce`, `POST /wallet/link` | F-01, selesai |
| `/kasir` | Kasir, pemilik toko | Katalog, keranjang, rincian pembagian, QR, status | `GET /items`, `POST /orders/preview`, `POST /orders`, `GET /orders/{id}/status` | F-02 |
| `/bayar/[orderId]` (usulan) | Publik | Layar bayar, tombol wallet, bukti pembayaran | `GET /public/orders/{id}`, `POST /solana-pay/orders/{id}` | F-03 |
| `/dasbor` | Pemilik toko | Ringkasan toko | `GET /dashboard/store/summary` | F-04 |
| `/dasbor` | Penitip | Penjualan, potongan, stok | `GET /dashboard/consignor/sales`, `/deductions`, `/stock` | F-04 |
| `/refund` (usulan) | Pemilik toko | Daftar order dan refund satu barang | `GET /orders`, `POST /orders/{id}/refunds`, `GET /refunds`, `POST /refunds/{id}/signature` | F-04 |
| `/barang` (usulan) | Pemilik toko | Kelola barang | `/items` | F-04 |
| `/penitip` (usulan) | Pemilik toko | Kelola penitip | `/consignors` | F-04 |

Catatan untuk F-02 dan F-03:

- Status order memakai polling `GET /orders/{id}/status`, karena kontrak tidak punya push. Polling tiap 2 detik sama dengan jeda worker dan masih di bawah batas 120 permintaan per menit per akun.
- QR butuh satu pustaka baru, yang dipilih saat F-02 dimulai.
- Wallet di HP tidak bisa menghubungi `localhost`, jadi QR sungguhan baru bisa diuji setelah API punya alamat HTTPS publik.
- `/bayar` berada di luar grup `(app)`, karena pembeli tidak login.

## Struktur kode

```
frontend/src/
  app/         route, dibuat tipis
  features/    satu folder per fitur: auth, wallet, lalu kasir, bayar, dasbor
  components/  komponen bersama
  lib/api/     klien dan tipe dari kontrak
```

Aturannya:

- Logika layar ada di `features`, dan route hanya merangkainya.
- Semua panggilan API lewat `useSession().api`. Tipe berasal dari `schema.d.ts`, yang dibuat ulang dengan `npm run api:types` dan tidak diedit tangan.
- Pesan error dipilih dari `ApiError.code`, bukan dari teks jawaban API.
- Frontend tidak menghitung pembagian sendiri. Angkanya selalu dari `POST /orders/preview`.
- `POST /orders` wajib mengirim `Idempotency-Key`, dan nilainya tetap sama saat tombol ditekan ulang.
- `@solana/web3.js` tetap di versi 1.x, karena wallet adapter membutuhkannya.
- Secret hanya ada di `.env.local`, yang tidak masuk ke repositori.
- Teks aplikasi berbahasa Indonesia, dan tiap layar harus muat di lebar 375 px.

## Desain aplikasi

Aplikasi memakai [tema amber](#tema-amber). Warnanya ada di [globals.css](../frontend/src/app/globals.css), dan komponen dasarnya di [ui.tsx](../frontend/src/components/ui.tsx). Font-nya Inter untuk teks, serta JetBrains Mono untuk alamat wallet dan angka teknis.

## Tema amber

Pilot memilih tema ini untuk aplikasi dan landing page. Aplikasi sudah memakainya, dan landing page menyusul saat F-06.

### Nilai tema

Nilai di bawah sama persis dengan yang diberikan pilot, dan yang terpasang di [globals.css](../frontend/src/app/globals.css) juga sama, kecuali tiga baris font. Penyesuaiannya dicatat terpisah di [Penyesuaian kontras](#penyesuaian-kontras).

```css
:root {
  --card: #ffffff;
  --ring: #f59e0b;
  --input: #e5e7eb;
  --muted: #f9fafb;
  --accent: #fffbeb;
  --border: #e5e7eb;
  --radius: 0.375rem;
  --chart-1: #f59e0b;
  --chart-2: #d97706;
  --chart-3: #b45309;
  --chart-4: #92400e;
  --chart-5: #78350f;
  --popover: #ffffff;
  --primary: #f59e0b;
  --sidebar: #f9fafb;
  --spacing: 0.25rem;
  --font-mono: JetBrains Mono, monospace;
  --font-sans: Inter, sans-serif;
  --secondary: #f3f4f6;
  --background: #ffffff;
  --font-serif: Source Serif 4, serif;
  --foreground: #262626;
  --destructive: #ef4444;
  --shadow-blur: 8px;
  --shadow-color: hsl(0 0% 0%);
  --sidebar-ring: #f59e0b;
  --shadow-spread: -1px;
  --letter-spacing: 0em;
  --shadow-opacity: 0.1;
  --sidebar-accent: #fffbeb;
  --sidebar-border: #e5e7eb;
  --card-foreground: #262626;
  --shadow-offset-x: 0px;
  --shadow-offset-y: 4px;
  --sidebar-primary: #f59e0b;
  --muted-foreground: #6b7280;
  --accent-foreground: #92400e;
  --popover-foreground: #262626;
  --primary-foreground: #000000;
  --sidebar-foreground: #262626;
  --secondary-foreground: #4b5563;
  --destructive-foreground: #ffffff;
  --sidebar-accent-foreground: #92400e;
  --sidebar-primary-foreground: #ffffff;
}

.dark {
  --card: #262626;
  --ring: #f59e0b;
  --input: #404040;
  --muted: #1f1f1f;
  --accent: #92400e;
  --border: #404040;
  --chart-1: #fbbf24;
  --chart-2: #d97706;
  --chart-3: #92400e;
  --chart-4: #b45309;
  --chart-5: #92400e;
  --popover: #262626;
  --primary: #f59e0b;
  --sidebar: #0f0f0f;
  --secondary: #262626;
  --background: #171717;
  --foreground: #e5e5e5;
  --destructive: #ef4444;
  --sidebar-ring: #f59e0b;
  --sidebar-accent: #92400e;
  --sidebar-border: #404040;
  --card-foreground: #e5e5e5;
  --sidebar-primary: #f59e0b;
  --muted-foreground: #a3a3a3;
  --accent-foreground: #fde68a;
  --popover-foreground: #e5e5e5;
  --primary-foreground: #000000;
  --sidebar-foreground: #e5e5e5;
  --secondary-foreground: #e5e5e5;
  --destructive-foreground: #ffffff;
  --sidebar-accent-foreground: #fde68a;
  --sidebar-primary-foreground: #ffffff;
}
```

### Pemetaan ke Tailwind

Blok `@theme inline` dari pilot tidak dipakai apa adanya, karena Tailwind membaca semua yang berawalan `--color-` sebagai warna. Baris seperti `--color-radius` dan `--color-font-sans` hanya menghasilkan kelas tak berguna seperti `bg-radius`, sementara font, radius, dan shadow tema tidak pernah terpakai. Semua pemetaan warnanya tetap sama, dan baris selain warna diganti seperti ini:

```css
@custom-variant dark (&:is(.dark *));

@theme inline {
  --color-card: var(--card);
  --color-ring: var(--ring);
  --color-input: var(--input);
  --color-muted: var(--muted);
  --color-accent: var(--accent);
  --color-border: var(--border);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  --color-popover: var(--popover);
  --color-primary: var(--primary);
  --color-sidebar: var(--sidebar);
  --color-secondary: var(--secondary);
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-destructive: var(--destructive);
  --color-sidebar-ring: var(--sidebar-ring);
  --color-sidebar-accent: var(--sidebar-accent);
  --color-sidebar-border: var(--sidebar-border);
  --color-card-foreground: var(--card-foreground);
  --color-sidebar-primary: var(--sidebar-primary);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent-foreground: var(--accent-foreground);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary-foreground: var(--primary-foreground);
  --color-sidebar-foreground: var(--sidebar-foreground);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
  --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);

  --font-sans: var(--font-inter), ui-sans-serif, system-ui, sans-serif;
  --font-mono: var(--font-jetbrains), ui-monospace, monospace;
  --font-serif: var(--font-source-serif), ui-serif, serif;

  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);

  --shadow-md: var(--shadow-offset-x) var(--shadow-offset-y) var(--shadow-blur) var(--shadow-spread)
    color-mix(in srgb, var(--shadow-color) calc(var(--shadow-opacity) * 100%), transparent);

  --tracking-normal: var(--letter-spacing);
}
```

Catatan pemasangan:

- Tiga baris `--font-*` tidak ikut di `:root`, karena font dimuat lewat `next/font` di [layout.tsx](../frontend/src/app/layout.tsx).
- Source Serif 4 belum dimuat, dan baru ditambahkan kalau ada layar yang memakainya.
- Token `.dark` sudah terpasang, tetapi belum ada yang mengaktifkannya.
- Halaman berlatar putih, jadi kartu dibedakan dari latar lewat garis tepi.

### Kelas yang dipakai

Layar baru memakai kelas di bawah ini.

| Kebutuhan | Kelas |
| --- | --- |
| Kartu dan panel | `rounded-xl border border-border bg-card` |
| Teks utama | `text-foreground` |
| Teks pendukung | `text-muted-foreground` |
| Garis pemisah | `divide-border` |
| Hover pada baris | `hover:bg-muted` |
| Tombol utama | `bg-primary text-primary-foreground hover:bg-primary/90` |
| Nama merek dan tautan | `text-accent-foreground` |
| Latar penanda | `bg-accent text-accent-foreground` |
| Fokus keyboard | `focus-visible:outline-ring` |
| Error | `bg-destructive/10 text-destructive` |

Dua aturan yang mudah terlewat:

- Jangan pakai `text-muted`. Di tema ini `muted` adalah warna latar `#f9fafb`, jadi teksnya hilang di atas putih dengan kontras 1,05:1.
- Amber hanya dipakai sebagai latar dengan teks gelap di atasnya. `text-primary` di atas putih hanya 2,15:1.

### Penyesuaian kontras

Batas aman teks adalah 4,5:1, dan batas penanda fokus adalah 3:1. Sebagian besar nilai tema lolos. Lima nilai ini perlu diganti, dan nilai aslinya masih terpasang sampai keputusan 11 diambil.

| Nilai | Mode | Asli | Kontras asli | Usulan | Kontras usulan |
| --- | --- | --- | --- | --- | --- |
| `--destructive` | Terang | `#ef4444` | 3,76:1 di atas putih, 3,29:1 di atas latar error | `#b91c1c` | 6,47:1 dan 5,45:1 |
| `--destructive` | Gelap | `#ef4444` | 4,02:1 di atas kartu | `#f87171` | 5,47:1 |
| `--destructive-foreground` | Gelap | `#ffffff` | 2,77:1 di atas `#f87171` | `#000000` | 7,59:1 |
| `--ring` | Terang | `#f59e0b` | 2,15:1 di atas putih | `#b45309` | 5,02:1 |
| `--sidebar-primary-foreground` | Keduanya | `#ffffff` | 2,15:1 di atas amber | `#000000` | 9,78:1 |

### Tema di landing page

Semua warna template ada di sembilan variabel `--color-*`, jadi penggantinya cukup di satu tempat. Salinan percobaan dengan nilai di bawah tampil tanpa error di lebar 1440 dan 375 px. Kontras pasangan utamanya juga lolos.

| Variabel | Fungsi | Asli | Baru |
| --- | --- | --- | --- |
| `--color-1` | Teks dan bagian gelap | `#1b1411` | `#262626` |
| `--color-2` | Latar halaman | `#faf5f0` | `#ffffff` |
| `--color-3` | Latar kedua | `#eee5dc` | `#f3f4f6` |
| `--color-4` | Aksen dan tombol | `#33cc7a` | `#f59e0b` |
| `--color-5` | Teks pendukung | `#66574f` | `#4b5563` |
| `--color-6` | Hero dan bagian `calls` | `#1f3954` | `#171717` |
| `--color-7` | Teks di atas `--color-6` | `#bacde0` | `#fde68a` |
| `--color-8` | Latar di belakang foto | `#2e241f` | `#1f1f1f` |
| `--color-9` | Teks pendukung di bagian gelap | `#b8aba3` | `#a3a3a3` |

Catatan pemasangannya:

- `--color-5` memakai `#4b5563`, karena `#6b7280` hanya 4,39:1 di atas `--color-3`.
- Radius besar template tetap dipakai, karena bentuk lembarannya adalah ciri desain halaman itu. Radius `0.375rem` hanya berlaku di aplikasi.
- Font diganti dari Host Grotesk ke Inter. Ukuran judul besar dan wordmark footer dihitung dari jumlah karakter, jadi keduanya diperiksa ulang setelah font diganti.

## Testing

Sebelum satu item dicentang, empat perintah ini harus lolos dari folder `frontend`:

```bash
npm test
```

```bash
npm run lint
```

```bash
npm run typecheck
```

```bash
npm run build
```

Tiap layar baru juga diuji di browser terhadap API Go sungguhan. Playwright masuk ke repositori saat F-05.

## Landing page

### Asal template

Sumbernya [template Linea dari 8B](https://8b.com/templates/linea/), yang berada di folder lokal `app-landing-page-template/`. Isinya satu file HTML berukuran 135 KB dengan CSS dan script di dalamnya, tanpa pustaka dan tanpa build. Halaman demonya menjual aplikasi teks langsung untuk telepon.

Hasil pemeriksaan:

- Tampil tanpa error konsol dan tanpa request gagal di Edge, pada lebar 1440 dan 375 px.
- Tidak ada scroll samping di kedua lebar.
- Script-nya tidak mengirim data ke mana pun. Sumber luarnya hanya foto Unsplash dan font Google.

Lisensinya:

| Bagian | Lisensi | Kewajiban |
| --- | --- | --- |
| Kode | MIT | Teks LICENSE ikut disertakan. |
| Foto | Unsplash License | Tidak berlaku lagi untuk foto yang diganti. CREDITS.md diperbarui. |
| Font | SIL OFL 1.1, baik Host Grotesk maupun Inter | Tidak ada tindakan tambahan. |

Komentar pembuka file, `<meta name="generator">`, dan tautan "Design: 8B" di footer dipertahankan, sesuai panduan template.

### Cara masuk ke Next.js

File `index.html` dipakai apa adanya sebagai halaman statis, bukan ditulis ulang ke React. Script template mengatur DOM secara langsung, jadi menulis ulang 135 KB ke komponen hanya menambah risiko tanpa manfaat bagi pengunjung.

1. Salin `index.html` ke `frontend/public/landing.html`, lalu edit di sana.
2. Salin LICENSE dan CREDITS.md ke `frontend/public/landing/`.
3. Tambahkan rewrite `/` ke `/landing.html` di `next.config.ts`.
4. Hapus `src/app/page.tsx`, karena isinya hanya pengalihan.

Pengguna yang sudah login tetap sampai ke layarnya lewat tombol Masuk, karena `/login` sudah mengalihkan mereka. Rewrite ini belum dicoba, jadi langkah 3 diuji lebih dulu saat eksekusi.

`index.gsap.html` tidak dipakai, karena ukurannya lebih besar dan butuh script dari CDN. Folder `app-landing-page-template/` tidak ikut di-commit.

### Pemetaan bagian

Tiga bagian punya jumlah item yang terkunci di CSS. Mengubah jumlahnya butuh perubahan CSS, jadi isinya disesuaikan dengan batas itu.

| Bagian template | Batas | Isi FastConsignee | Sumber fakta |
| --- | --- | --- | --- |
| `navigation` | Tidak ada | Logo, tautan bagian, tombol utama | Keputusan 3 |
| `hero` | Tidak ada | Tagline *Every sale, split instantly.* Rating diganti fakta produk: Devnet, satu transaksi, bisa diperiksa di Explorer. | PRD |
| `voice-to-text` | 3 baris | Satu pembayaran masuk, lalu terbagi menjadi bagian penitip dan komisi toko | FR-04, FR-09 |
| `calls` | 5 fitur | Pilih barang, lihat rincian, tampilkan QR, pembeli setuju sekali, semua pihak terbayar. HP di layar menyusun langkah yang sama. | K-1 sampai K-5 |
| `caption-styles` | 8 baris | Komisi per barang, penanggung diskon, potongan refund, kurs terkunci, stok tertahan, batas waktu QR, batas penitip per keranjang, wallet bertanda tangan | T-5, K-6, T-3, B-2, K-5, K-4, K-2, P-6 |
| `in-person` | Tidak ada | Jenis penitip: pengrajin, barang bekas, makanan rumahan | Latar belakang PRD |
| `testimonials` | Tidak ada | Kasus nyata | Keputusan 4 |
| `how-it-works` | 3 kartu | Toko menautkan wallet, kasir membuat QR, pembeli membayar sekali | User flow PRD |
| `faq` | Tidak ada | Devnet, kebutuhan wallet, pembayaran gagal, refund, status di Indonesia | Risiko PRD |
| `call-to-action` | Tidak ada | Ajakan utama | Keputusan 3 |
| `contact` | Tidak ada | Dihapus atau diganti | Keputusan 5 |
| `footer` | Tidak ada | Wordmark FastConsignee dengan `--len:13`, tahun, dan "Design: 8B". Tautan Privacy, Terms, dan Emergency calling dihapus. | Panduan template |

### Batas isi

Landing page hanya memuat hal yang benar hari ini:

- Tidak ada rating, ulasan, jumlah pengguna, atau dukungan pihak lain yang dikarang.
- Tidak ada harga, karena belum diputuskan.
- Selalu jelas bahwa produk berjalan di Devnet, bukan uang sungguhan.
- Tidak ada klaim siap dipakai toko di Indonesia, karena Bank Indonesia melarang kripto sebagai alat pembayaran.
- Fitur yang belum jadi tidak ditulis sebagai fitur yang sudah ada, termasuk NFT sertifikat.

### Cara uji

- Tampil benar di lebar 375 dan 1440 px, tanpa error konsol.
- Tetap terbaca penuh dengan reduced motion dan di Firefox.
- Kata Linea hanya tersisa di nama class, kalau ada, dan atribut `data-8b-placeholder` sudah habis.
- `/` menampilkan landing page, dan tombol Masuk sampai ke `/login`.
- `npm run build` lolos.
