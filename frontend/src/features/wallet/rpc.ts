// Layar kasir memuat berkas ini, jadi @solana/web3.js yang besar sengaja tidak diimpor di sini.
export const RPC_URL = process.env.NEXT_PUBLIC_SOLANA_RPC_URL || "https://api.devnet.solana.com";

type AccountsResponse = {
  result?: { value: ({ lamports: number } | null)[] };
  error?: { message: string };
};

// readBalances membaca saldo beberapa wallet dalam satu panggilan RPC.
export async function readBalances(addresses: string[]): Promise<Record<string, bigint>> {
  const response = await fetch(RPC_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      method: "getMultipleAccounts",
      params: [addresses, { commitment: "confirmed", encoding: "base64", dataSlice: { offset: 0, length: 0 } }],
    }),
  });
  const body = (await response.json()) as AccountsResponse;
  if (!body.result) {
    throw new Error(body.error?.message ?? `RPC menjawab ${response.status}`);
  }
  const { value } = body.result;
  return Object.fromEntries(addresses.map((address, index) => [address, BigInt(value[index]?.lamports ?? 0)]));
}
