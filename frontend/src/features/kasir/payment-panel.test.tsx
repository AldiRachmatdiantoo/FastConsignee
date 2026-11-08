import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { components } from "@/lib/api/schema";
import { ok } from "@/test/api";
import { PaymentPanel } from "./payment-panel";

type Order = components["schemas"]["Order"];

const get = vi.fn();
const api = { GET: get };

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ api }) }));
vi.mock("./balance-panel", () => ({ BalancePanel: () => null }));

function order(status: Order["status"], failureCode: string | null = null) {
  return {
    id: "o1",
    status,
    total_idr: 45000,
    total_lamports: "1000000000",
    created_at: "2026-11-07T00:00:00Z",
    expires_at: new Date(Date.now() + 120_000).toISOString(),
    recipients: [],
    payment: {
      solana_pay_url: "solana:https://api.test/pay",
      signature: null,
      explorer_url: null,
      failure_code: failureCode,
    },
  } as unknown as Order;
}

describe("PaymentPanel", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("menampilkan QR, hitung mundur, dan tautan layar bayar", () => {
    render(<PaymentPanel order={order("pending")} onRetry={vi.fn()} onDone={vi.fn()} />);

    expect(screen.getByTitle("Solana Pay payment QR")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/Waiting for payment · [12]:\d\d/);
    expect(screen.getByRole("link", { name: "Open the pay screen in a browser" })).toHaveAttribute("href", "/bayar/o1");
  });

  it("berubah sendiri saat pembayaran terkonfirmasi", async () => {
    get.mockImplementation(() =>
      ok({ id: "o1", status: "confirmed", expires_at: "2026-11-07T00:05:00Z", explorer_url: "https://explorer.test/tx" }),
    );
    const onDone = vi.fn();
    const user = userEvent.setup();
    render(<PaymentPanel order={order("pending")} onRetry={vi.fn()} onDone={onDone} />);

    expect(await screen.findByText("Payment confirmed. The goods can be handed over.", {}, { timeout: 3500 })).toBeVisible();
    expect(screen.queryByTitle("Solana Pay payment QR")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View on Solana Explorer" })).toHaveAttribute("href", "https://explorer.test/tx");

    await user.click(screen.getByRole("button", { name: "New transaction" }));
    expect(onDone).toHaveBeenCalledOnce();
  });

  it("menawarkan QR baru setelah kedaluwarsa", async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(<PaymentPanel order={order("expired")} onRetry={onRetry} onDone={vi.fn()} />);

    expect(screen.getByRole("alert")).toHaveTextContent("The QR expired, and the stock was released.");
    await user.click(screen.getByRole("button", { name: "Create a new QR" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("melarang menyerahkan barang kalau transaksi tidak cocok dengan order", () => {
    render(<PaymentPanel order={order("failed", "VALIDATION_MISMATCH")} onRetry={vi.fn()} onDone={vi.fn()} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Contact the store owner before handing over the goods.");
  });
});
