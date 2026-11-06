import bs58 from "bs58";
import { ApiError, ApiUnavailableError, unwrap, type Api } from "@/lib/api/client";

type LinkOptions = {
  api: Api;
  address: string;
  signMessage: (message: Uint8Array) => Promise<Uint8Array>;
};

// linkWallet meminta pesan dari API, menandatanganinya, lalu mengirim buktinya.
export async function linkWallet({ api, address, signMessage }: LinkOptions) {
  const nonce = await unwrap(api.POST("/api/v1/wallet/nonce", { body: { address } }));
  const signature = await signMessage(new TextEncoder().encode(nonce.message));
  return unwrap(
    api.POST("/api/v1/wallet/link", {
      body: { address, nonce: nonce.nonce, signature: bs58.encode(signature) },
    }),
  );
}

// linkErrorMessage mengubah kegagalan menjadi langkah yang bisa dilakukan pengguna.
export function linkErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case "NONCE_INVALID":
        return "Pesan sudah tidak berlaku. Coba tandatangani lagi.";
      case "SIGNATURE_INVALID":
        return "Tanda tangan tidak cocok. Pastikan akun aktif di wallet sama dengan alamat di layar.";
      case "WALLET_ALREADY_LINKED":
        return "Wallet ini sudah tertaut ke pihak lain. Pilih wallet lain.";
      default:
        return error.message;
    }
  }
  if (error instanceof ApiUnavailableError) {
    return error.message;
  }
  return "Penandatanganan dibatalkan atau gagal di wallet. Saldo tidak berubah.";
}

export function shortAddress(address: string): string {
  return `${address.slice(0, 6)}...${address.slice(-6)}`;
}
