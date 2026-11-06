import { ApiError, ApiUnavailableError, type ErrorCode } from "./client";

const MESSAGES: Partial<Record<ErrorCode, string>> = {
  FORBIDDEN: "Akun ini tidak punya akses ke data tersebut.",
  NOT_FOUND: "Data tidak ditemukan.",
  RATE_LIMITED: "Terlalu banyak permintaan. Tunggu sebentar, lalu coba lagi.",
  SERVICE_UNAVAILABLE: "Jaringan Solana sedang tidak bisa dihubungi. Coba lagi sebentar lagi.",
  WALLET_NOT_LINKED: "Tautkan wallet lebih dulu.",
  STORE_WALLET_NOT_LINKED: "Wallet toko belum tertaut. Minta pemilik toko menautkannya.",
  RECIPIENT_WALLET_MISSING: "Ada penitip yang belum punya wallet. Lengkapi dulu di daftar penitip.",
  BELOW_RENT_MINIMUM: "Bagian salah satu penerima terlalu kecil untuk wallet yang masih kosong. Tambah barang atau isi wallet itu.",
  INSUFFICIENT_STOCK: "Stok tidak cukup. Kurangi jumlah barang.",
  TOO_MANY_CONSIGNORS: "Satu keranjang paling banyak memuat 5 penitip.",
  DISCOUNT_TOO_LARGE: "Diskon melebihi bagian pihak yang menanggungnya.",
  RATE_UNAVAILABLE: "Kurs SOL sedang tidak tersedia. Coba lagi sebentar lagi.",
  STOCK_BELOW_RESERVED: "Stok tidak boleh lebih kecil dari jumlah yang sedang tertahan.",
  CONSIGNOR_EMAIL_TAKEN: "Email ini sudah dipakai penitip lain.",
  ORDER_NOT_REFUNDABLE: "Order ini belum terkonfirmasi, jadi belum bisa di-refund.",
  ALREADY_REFUNDED: "Semua unit di baris ini sudah di-refund.",
  REFUND_IN_PROGRESS: "Masih ada refund yang berjalan untuk baris ini.",
  REFUND_NOT_AWAITING_SIGNATURE: "Refund ini tidak lagi menunggu tanda tangan.",
};

// errorMessage mengubah kegagalan API menjadi kalimat untuk pengguna. Keputusannya memakai `code`.
export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "VALIDATION_FAILED" && error.problem.errors?.length) {
      return error.problem.errors.map(({ message }) => message).join(" ");
    }
    return MESSAGES[error.code] ?? error.message;
  }
  if (error instanceof ApiUnavailableError) {
    return error.message;
  }
  return "Terjadi kesalahan. Coba lagi.";
}
