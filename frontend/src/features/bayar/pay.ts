import { decodeTransaction } from "@/features/wallet/transaction";
import type { Api } from "@/lib/api/client";

// PayError membawa pesan dari endpoint Solana Pay, yang formatnya berbeda dari error API lain.
export class PayError extends Error {}

// requestTransaction memakai endpoint yang sama dengan QR, jadi transaksinya sama persis.
export async function requestTransaction(api: Api, orderId: string, account: string) {
  const result = await api
    .POST("/api/v1/solana-pay/orders/{order_id}", { params: { path: { order_id: orderId } }, body: { account } })
    .catch(() => null);
  if (!result) {
    throw new PayError("API tidak bisa dihubungi. Coba lagi sebentar lagi.");
  }
  if (result.data) {
    return decodeTransaction(result.data.transaction);
  }
  const body = result.error as { message?: unknown } | undefined;
  throw new PayError(typeof body?.message === "string" ? body.message : "Transaksi tidak bisa disusun. Coba lagi.");
}

export function payErrorMessage(error: unknown): string {
  if (error instanceof PayError) {
    return error.message;
  }
  return "Pembayaran dibatalkan atau gagal di wallet. Saldo tidak berubah, jadi kamu bisa mencoba lagi.";
}
