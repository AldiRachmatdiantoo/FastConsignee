import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { ok } from "@/test/api";
import { BayarScreen } from "./bayar-screen";

const api = {
  GET: () =>
    ok({
      id: "o1",
      status: "pending",
      store_name: "Toko Titip Nusantara",
      lines: [{ item_name: "Dompet Pandan", quantity: 1, line_total_idr: 45000, certificate: null }],
      total_idr: 45000,
      total_lamports: "1000000000",
      rate: { idr_per_sol: 45000 },
      expires_at: new Date(Date.now() + 120_000).toISOString(),
      solana_pay_url: "solana:https://api.test/pay",
      confirmed_at: null,
      explorer_url: null,
    }),
};

vi.mock("@/features/auth/session", () => ({ useSession: () => ({ api }) }));
vi.mock("./wallet-payment", () => {
  throw new Error("berkas gagal dimuat");
});

it("tetap menampilkan QR kalau berkas wallet gagal dimuat", async () => {
  render(<BayarScreen orderId="o1" />);

  await userEvent.click(await screen.findByRole("button", { name: "Pay with wallet" }));
  expect(await screen.findByText(/The wallet could not load/)).toBeVisible();
  expect(screen.getByTitle("Solana Pay payment QR")).toBeInTheDocument();
});
