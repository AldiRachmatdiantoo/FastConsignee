import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const signInWithOAuth = vi.fn();
const signInWithIdToken = vi.fn();
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
    signInWithIdToken = signInWithIdToken;
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
  sessionStorage.clear();
  history.replaceState(null, "", "/login");
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

async function sha256(text: string): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

describe("login Google yang kembali langsung ke situs ini", () => {
  beforeEach(() => {
    vi.stubEnv("NEXT_PUBLIC_GOOGLE_CLIENT_ID", "klien.apps.googleusercontent.com");
  });

  it("membawa ke Google dengan alamat balik /login, hash nonce, dan state", async () => {
    const google = await loadGoogle();
    const assign = vi.spyOn(google.navigation, "assign").mockImplementation(() => {});

    await expect(google.signInWithGoogle()).resolves.toBeNull();

    const target = new URL(assign.mock.calls[0][0] as string);
    const pending = JSON.parse(sessionStorage.getItem("fastconsignee.google")!);
    expect(target.origin + target.pathname).toBe("https://accounts.google.com/o/oauth2/v2/auth");
    expect(Object.fromEntries(target.searchParams)).toEqual({
      client_id: "klien.apps.googleusercontent.com",
      redirect_uri: `${location.origin}/login`,
      response_type: "id_token",
      scope: "openid email profile",
      nonce: await sha256(pending.nonce),
      state: pending.state,
      prompt: "select_account",
    });
    expect(pending.nonce).toMatch(/^[0-9a-f]{64}$/);
    expect(signInWithOAuth).not.toHaveBeenCalled();
  });

  it("menukar ID token balasan Google dengan sesi Supabase, lalu menghapus token dari alamat", async () => {
    sessionStorage.setItem("fastconsignee.google", JSON.stringify({ nonce: "nonce-mentah", state: "state-1" }));
    history.replaceState(null, "", "/login#state=state-1&id_token=token-google");
    signInWithIdToken.mockResolvedValue({ error: null });
    getSession.mockResolvedValue({ data: { session: { access_token: "token-supabase" } } });
    const google = await loadGoogle();
    const onToken = vi.fn();

    expect(google.hasGoogleSession()).toBe(true);
    await google.watchGoogleSession(onToken);

    expect(signInWithIdToken).toHaveBeenCalledWith({ provider: "google", token: "token-google", nonce: "nonce-mentah" });
    expect(location.hash).toBe("");
    expect(sessionStorage.getItem("fastconsignee.google")).toBeNull();
    expect(onToken).toHaveBeenLastCalledWith("token-supabase");
  });

  it("menolak balasan Google yang state-nya bukan dari tab ini", async () => {
    sessionStorage.setItem("fastconsignee.google", JSON.stringify({ nonce: "nonce-mentah", state: "state-1" }));
    history.replaceState(null, "", "/login#state=state-lain&id_token=token-google");
    getSession.mockResolvedValue({ data: { session: null } });
    const google = await loadGoogle();
    const onToken = vi.fn();

    await google.watchGoogleSession(onToken);

    expect(signInWithIdToken).not.toHaveBeenCalled();
    expect(location.hash).toBe("");
    expect(onToken).toHaveBeenLastCalledWith(null);
  });

  it("tetap memakai alur Supabase kalau Client ID Google tidak diisi", async () => {
    vi.stubEnv("NEXT_PUBLIC_GOOGLE_CLIENT_ID", "");
    signInWithOAuth.mockResolvedValue({ error: null });
    const google = await loadGoogle();

    await expect(google.signInWithGoogle()).resolves.toBeNull();

    expect(signInWithOAuth).toHaveBeenCalled();
  });
});
