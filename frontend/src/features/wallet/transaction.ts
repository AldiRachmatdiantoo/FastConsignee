import { Transaction } from "@solana/web3.js";

// decodeTransaction membaca transaksi base64 dari API tanpa bergantung pada Buffer.
export function decodeTransaction(encoded: string): Transaction {
  return Transaction.from(Uint8Array.from(atob(encoded), (char) => char.charCodeAt(0)));
}
