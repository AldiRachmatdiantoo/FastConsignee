import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const signInWithOAuth = vi.fn();
const signOut = vi.fn();
const getSession = vi.fn();
const unsubscribe = vi.fn();
let authChange: (event: string, session: { access_token: string } | null) => void = () => {};
const constructed = vi.fn();

vi.mock("@supabase/auth-js", () => ({
  AuthClient: class {
    constructor(options: unknown) {
      constructed(options);
    }
    signInWithOAuth = signInWithOAuth;
    signOut = signOut;
    getSession = getSession;
    onAuthStateChange(callback: typeof authChange) {
      authChange = callback;
      return { data: { subscription: { unsubscribe } } };
    }
  },
}));

async function loadGoogle() {
  vi.resetModules();
  return import("./google");
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://proyek.supabase.co");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "kunci-publik");
});

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("login Google", () => {
  it("mati kalau Supabase belum diatur", async () => {
    vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "");
    const google = await loadGoogle();

    expect(google.googleEnabled).toBe(false);
    expect(google.hasGoogleSession()).toBe(false);
  });

  it("memakai alur PKCE dan kembali ke halaman login", async () => {
    signInWithOAuth.mockResolvedValue({ error: null });
    const google = await loadGoogle();

    await expect(google.signInWithGoogle()).resolves.toBeNull();

    expect(constructed).toHaveBeenCalledWith(
      expect.objectContaining({ url: "https://proyek.supabase.co/auth/v1", flowType: "pkce", detectSessionInUrl: true }),
    );
    expect(signInWithOAuth).toHaveBeenCalledWith({
      provider: "google",
      options: { redirectTo: `${location.origin}/login`, queryParams: { prompt: "select_account" } },
    });
  });

  it("memberi pesan kalau Google gagal", async () => {
    signInWithOAuth.mockResolvedValue({ error: new Error("ditolak") });
    const google = await loadGoogle();

    await expect(google.signInWithGoogle()).resolves.toBe("Login Google gagal. Coba lagi.");
  });

  it("mengirim token awal dan token yang diperbarui", async () => {
    getSession.mockResolvedValue({ data: { session: { access_token: "token-1" } } });
    const google = await loadGoogle();
    const onToken = vi.fn();

    const stop = await google.watchGoogleSession(onToken);
    expect(onToken).toHaveBeenLastCalledWith("token-1");

    authChange("TOKEN_REFRESHED", { access_token: "token-2" });
    expect(onToken).toHaveBeenLastCalledWith("token-2");

    authChange("SIGNED_OUT", null);
    expect(onToken).toHaveBeenLastCalledWith(null);

    stop();
    expect(unsubscribe).toHaveBeenCalled();
  });

  it("tidak mengunduh paket auth saat keluar tanpa sesi Google", async () => {
    const google = await loadGoogle();

    await google.signOutGoogle();

    expect(constructed).not.toHaveBeenCalled();
  });

  it("keluar dari sesi Google yang tersimpan", async () => {
    localStorage.setItem("fastconsignee.supabase", "{}");
    const google = await loadGoogle();

    await google.signOutGoogle();

    expect(signOut).toHaveBeenCalledWith({ scope: "local" });
  });
});
