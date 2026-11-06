import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { hasGoogleSession, signOutGoogle, watchGoogleSession } from "./google";
import { SessionProvider, useSession } from "./session";

vi.mock("./google", () => ({
  hasGoogleSession: vi.fn(),
  signOutGoogle: vi.fn(),
  watchGoogleSession: vi.fn(),
}));

const stop = vi.fn();
const fetchMock = vi.fn();
let pushToken: (token: string | null) => void = () => {};

const me = { id: "u1", role: "cashier", email: "kasir@toko.id", name: "Kasir", store: { id: "s1", name: "Toko" } };

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function problem(status: number, code: string) {
  return json({ code, status, title: code, detail: "Akun ini belum terdaftar.", request_id: "r1" }, status);
}

function Probe() {
  const { session, refresh, signOut } = useSession();
  return (
    <>
      <p>{session.status}</p>
      <p>{"error" in session ? session.error : ""}</p>
      <button onClick={() => void refresh()}>muat ulang</button>
      <button onClick={signOut}>keluar</button>
    </>
  );
}

function sentToken() {
  const request = fetchMock.mock.lastCall?.[0] as Request;
  return request.headers.get("Authorization");
}

beforeEach(() => {
  vi.clearAllMocks();
  sessionStorage.clear();
  vi.stubGlobal("fetch", fetchMock);
  vi.mocked(hasGoogleSession).mockReturnValue(true);
  vi.mocked(watchGoogleSession).mockImplementation(async (onToken) => {
    pushToken = onToken;
    onToken("token-1");
    return stop;
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("sesi Google", () => {
  it("memakai token Google dan token yang diperbarui", async () => {
    fetchMock.mockImplementation(async () => json(me));
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    expect(await screen.findByText("authenticated")).toBeInTheDocument();
    expect(sentToken()).toBe("Bearer token-1");

    pushToken("token-2");
    await userEvent.click(screen.getByRole("button", { name: "muat ulang" }));
    await waitFor(() => expect(sentToken()).toBe("Bearer token-2"));
  });

  it("melepas sesi Google kalau akun belum terdaftar", async () => {
    fetchMock.mockImplementation(async () => problem(403, "ROLE_NOT_ASSIGNED"));
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    expect(await screen.findByText("anonymous")).toBeInTheDocument();
    expect(signOutGoogle).toHaveBeenCalled();
  });

  it("menyimpan sesi saat API tidak terjangkau", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    expect(await screen.findByText("unavailable")).toBeInTheDocument();
    expect(signOutGoogle).not.toHaveBeenCalled();
  });

  it("melepas sesi Google saat pengguna keluar", async () => {
    fetchMock.mockImplementation(async () => json(me));
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );
    await screen.findByText("authenticated");

    await userEvent.click(screen.getByRole("button", { name: "keluar" }));

    expect(await screen.findByText("anonymous")).toBeInTheDocument();
    expect(signOutGoogle).toHaveBeenCalled();
  });

  it("berhenti mendengar sesi saat komponen dilepas", async () => {
    fetchMock.mockImplementation(async () => json(me));
    const { unmount } = render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );
    await screen.findByText("authenticated");

    unmount();

    expect(stop).toHaveBeenCalled();
  });

  it("tidak mengunduh paket auth tanpa sesi Google", async () => {
    vi.mocked(hasGoogleSession).mockReturnValue(false);
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );

    expect(await screen.findByText("anonymous")).toBeInTheDocument();
    expect(watchGoogleSession).not.toHaveBeenCalled();
  });
});
