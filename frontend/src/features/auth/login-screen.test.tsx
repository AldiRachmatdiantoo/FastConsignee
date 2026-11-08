import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginScreen } from "./login-screen";
import { SessionProvider } from "./session";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

const cashier = {
  id: "1",
  email: "kasir@fastconsignee.test",
  name: "Demo Cashier",
  role: "cashier",
  store: { id: "2", name: "Toko Titip Nusantara" },
  consignor_id: null,
  wallet: null,
  wallet_required: false,
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

type Handler = (url: string, request: Request) => Response;

function stubFetch(handler: Handler) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: Request | string, init?: RequestInit) => {
      const request = input instanceof Request ? input : new Request(new URL(input, "http://localhost:3000"), init);
      return handler(request.url, request);
    }),
  );
}

function show() {
  render(
    <SessionProvider>
      <LoginScreen />
    </SessionProvider>,
  );
}

describe("LoginScreen", () => {
  beforeEach(() => {
    replace.mockClear();
    vi.stubEnv("NEXT_PUBLIC_LOCAL_LOGIN", "true");
  });

  it("memasukkan akun lokal lalu membuka layar sesuai peran", async () => {
    stubFetch((url) => (url.endsWith("/api/local-login") ? json({ token: "token-lokal" }) : json(cashier)));
    show();
    await userEvent.click(await screen.findByRole("button", { name: /Demo Cashier/ }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/kasir"));
    expect(sessionStorage.getItem("fastconsignee.token")).toBe("token-lokal");
  });

  it("memulihkan sesi dari token tersimpan", async () => {
    sessionStorage.setItem("fastconsignee.token", "token-lama");
    stubFetch(() => json(cashier));
    show();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/kasir"));
  });

  it("menghapus token yang ditolak API", async () => {
    sessionStorage.setItem("fastconsignee.token", "token-basi");
    stubFetch(() =>
      json({ title: "Belum login", status: 401, code: "UNAUTHENTICATED", request_id: "req-1" }, 401),
    );
    show();
    expect(await screen.findByRole("alert")).toHaveTextContent("Your session ended");
    expect(sessionStorage.getItem("fastconsignee.token")).toBeNull();
    expect(replace).not.toHaveBeenCalled();
  });

  it("menampilkan alasan API saat akun belum terdaftar", async () => {
    const detail = "Akun ini belum terdaftar di toko mana pun.";
    stubFetch((url) =>
      url.endsWith("/api/local-login")
        ? json({ token: "token-lokal" })
        : json({ title: "Tidak boleh", detail, status: 403, code: "ROLE_NOT_ASSIGNED", request_id: "req-2" }, 403),
    );
    show();
    await userEvent.click(await screen.findByRole("button", { name: /Budi Kopi/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent(detail);
    expect(sessionStorage.getItem("fastconsignee.token")).toBeNull();
  });

  it("memberi tahu kalau login lokal belum dikonfigurasi", async () => {
    stubFetch(() => new Response(null, { status: 404 }));
    show();
    await userEvent.click(await screen.findByRole("button", { name: /Demo Cashier/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent("LOCAL_JWT_SECRET");
  });

  it("memberi tahu kalau API mati", async () => {
    sessionStorage.setItem("fastconsignee.token", "token-lama");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch");
      }),
    );
    show();
    expect(await screen.findByRole("alert")).toHaveTextContent("The API cannot be reached");
  });

  it("menyembunyikan akun lokal di luar mode lokal", async () => {
    vi.stubEnv("NEXT_PUBLIC_LOCAL_LOGIN", "");
    stubFetch(() => json(cashier));
    show();
    expect(await screen.findByText("Google sign-in is not connected yet.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Demo Cashier/ })).not.toBeInTheDocument();
  });

  it("menonaktifkan tombol Google selama belum tersambung", async () => {
    stubFetch(() => json(cashier));
    show();
    expect(await screen.findByRole("button", { name: "Sign in with Google" })).toBeDisabled();
  });

  it("menyediakan jalan kembali ke landing page", async () => {
    stubFetch(() => json(cashier));
    show();
    expect(await screen.findByRole("link", { name: "Back to home" })).toHaveAttribute("href", "/");
  });
});
