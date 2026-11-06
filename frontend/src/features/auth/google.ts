import type { GoTrueClient } from "@supabase/auth-js";

const SUPABASE_URL = process.env.NEXT_PUBLIC_SUPABASE_URL;
const SUPABASE_KEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
const STORAGE_KEY = "fastconsignee.supabase";

export const googleEnabled = Boolean(SUPABASE_URL && SUPABASE_KEY);

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

// hasGoogleSession mengecek sesi tersimpan atau kode balasan Google tanpa mengunduh paket auth.
export function hasGoogleSession(): boolean {
  if (!googleEnabled) {
    return false;
  }
  try {
    return Boolean(localStorage.getItem(STORAGE_KEY)) || new URLSearchParams(location.search).has("code");
  } catch {
    return false;
  }
}

// watchGoogleSession mengirim token terbaru, termasuk setelah diperbarui di latar belakang.
export async function watchGoogleSession(onToken: (token: string | null) => void): Promise<() => void> {
  const supabase = await auth();
  const { data } = supabase.onAuthStateChange((_event, session) => onToken(session?.access_token ?? null));
  const { data: current } = await supabase.getSession();
  onToken(current.session?.access_token ?? null);
  return () => data.subscription.unsubscribe();
}

// signInWithGoogle membawa pengguna ke Google. Pilihan akun selalu tampil, karena satu HP bisa dipakai bergantian.
export async function signInWithGoogle(): Promise<string | null> {
  try {
    const supabase = await auth();
    const { error } = await supabase.signInWithOAuth({
      provider: "google",
      options: { redirectTo: `${location.origin}/login`, queryParams: { prompt: "select_account" } },
    });
    return error ? "Login Google gagal. Coba lagi." : null;
  } catch {
    return "Login Google gagal dimuat. Muat ulang halaman ini.";
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
