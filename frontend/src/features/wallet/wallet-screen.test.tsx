import { WalletReadyState } from "@solana/wallet-adapter-base";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, type Me } from "@/lib/api/client";
import { linkWallet } from "./link-wallet";
import { WalletScreen } from "./wallet-screen";

const address = "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a";
const replace = vi.fn();
const refresh = vi.fn(async () => {});
const select = vi.fn();
const disconnect = vi.fn(async () => {});
const signMessage = vi.fn(async () => new Uint8Array(64));

let me: Me;
let wallet: Record<string, unknown>;

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

vi.mock("@/features/auth/session", () => ({
  useSession: () => ({ session: { status: "authenticated", me }, api: {}, refresh }),
}));

vi.mock("./link-wallet", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./link-wallet")>()),
  linkWallet: vi.fn(),
}));

vi.mock("@solana/wallet-adapter-react", () => ({
  ConnectionProvider: ({ children }: { children: ReactNode }) => children,
  WalletProvider: ({ children }: { children: ReactNode }) => children,
  useWallet: () => wallet,
}));

function installed(name: string) {
  return { adapter: { name }, readyState: WalletReadyState.Installed };
}

function connected() {
  return { toBase58: () => address };
}

describe("WalletScreen", () => {
  beforeEach(() => {
    vi.mocked(linkWallet).mockReset();
    replace.mockClear();
    refresh.mockClear();
    select.mockClear();
    me = {
      id: "1",
      email: "pemilik@fastconsignee.test",
      name: "Demo Owner",
      role: "store_owner",
      store: { id: "2", name: "Shop" },
      consignor_id: null,
      wallet: null,
      wallet_required: true,
    };
    wallet = { wallets: [], select, publicKey: null, signMessage, disconnect, connecting: false };
  });

  it("mengarahkan ke Phantom kalau browser belum punya wallet", () => {
    wallet.wallets = [{ adapter: { name: "Lain" }, readyState: WalletReadyState.NotDetected }];
    render(<WalletScreen />);
    expect(screen.getByRole("link", { name: "Install Phantom" })).toHaveAttribute("href", "https://phantom.com/download");
  });

  it("memilih wallet yang terpasang", async () => {
    wallet.wallets = [installed("Phantom")];
    render(<WalletScreen />);
    await userEvent.click(screen.getByRole("button", { name: "Phantom" }));
    expect(select).toHaveBeenCalledWith("Phantom");
  });

  it("menjelaskan kegunaan wallet sesuai peran", () => {
    me.role = "consignor";
    render(<WalletScreen />);
    expect(screen.getByText(/receives your share of each sale/)).toBeInTheDocument();
  });

  it("menautkan wallet lalu memuat ulang profil", async () => {
    wallet.publicKey = connected();
    vi.mocked(linkWallet).mockResolvedValue({ address, linked_at: "2026-11-07T00:00:00Z" });
    render(<WalletScreen />);

    await userEvent.click(screen.getByRole("button", { name: "Sign and link" }));

    expect(linkWallet).toHaveBeenCalledWith(expect.objectContaining({ address, signMessage }));
    await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
  });

  it("menampilkan langkah berikutnya saat API menolak, dan boleh mencoba lagi", async () => {
    wallet.publicKey = connected();
    vi.mocked(linkWallet).mockRejectedValue(
      new ApiError({ type: "about:blank", title: "Konflik", status: 409, code: "WALLET_ALREADY_LINKED", request_id: "r" }),
    );
    render(<WalletScreen />);

    await userEvent.click(screen.getByRole("button", { name: "Sign and link" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("already linked to someone else");
    expect(refresh).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Sign and link" })).toBeEnabled();
  });

  it("memberi tahu kalau wallet tidak mendukung tanda tangan pesan", async () => {
    wallet.publicKey = connected();
    wallet.signMessage = undefined;
    render(<WalletScreen />);

    await userEvent.click(screen.getByRole("button", { name: "Sign and link" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("cannot sign messages");
    expect(linkWallet).not.toHaveBeenCalled();
  });

  it("melanjutkan ke dasbor setelah wallet yang sama tertaut", async () => {
    wallet.publicKey = connected();
    me.wallet = { address, linked_at: "2026-11-07T00:00:00Z" };
    render(<WalletScreen />);

    expect(screen.getByRole("status")).toHaveTextContent("Linked wallet");
    await userEvent.click(screen.getByRole("button", { name: "Continue to dashboard" }));
    expect(replace).toHaveBeenCalledWith("/dasbor");
  });

  it("boleh mengganti wallet yang tertaut dengan wallet lain", () => {
    wallet.publicKey = { toBase58: () => "7Pxwcgh7QWWfjLrvbdKHm5ssob54s8ynyKHJ1tFz8zYk" };
    me.wallet = { address, linked_at: "2026-11-07T00:00:00Z" };
    render(<WalletScreen />);
    expect(screen.getByRole("button", { name: "Sign and link" })).toBeInTheDocument();
  });
});
