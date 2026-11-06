// Membuat koleksi Postman dari openapi.yaml. Jalankan ulang tiap kontrak berubah: npm run postman
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import YAML from "yaml";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const output = join(root, "postman", "FastConsignee.postman_collection.json");

// ID tetap supaya import ulang menimpa koleksi lama, bukan membuat salinan.
const collectionId = "6f2f1a52-3c1e-4b7a-9d55-0b6e8c4a1f10";

// Urutan alur kerja, jadi ID dari satu langkah sudah tersimpan saat langkah berikutnya butuh.
const folderOrder = ["Health", "Auth", "Consignors", "Items", "Orders", "Payment", "Refunds", "Dashboard"];

const methods = ["get", "post", "patch", "put", "delete"];

// ID kosong membuat alamat seperti /refunds//signature, yang jawabannya bukan jawaban endpoint itu.
const noId = "00000000-0000-0000-0000-000000000000";

const variables = [
  ["baseUrl", "", "Alamat API."],
  ["bearerToken", "", "JWT Supabase. Peran yang boleh memanggil tiap request tertulis di deskripsinya."],
  ["wallet_address", "", "Alamat wallet yang akan ditautkan. Harus wallet yang bisa kamu tanda tangani."],
  ["wallet_signature", "", "Tanda tangan base58 atas wallet_message."],
  ["consignor_wallet", "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a", "Contoh saja, kuncinya sudah dibuang. Ganti dengan wallet Devnet penitip."],
  ["payer_wallet", "7Pxwcgh7QWWfjLrvbdKHm5ssob54s8ynyKHJ1tFz8zYk", "Contoh saja, kuncinya sudah dibuang. Ganti dengan wallet pembeli."],
  ["refund_signature", "", "Signature transaksi refund setelah dikirim wallet toko."],
  ["wallet_nonce", "", "Terisi otomatis."],
  ["wallet_message", "", "Terisi otomatis. Pesan yang harus ditandatangani wallet."],
  ["consignor_id", noId, "Terisi otomatis."],
  ["item_id", noId, "Terisi otomatis."],
  ["order_id", noId, "Terisi otomatis."],
  ["order_line_id", noId, "Terisi otomatis."],
  ["refund_id", noId, "Terisi otomatis."],
  ["certificate_id", noId, "Isi dengan ID sertifikat yang sudah ada."],
];

const cart = { lines: [{ item_id: "{{item_id}}", quantity: 1 }] };

const bodies = {
  "POST /api/v1/wallet/nonce": { address: "{{wallet_address}}" },
  "POST /api/v1/wallet/link": { address: "{{wallet_address}}", nonce: "{{wallet_nonce}}", signature: "{{wallet_signature}}" },
  "POST /api/v1/consignors": { name: "Sari Anyaman", email: "sari+{{$timestamp}}@example.com", wallet_address: "{{consignor_wallet}}" },
  "PATCH /api/v1/consignors/{consignor_id}": { name: "Sari Anyaman Nusantara" },
  "POST /api/v1/items": { name: "Tas Rotan Bundar", price_idr: 185000, commission_bps: 2000, consignor_id: "{{consignor_id}}", stock_on_hand: 12 },
  "PATCH /api/v1/items/{item_id}": { price_idr: 190000, stock_on_hand: 10 },
  "POST /api/v1/orders/preview": cart,
  "POST /api/v1/orders": cart,
  "POST /api/v1/solana-pay/orders/{order_id}": { account: "{{payer_wallet}}" },
  "POST /api/v1/orders/{order_id}/refunds": { order_line_id: "{{order_line_id}}", reason: "Barang cacat" },
  "POST /api/v1/refunds/{refund_id}/signature": { signature: "{{refund_signature}}" },
};

// Nilai dari jawaban yang disimpan ke variabel koleksi.
const saves = {
  "POST /api/v1/wallet/nonce": { wallet_nonce: "body.nonce", wallet_message: "body.message" },
  "POST /api/v1/consignors": { consignor_id: "body.id" },
  "POST /api/v1/items": { item_id: "body.id" },
  "POST /api/v1/orders": { order_id: "body.id", order_line_id: "body.lines[0].id" },
  "POST /api/v1/orders/{order_id}/refunds": { refund_id: "body.id" },
};

function saveScript(fields) {
  const lines = Object.entries(fields).map(([name, path]) => `    pm.collectionVariables.set("${name}", ${path});`);
  return ["if (pm.response.code < 300) {", "    const body = pm.response.json();", ...lines, "}"];
}

// Tiap request diperiksa terhadap kontrak, jadi Newman gagal kalau API menjawab di luar kontrak.
function checkScript(operation) {
  const statuses = Object.keys(operation.responses).map(Number);
  return [
    `pm.test("status ada di kontrak", function () { pm.expect(${JSON.stringify(statuses)}).to.include(pm.response.code); });`,
    'pm.test("tidak ada error server", function () { pm.expect(pm.response.code).to.be.below(500); });',
    'pm.test("memuat X-Request-Id", function () { pm.response.to.have.header("X-Request-Id"); });',
  ];
}

