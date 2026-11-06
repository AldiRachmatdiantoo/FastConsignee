// Node Solana tiruan untuk uji Playwright. Tanda tangan diperiksa sungguhan, dan transfer mengubah saldo.
// Dengan begitu alur bayar dan refund bisa diuji penuh tanpa SOL Devnet.
import { createServer } from "node:http";
import { Message, SystemInstruction, SystemProgram, Transaction } from "@solana/web3.js";
import bs58 from "bs58";

const PORT = Number(process.env.SOLANA_STUB_PORT ?? 8899);
const DEFAULT_BALANCE = 2_000_000_000;
const RENT_MINIMUM = 890_880;
const FEE = 5_000;

const balances = new Map();
const byAddress = new Map();
const transactions = new Map();
let slot = 1000;

const balanceOf = (address) => balances.get(address) ?? DEFAULT_BALANCE;
const context = () => ({ context: { slot: ++slot, apiVersion: "2.0.0" } });

function blockhash() {
  return bs58.encode(Buffer.alloc(32, (slot % 250) + 1));
}

function accountKeys(raw) {
  const signatures = raw[0];
  return Message.from(raw.subarray(1 + signatures * 64)).accountKeys.map((key) => key.toBase58());
}

function send(encoded) {
  const raw = Buffer.from(encoded, "base64");
  const transaction = Transaction.from(raw);
  if (!transaction.verifySignatures()) {
    throw new Error("signature verification failed");
  }
  const keys = accountKeys(raw);
  const signature = bs58.encode(transaction.signature);
  const before = keys.map(balanceOf);

  const payer = keys[0];
  balances.set(payer, balanceOf(payer) - FEE);
  const transfers = [];
  for (const instruction of transaction.instructions) {
    if (!instruction.programId.equals(SystemProgram.programId)) {
      continue;
    }
    const { fromPubkey, toPubkey, lamports } = SystemInstruction.decodeTransfer(instruction);
    const from = fromPubkey.toBase58();
    const to = toPubkey.toBase58();
    balances.set(from, balanceOf(from) - Number(lamports));
    balances.set(to, balanceOf(to) + Number(lamports));
    transfers.push({ from, to, lamports: Number(lamports) });
  }

  transactions.set(signature, { encoded, slot: ++slot, before, after: keys.map(balanceOf), transfers });
  for (const key of keys) {
    byAddress.set(key, [signature, ...(byAddress.get(key) ?? [])]);
  }
  return signature;
}

const methods = {
  getHealth: () => "ok",
  getVersion: () => ({ "solana-core": "2.0.0", "feature-set": 1 }),
  getSlot: () => slot,
  getBlockHeight: () => slot,
  getGenesisHash: () => "EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG",
  getLatestBlockhash: () => ({ ...context(), value: { blockhash: blockhash(), lastValidBlockHeight: slot + 150 } }),
  getBalance: ([address]) => ({ ...context(), value: balanceOf(address) }),
  getMinimumBalanceForRentExemption: () => RENT_MINIMUM,
  getMultipleAccounts: ([addresses]) => ({
    ...context(),
    value: addresses.map((address) => ({
      lamports: balanceOf(address),
      owner: "11111111111111111111111111111111",
      data: ["", "base64"],
      executable: false,
      rentEpoch: 0,
      space: 0,
    })),
  }),
  sendTransaction: ([encoded]) => send(encoded),
  getSignaturesForAddress: ([address]) =>
    (byAddress.get(address) ?? []).map((signature) => ({
      signature,
      slot: transactions.get(signature).slot,
      err: null,
      memo: null,
      blockTime: Math.floor(Date.now() / 1000),
      confirmationStatus: "confirmed",
    })),
  getSignatureStatuses: ([signatures]) => ({
    ...context(),
    value: signatures.map((signature) =>
      transactions.has(signature)
        ? { slot: transactions.get(signature).slot, confirmations: null, err: null, confirmationStatus: "confirmed" }
        : null,
    ),
  }),
  getTransaction: ([signature]) => {
    const found = transactions.get(signature);
    if (!found) {
      return null;
    }
    return {
      slot: found.slot,
      blockTime: Math.floor(Date.now() / 1000),
      version: "legacy",
      transaction: [found.encoded, "base64"],
      meta: {
        err: null,
        fee: FEE,
        preBalances: found.before,
        postBalances: found.after,
        innerInstructions: [],
        logMessages: [],
        preTokenBalances: [],
        postTokenBalances: [],
        rewards: [],
        loadedAddresses: { readonly: [], writable: [] },
      },
    };
  },
  // Bukan metode Solana. Test memakainya untuk memeriksa isi transaksi yang terkirim.
  stubTransaction: ([signature]) => transactions.get(signature) ?? null,
};

function answer(call) {
  const method = methods[call.method];
  if (!method) {
    return { jsonrpc: "2.0", id: call.id, error: { code: -32601, message: `Method not found: ${call.method}` } };
  }
  try {
    return { jsonrpc: "2.0", id: call.id, result: method(call.params ?? []) };
  } catch (error) {
    return { jsonrpc: "2.0", id: call.id, error: { code: -32002, message: error.message } };
  }
}

const server = createServer((request, response) => {
  response.setHeader("Access-Control-Allow-Origin", "*");
  response.setHeader("Access-Control-Allow-Headers", "*");
  if (request.method === "OPTIONS") {
    response.writeHead(204).end();
    return;
  }
  if (request.method === "GET") {
    response.writeHead(200).end("ok");
    return;
  }
  let body = "";
  request.on("data", (chunk) => (body += chunk));
  request.on("end", () => {
    const call = JSON.parse(body);
    const result = Array.isArray(call) ? call.map(answer) : answer(call);
    response.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify(result));
  });
});

server.listen(PORT, "127.0.0.1", () => console.log(`Solana tiruan di http://127.0.0.1:${PORT}`));
