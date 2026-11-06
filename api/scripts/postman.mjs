// Membuat koleksi Postman dari openapi.yaml. Jalankan ulang tiap kontrak berubah: npm run postman
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import Converter from "openapi-to-postmanv2";
import YAML from "yaml";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const output = join(root, "postman", "FastConsignee.postman_collection.json");

// ID tetap supaya import ulang menimpa koleksi lama, bukan membuat salinan.
const collectionId = "6f2f1a52-3c1e-4b7a-9d55-0b6e8c4a1f10";

// Urutan alur kerja, jadi ID dari satu langkah sudah tersimpan saat langkah berikutnya butuh.
const folderOrder = ["Health", "Auth", "Consignors", "Items", "Orders", "Payment", "Refunds", "Dashboard"];

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

function convert(spec) {
  const options = { folderStrategy: "Tags", parametersResolution: "Schema", requestNameSource: "Fallback", enableOptionalParameters: false };
  return new Promise((resolve, reject) => {
    Converter.convertV2({ type: "string", data: spec }, options, (err, result) => {
      if (err) return reject(err);
      if (!result.result) return reject(new Error(result.reason));
      resolve(result.output[0].data);
    });
  });
}

function operationKey(request) {
  const path = request.url.path.map((part) => part.replace(/^:(.+)$/, "{$1}")).join("/");
  return `${request.method} /${path}`;
}

function description(value) {
  return typeof value === "string" ? value : (value?.content ?? "");
}

function cleanRequest(item, operation) {
  const key = operationKey(item.request);
  const { request } = item;
  delete item.id;
  delete item.response;
  delete item.protocolProfileBehavior;
  delete item.event;

  const roles = operation["x-roles"].map((role) => `\`${role}\``).join(", ");
  request.description = `**Peran:** ${roles}\n\n${description(request.description)}`.trim();

  delete request.auth;
  if (operation.security?.length === 0) {
    request.auth = { type: "noauth" };
  }

  for (const header of request.header ?? []) {
    if (header.key === "Idempotency-Key") {
      header.value = "{{$guid}}";
    }
  }
  for (const param of request.url.variable ?? []) {
    param.value = `{{${param.key}}}`;
  }
  for (const param of request.url.query ?? []) {
    param.value = param.key === "limit" ? "20" : "";
  }

  if (operation.requestBody) {
    if (!bodies[key]) {
      throw new Error(`Contoh body belum ada untuk ${key}`);
    }
    request.body.raw = JSON.stringify(bodies[key], null, 2);
  }
  const exec = [...checkScript(operation), ...(saves[key] ? saveScript(saves[key]) : [])];
  item.event = [{ listen: "test", script: { type: "text/javascript", exec } }];
}

const spec = readFileSync(join(root, "openapi.yaml"), "utf8");
const contract = YAML.parse(spec);
const collection = await convert(spec);

const operations = new Map();
for (const [path, methods] of Object.entries(contract.paths)) {
  for (const [method, operation] of Object.entries(methods)) {
    if (operation.operationId) {
      operations.set(`${method.toUpperCase()} ${path}`, operation);
    }
  }
}

let converted = 0;
for (const folder of collection.item) {
  delete folder.id;
  for (const item of folder.item) {
    const operation = operations.get(operationKey(item.request));
    if (!operation) {
      throw new Error(`Request tidak ada di kontrak: ${operationKey(item.request)}`);
    }
    cleanRequest(item, operation);
    converted++;
  }
}
if (converted !== operations.size) {
  throw new Error(`Kontrak punya ${operations.size} operasi, koleksi punya ${converted}`);
}

collection.item.sort((a, b) => folderOrder.indexOf(a.name) - folderOrder.indexOf(b.name));
if (collection.item.some((folder) => !folderOrder.includes(folder.name))) {
  throw new Error("Ada tag baru yang belum masuk folderOrder");
}

collection.info._postman_id = collectionId;
collection.info.version = contract.info.version;
collection.event = [];
collection.variable = variables.map(([key, value, note]) => ({
  key,
  value: key === "baseUrl" ? contract.servers[0].url : value,
  type: "string",
  description: note,
}));

mkdirSync(dirname(output), { recursive: true });
writeFileSync(output, `${JSON.stringify(collection, null, 2)}\n`);
console.log(`${converted} request ditulis ke postman/FastConsignee.postman_collection.json`);
