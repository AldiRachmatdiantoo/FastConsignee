import { beforeEach, describe, expect, it, vi } from "vitest";
import { decodeTransaction } from "@/features/wallet/transaction";
import type { Api } from "@/lib/api/client";
import { PayError, payErrorMessage, requestTransaction } from "./pay";

vi.mock("@/features/wallet/transaction", () => ({ decodeTransaction: vi.fn() }));

const post = vi.fn();
const api = { POST: post } as unknown as Api;

describe("requestTransaction", () => {
  beforeEach(() => {
    post.mockReset();
    vi.mocked(decodeTransaction).mockReset();
  });

  it("meminta transaksi yang sama dengan QR untuk wallet pembeli", async () => {
    const transaction = {} as ReturnType<typeof decodeTransaction>;
    vi.mocked(decodeTransaction).mockReturnValue(transaction);
    post.mockResolvedValue({ data: { transaction: "dHg=" } });

    await expect(requestTransaction(api, "o1", "wallet")).resolves.toBe(transaction);
    expect(post).toHaveBeenCalledWith("/api/v1/solana-pay/orders/{order_id}", {
      params: { path: { order_id: "o1" } },
      body: { account: "wallet" },
    });
    expect(decodeTransaction).toHaveBeenCalledWith("dHg=");
  });

  it("memakai pesan endpoint Solana Pay", async () => {
    post.mockResolvedValue({ error: { message: "Order sudah kedaluwarsa." } });
    await expect(requestTransaction(api, "o1", "wallet")).rejects.toThrow("Order sudah kedaluwarsa.");
  });

  it("punya pesan cadangan kalau jawaban tidak berisi pesan", async () => {
    post.mockResolvedValue({ error: {} });
    await expect(requestTransaction(api, "o1", "wallet")).rejects.toThrow("Transaksi tidak bisa disusun. Coba lagi.");
  });

  it("memberi tahu kalau API tidak bisa dihubungi", async () => {
    post.mockRejectedValue(new TypeError("failed to fetch"));
    await expect(requestTransaction(api, "o1", "wallet")).rejects.toThrow("API tidak bisa dihubungi.");
  });
});

describe("payErrorMessage", () => {
  it("meneruskan pesan PayError", () => {
    expect(payErrorMessage(new PayError("Order sudah kedaluwarsa."))).toBe("Order sudah kedaluwarsa.");
  });

  it("menganggap error lain sebagai penolakan di wallet", () => {
    expect(payErrorMessage(new Error("User rejected"))).toContain("Saldo tidak berubah");
  });
});
