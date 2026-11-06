import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ok, page } from "@/test/api";
import { RefundScreen } from "./refund-screen";

const storeWallet = "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a";
const otherWallet = "9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin";
const get = vi.fn();
const post = vi.fn();
const api = { GET: get, POST: post };
const sendTransaction = vi.fn();
const session = { status: "authenticated", me: { wallet: { address: storeWallet } } };

let wallet: Record<string, unknown>;

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ session, api }) }));
vi.mock("@/features/wallet/solana-provider", () => ({
  SolanaProvider: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("@/features/wallet/transaction", () => ({ decodeTransaction: () => ({}) }));
vi.mock("@solana/wallet-adapter-react", () => ({
  useConnection: () => ({ connection: {} }),
  useWallet: () => wallet,
}));

const summary = { id: "o1", total_idr: 45000, item_count: 1, confirmed_at: "2026-11-07T00:01:00Z", created_at: "2026-11-07T00:00:00Z" };
const detail = {
  lines: [{ id: "l1", item_name: "Dompet Pandan", quantity: 1, refunded_quantity: 0, consignor: { name: "Sari Anyaman" } }],
};
const draft = { id: "r1", item_name: "Dompet Pandan", status: "awaiting_signature", transaction: "dHg=" };

function connect(address: string) {
  wallet.publicKey = { toBase58: () => address };
}

function createCalls() {
  return post.mock.calls.filter(([path]) => path === "/api/v1/orders/{order_id}/refunds");
}

async function openOrder(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: /Rp/ }));
  return screen.findByRole("button", { name: "Refund satu Dompet Pandan" });
}

describe("RefundScreen", () => {
  beforeEach(() => {
    get.mockReset();
    post.mockReset();
    sendTransaction.mockReset();
    wallet = { publicKey: null, sendTransaction, disconnect: vi.fn(), wallets: [], select: vi.fn(), connecting: false };
    get.mockImplementation((path: string) => {
      if (path === "/api/v1/orders") {
        return page([summary]);
      }
      if (path === "/api/v1/orders/{order_id}") {
        return ok(detail);
      }
      if (path === "/api/v1/refunds/{refund_id}") {
        return ok({ ...draft, status: "confirming" });
      }
      return page([]);
    });
    post.mockImplementation((path: string) =>
      path === "/api/v1/orders/{order_id}/refunds" ? ok(draft) : ok({ ...draft, status: "confirming" }),
    );
  });

  it("menolak wallet selain wallet toko", async () => {
    connect(otherWallet);
    const user = userEvent.setup();
    render(<RefundScreen />);

    const button = await openOrder(user);

    expect(screen.getByRole("alert")).toHaveTextContent("Ini bukan wallet toko.");
    expect(button).toBeDisabled();
  });

  it("membuat refund, menandatangani, lalu mengikuti statusnya", async () => {
    connect(storeWallet);
    sendTransaction.mockResolvedValue("signature");
    const user = userEvent.setup();
    render(<RefundScreen />);

    await user.click(await openOrder(user));

    expect(await screen.findByText("Refund Dompet Pandan: Menunggu konfirmasi")).toBeVisible();
    const [, create] = createCalls()[0] as [string, { params: { header: Record<string, string> }; body: unknown }];
    expect(create.params.header["Idempotency-Key"]).toBeTruthy();
    expect(create.body).toEqual({ order_line_id: "l1" });
    expect(post).toHaveBeenCalledWith("/api/v1/refunds/{refund_id}/signature", {
      params: { path: { refund_id: "r1" } },
      body: { signature: "signature" },
    });
  });

  it("menandatangani ulang tanpa membuat refund baru", async () => {
    connect(storeWallet);
    sendTransaction.mockRejectedValueOnce(new Error("User rejected")).mockResolvedValue("signature");
    const user = userEvent.setup();
    render(<RefundScreen />);

    await user.click(await openOrder(user));

    expect(await screen.findByRole("alert")).toHaveTextContent("Tanda tangan dibatalkan atau gagal di wallet.");
    await user.click(screen.getByRole("button", { name: "Tandatangani lagi" }));

    await waitFor(() => expect(sendTransaction).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("Refund Dompet Pandan: Menunggu konfirmasi")).toBeVisible();
    expect(createCalls()).toHaveLength(1);
  });

  it("mematikan tombol kalau semua unit sudah di-refund", async () => {
    connect(storeWallet);
    get.mockImplementation((path: string) => {
      if (path === "/api/v1/orders") {
        return page([summary]);
      }
      if (path === "/api/v1/orders/{order_id}") {
        return ok({ lines: [{ ...detail.lines[0], refunded_quantity: 1 }] });
      }
      return page([]);
    });
    const user = userEvent.setup();
    render(<RefundScreen />);

    expect(await openOrder(user)).toBeDisabled();
  });
});
