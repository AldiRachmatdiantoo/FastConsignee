import type { GoTrueClient } from "@supabase/auth-js";

const SUPABASE_URL = process.env.NEXT_PUBLIC_SUPABASE_URL;
const SUPABASE_KEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
// Client ID Google yang sama dengan di provider Google Supabase. Kalau diisi, Google langsung kembali ke situs ini.
const GOOGLE_CLIENT_ID = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;
const STORAGE_KEY = "fastconsignee.supabase";
const PENDING_KEY = "fastconsignee.google";
const FAILED = "Google sign-in failed. Try again.";

export const googleEnabled = Boolean(SUPABASE_URL && SUPABASE_KEY);

// Perpindahan ke Google dibungkus supaya uji bisa membaca alamat tujuannya. jsdom tidak mengizinkan location.assign diganti.
export const navigation = { assign: (url: string) => location.assign(url) };

let client: Promise<GoTrueClient> | null = null;

// Paket auth diunduh saat dipakai, supaya pembeli di layar bayar tidak ikut memuatnya.
function auth(): Promise<GoTrueClient> {
  client ??= import("@supabase/auth-js").then(
    ({ AuthClient }) =>
      new AuthClient({
        url: `${SUPABASE_URL}/auth/v1`,
        headers: { apikey: SUPABASE_KEY ?? "" },
        storageKey: STORAGE_KEY,
        flowType: "pkce",
        detectSessionInUrl: true,
        autoRefreshToken: true,
        persistSession: true,
      }),
  );
  return client;
}

function hex(bytes: Uint8Array): string {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

async function sha256(text: string): Promise<string> {
  return hex(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text))));
}

// googleReply membaca balasan Google di fragmen alamat. Fragmen tidak ikut terkirim ke server.
function googleReply(): URLSearchParams | null {
  const reply = new URLSearchParams(location.hash.slice(1));
  return reply.has("state") && (reply.has("id_token") || reply.has("error")) ? reply : null;
}

// hasGoogleSession mengecek sesi tersimpan atau balasan Google tanpa mengunduh paket auth.
export function hasGoogleSession(): boolean {
  if (!googleEnabled) {
    return false;
  }
  try {
    return (
      Boolean(localStorage.getItem(STORAGE_KEY)) || new URLSearchParams(location.search).has("code") || googleReply() !== null
    );
  } catch {
    return false;
  }
}

// finishGoogleReply menukar ID token dari Google dengan sesi Supabase. Supabase memeriksa tanda tangan Google,
// audience Client ID, dan nonce, sedangkan state di sini memastikan balasan ini memang dari permintaan di tab ini.
async function finishGoogleReply(supabase: GoTrueClient): Promise<void> {
  const reply = googleReply();
  if (!reply) {
    return;
  }
  history.replaceState(history.state, "", location.pathname + location.search);
  let pending: { nonce?: string; state?: string } = {};
  try {
    pending = JSON.parse(sessionStorage.getItem(PENDING_KEY) ?? "{}");
    sessionStorage.removeItem(PENDING_KEY);
  } catch {}
  const token = reply.get("id_token");
  if (token && pending.nonce && reply.get("state") === pending.state) {
    await supabase.signInWithIdToken({ provider: "google", token, nonce: pending.nonce });
  }
}

// watchGoogleSession mengirim token terbaru, termasuk setelah diperbarui di latar belakang.
export async function watchGoogleSession(onToken: (token: string | null) => void): Promise<() => void> {
  const supabase = await auth();
  const { data } = supabase.onAuthStateChange((_event, session) => onToken(session?.access_token ?? null));
  try {
    await finishGoogleReply(supabase);
  } catch {}
  const { data: current } = await supabase.getSession();
  onToken(current.session?.access_token ?? null);
  return () => data.subscription.unsubscribe();
}

// Dengan Client ID, pengguna dibawa langsung ke Google dan kembali ke /login di situs ini, jadi layar izin Google
// menyebut alamat situs ini, bukan alamat proyek Supabase. Nonce mentah disimpan di tab ini, Google hanya menerima hash-nya.
async function redirectToGoogle(clientId: string): Promise<void> {
  const nonce = hex(crypto.getRandomValues(new Uint8Array(32)));
  const state = hex(crypto.getRandomValues(new Uint8Array(16)));
  sessionStorage.setItem(PENDING_KEY, JSON.stringify({ nonce, state }));
  const query = new URLSearchParams({
    client_id: clientId,
    redirect_uri: `${location.origin}/login`,
    response_type: "id_token",
    scope: "openid email profile",
    nonce: await sha256(nonce),
    state,
    prompt: "select_account",
  });
  navigation.assign(`https://accounts.google.com/o/oauth2/v2/auth?${query}`);
}

// signInWithGoogle membawa pengguna ke Google. Pilihan akun selalu tampil, karena satu HP bisa dipakai bergantian.
export async function signInWithGoogle(): Promise<string | null> {
  try {
    if (GOOGLE_CLIENT_ID) {
      await redirectToGoogle(GOOGLE_CLIENT_ID);
      return null;
    }
    const supabase = await auth();
    const { error } = await supabase.signInWithOAuth({
      provider: "google",
      options: { redirectTo: `${location.origin}/login`, queryParams: { prompt: "select_account" } },
    });
    return error ? FAILED : null;
  } catch {
    return "Google sign-in failed to load. Reload this page.";
  }
}

export async function signOutGoogle(): Promise<void> {
  if (!hasGoogleSession()) {
    return;
  }
  try {
    const supabase = await auth();
    await supabase.signOut({ scope: "local" });
  } catch {}
}
