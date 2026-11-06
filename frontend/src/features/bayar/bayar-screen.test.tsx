import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ok, problem } from "@/test/api";
import { BayarScreen } from "./bayar-screen";
import { requestTransaction } from "./pay";

const address = "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a";
const get = vi.fn();
const api = { GET: get };
const sendTransaction = vi.fn();
const connection = {};

let wallet: Record<string, unknown>;

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ api }) }));
vi.mock("@/features/wallet/solana-provider", () => ({
  SolanaProvider: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("@solana/wallet-adapter-react", () => ({
  useConnection: () => ({ connection }),
  useWallet: () => wallet,
}));
vi.mock("./pay", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./pay")>()),
  requestTransaction: vi.fn(),
}));

function publicOrder(status: string, extra: Record<string, unknown> = {}) {
  return {
    id: "o1",
    status,
    store_name: "Toko Titip Nusantara",
    lines: [{ item_name: "Dompet Pandan", quantity: 1, line_total_idr: 45000, certificate: null }],
    total_idr: 45000,
    total_lamports: "1000000000",
    rate: { idr_per_sol: 45000 },
    expires_at: new Date(Date.now() + 120_000).toISOString(),
    solana_pay_url: "solana:https://api.test/pay",
    confirmed_at: null,
    explorer_url: null,
    ...extra,
  };
}

describe("BayarScreen", () => {
  beforeEach(() => {
    get.mockReset();
    sendTransaction.mockReset();
    vi.mocked(requestTransaction).mockReset();
    wallet = { publicKey: null, sendTransaction, disconnect: vi.fn(), wallets: [], select: vi.fn(), connecting: false };
  });

  it("menampilkan rincian, label Devnet, dan QR untuk pembeli tanpa wallet", async () => {
    get.mockImplementation(() => ok(publicOrder("pending")));
    render(<BayarScreen orderId="o1" />);

    expect(await screen.findByRole("heading", { name: "Toko Titip Nusantara" })).toBeVisible();
    expect(screen.getByText("Devnet")).toBeVisible();
    expect(screen.getByText(/tidak memakai uang sungguhan/)).toBeVisible();
    expect(screen.getByRole("link", { name: "Pasang Phantom" })).toBeVisible();
    expect(screen.getByTitle("QR pembayaran Solana Pay")).toBeInTheDocument();
  });

  it("membayar dengan transaksi dari server", async () => {
    const transaction = {} as Awaited<ReturnType<typeof requestTransaction>>;
    get.mockImplementation(() => ok(publicOrder("pending")));
    vi.mocked(requestTransaction).mockResolvedValue(transaction);
    sendTransaction.mockResolvedValue("signature");
    wallet.publicKey = { toBase58: () => address };
    const user = userEvent.setup();
    render(<BayarScreen orderId="o1" />);

    await user.click(await screen.findByRole("button", { name: "Bayar sekali" }));

    expect(await screen.findByText("Transaksi terkirim. Menunggu konfirmasi jaringan...")).toBeVisible();
    expect(requestTransaction).toHaveBeenCalledWith(api, "o1", address);
    expect(sendTransaction).toHaveBeenCalledWith(transaction, connection);
  });

  it("boleh mencoba lagi kalau pembeli menolak di wallet", async () => {
    get.mockImplementation(() => ok(publicOrder("pending")));
    vi.mocked(requestTransaction).mockResolvedValue({} as Awaited<ReturnType<typeof requestTransaction>>);
    sendTransaction.mockRejectedValue(new Error("User rejected"));
    wallet.publicKey = { toBase58: () => address };
    const user = userEvent.setup();
    render(<BayarScreen orderId="o1" />);

    await user.click(await screen.findByRole("button", { name: "Bayar sekali" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Saldo tidak berubah, jadi kamu bisa mencoba lagi.");
    await waitFor(() => expect(screen.getByRole("button", { name: "Bayar sekali" })).toBeEnabled());
  });

  it("menampilkan bukti setelah terkonfirmasi", async () => {
    get.mockImplementation(() =>
      ok(publicOrder("confirmed", { confirmed_at: "2026-11-07T00:01:00Z", explorer_url: "https://explorer.test/tx" })),
    );
    render(<BayarScreen orderId="o1" />);

    expect(await screen.findByText("Pembayaran terkonfirmasi. Terima kasih.")).toBeVisible();
    expect(screen.getByRole("link", { name: "Lihat di Solana Explorer" })).toHaveAttribute("href", "https://explorer.test/tx");
    expect(screen.queryByTitle("QR pembayaran Solana Pay")).not.toBeInTheDocument();
  });

  it("mengarahkan pembeli ke kasir kalau QR kedaluwarsa", async () => {
    get.mockImplementation(() => ok(publicOrder("expired")));
    render(<BayarScreen orderId="o1" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Minta kasir membuat QR baru.");
  });

  it("memberi tahu kalau order tidak ada", async () => {
    get.mockImplementation(() => problem("NOT_FOUND", 404));
    render(<BayarScreen orderId="o1" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Data tidak ditemukan.");
  });
});
