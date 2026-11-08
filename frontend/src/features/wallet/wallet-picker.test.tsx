import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { isMobileDevice, phantomBrowseLink, WalletPicker } from "./wallet-picker";

vi.mock("@solana/wallet-adapter-react", () => ({
  useWallet: () => ({ wallets: [], select: vi.fn(), connecting: false }),
}));

const IPHONE = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1";
const WINDOWS = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0";

function asDevice(userAgent: string) {
  vi.spyOn(window.navigator, "userAgent", "get").mockReturnValue(userAgent);
}

describe("phantomBrowseLink", () => {
  it("meng-encode alamat halaman dan asal situs", () => {
    expect(phantomBrowseLink("https://fast-consignee.vercel.app/bayar/o1", "https://fast-consignee.vercel.app")).toBe(
      "https://phantom.com/ul/browse/https%3A%2F%2Ffast-consignee.vercel.app%2Fbayar%2Fo1?ref=https%3A%2F%2Ffast-consignee.vercel.app",
    );
  });
});

describe("isMobileDevice", () => {
  it("mengenali HP dan iPad, bukan laptop", () => {
    expect(isMobileDevice(IPHONE, "iPhone", 5)).toBe(true);
    expect(isMobileDevice("Mozilla/5.0 (Linux; Android 15; Pixel 9) Mobile", "Linux armv8l", 5)).toBe(true);
    expect(isMobileDevice("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "MacIntel", 5)).toBe(true);
    expect(isMobileDevice("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", "MacIntel", 0)).toBe(false);
    expect(isMobileDevice(WINDOWS, "Win32", 0)).toBe(false);
  });
});

describe("WalletPicker tanpa wallet", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("di laptop meminta memasang Phantom lalu memuat ulang", () => {
    asDevice(WINDOWS);
    render(<WalletPicker openInPhantom />);
    expect(screen.getByText("No wallet in this browser yet")).toBeVisible();
    expect(screen.queryByRole("link", { name: "Open in the Phantom app" })).not.toBeInTheDocument();
  });

  it("di HP layar bayar menawarkan membuka halaman ini di aplikasi Phantom", () => {
    asDevice(IPHONE);
    render(<WalletPicker openInPhantom />);
    expect(screen.queryByText("No wallet in this browser yet")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open in the Phantom app" })).toHaveAttribute(
      "href",
      phantomBrowseLink(window.location.href, window.location.origin),
    );
    expect(screen.getByRole("link", { name: "No Phantom yet? Install it first" })).toHaveAttribute("href", "https://phantom.com/download");
  });

  it("di HP layar yang butuh login mengarahkan ke laptop", () => {
    asDevice(IPHONE);
    render(<WalletPicker />);
    expect(screen.getByText(/Open this page on a laptop with Phantom installed/)).toBeVisible();
    expect(screen.queryByRole("link", { name: "Open in the Phantom app" })).not.toBeInTheDocument();
  });
});