const contract = YAML.parse(readFileSync(join(root, "openapi.yaml"), "utf8"));

function resolve(node) {
  if (!node.$ref) {
    return node;
  }
  return node.$ref
    .split("/")
    .slice(1)
    .reduce((value, key) => value[key], contract);
}

function note(parameter) {
  const prefix = parameter.required ? "(Required) " : "";
  return { content: `${prefix}${(parameter.description ?? "").trim()}`, type: "text/plain" };
}

function queryNote(parameter) {
  const choices = resolve(parameter.schema ?? {}).enum;
  const hint = choices ? `Pilihan: ${choices.join(", ")}.` : "";
  return [(parameter.description ?? "").trim(), hint].filter(Boolean).join(" ");
}

function headers(operation, parameters) {
  const list = parameters
    .filter((parameter) => parameter.in === "header")
    .map((parameter) => {
      if (parameter.name !== "Idempotency-Key") {
        throw new Error(`Nilai header belum ada untuk ${parameter.name}`);
      }
      return { disabled: false, description: note(parameter), key: parameter.name, value: "{{$guid}}" };
    });
  if (operation.requestBody) {
    list.push({ key: "Content-Type", value: "application/json" });
  }
  list.push({ key: "Accept", value: "application/json" });
  return list;
}

function url(path, parameters) {
  const pathParts = path.slice(1).split("/");
  return {
    path: pathParts.map((part) => part.replace(/^\{(.+)\}$/, ":$1")),
    host: ["{{baseUrl}}"],
    query: parameters
      .filter((parameter) => parameter.in === "query")
      .map((parameter) => ({
        disabled: true,
        description: { content: queryNote(parameter), type: "text/plain" },
        key: parameter.name,
        value: parameter.name === "limit" ? "20" : "",
      })),
    variable: parameters
      .filter((parameter) => parameter.in === "path")
      .map((parameter) => ({
        type: "any",
        value: `{{${parameter.name}}}`,
        key: parameter.name,
        disabled: false,
        description: note(parameter),
      })),
  };
}

function body(key, operation) {
  if (!operation.requestBody) {
    return {};
  }
  if (!bodies[key]) {
    throw new Error(`Contoh body belum ada untuk ${key}`);
  }
  return {
    mode: "raw",
    raw: JSON.stringify(bodies[key], null, 2),
    options: { raw: { headerFamily: "json", language: "json" } },
  };
}

function toItem(path, method, operation, shared) {
  const key = `${method.toUpperCase()} ${path}`;
  const parameters = [...shared, ...(operation.parameters ?? [])].map(resolve);
  const roles = operation["x-roles"].map((role) => `\`${role}\``).join(", ");
  const request = {
    name: operation.summary,
    description: `**Peran:** ${roles}\n\n${(operation.description ?? "").trim()}`.trim(),
    url: url(path, parameters),
    header: headers(operation, parameters),
    method: method.toUpperCase(),
    body: body(key, operation),
  };
  if (operation.security?.length === 0) {
    request.auth = { type: "noauth" };
  }
  const exec = [...checkScript(operation), ...(saves[key] ? saveScript(saves[key]) : [])];
  return { name: operation.summary, request, event: [{ listen: "test", script: { type: "text/javascript", exec } }] };
}

const folders = new Map(contract.tags.map((tag) => [tag.name, { name: tag.name, description: tag.description, item: [] }]));
let converted = 0;
for (const [path, entry] of Object.entries(contract.paths)) {
  // Urutan operasi mengikuti kontrak, karena POST /orders harus lebih dulu dari GET /orders.
  for (const [method, operation] of Object.entries(entry).filter(([name]) => methods.includes(name))) {
    const folder = folders.get(operation.tags[0]);
    if (!folder) {
      throw new Error(`Tag belum terdaftar: ${operation.tags[0]}`);
    }
    folder.item.push(toItem(path, method, operation, entry.parameters ?? []));
    converted++;
  }
}

const item = [...folders.values()].sort((a, b) => folderOrder.indexOf(a.name) - folderOrder.indexOf(b.name));
if (item.some((folder) => !folderOrder.includes(folder.name))) {
  throw new Error("Ada tag baru yang belum masuk folderOrder");
}

const collection = {
  item,
  auth: { type: "bearer", bearer: [{ type: "any", value: "{{bearerToken}}", key: "token" }] },
  event: [],
  variable: variables.map(([key, value, description]) => ({
    key,
    value: key === "baseUrl" ? contract.servers[0].url : value,
    type: "string",
    description,
  })),
  info: {
    _postman_id: collectionId,
    name: contract.info.title,
    version: contract.info.version,
    schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
    description: { content: contract.info.description, type: "text/plain" },
  },
};

mkdirSync(dirname(output), { recursive: true });
writeFileSync(output, `${JSON.stringify(collection, null, 2)}\n`);
console.log(`${converted} request ditulis ke postman/FastConsignee.postman_collection.json`);
