"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { ApiError, ApiUnavailableError, createApiClient, unwrap, type Api, type Me } from "@/lib/api/client";

const STORAGE_KEY = "fastconsignee.token";

export type Session =
  | { status: "loading" }
  | { status: "anonymous"; error?: string }
  | { status: "authenticated"; me: Me };

type SessionContextValue = {
  session: Session;
  api: Api;
  signInLocal: (email: string) => Promise<void>;
  signOut: () => void;
  refresh: () => Promise<void>;
};

const SessionContext = createContext<SessionContextValue | null>(null);

// Tanpa sessionStorage, token tetap hidup di memori sampai halaman dimuat ulang.
function createTokenStore() {
  let token: string | null = null;
  return {
    get: () => token,
    set(next: string | null) {
      token = next;
      try {
        if (next) {
          sessionStorage.setItem(STORAGE_KEY, next);
        } else {
          sessionStorage.removeItem(STORAGE_KEY);
        }
      } catch {}
    },
    restore() {
      try {
        token = sessionStorage.getItem(STORAGE_KEY);
      } catch {}
    },
  };
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session>({ status: "loading" });
  const [tokens] = useState(createTokenStore);

  const clear = useCallback(
    (error?: string) => {
      tokens.set(null);
      setSession({ status: "anonymous", error });
    },
    [tokens],
  );

  const api = useMemo(
    () =>
      createApiClient({
        getToken: tokens.get,
        onUnauthorized: () => clear("Sesi berakhir. Silakan masuk lagi."),
      }),
    [tokens, clear],
  );

  // Mengembalikan null kalau jawaban 401 sudah ditangani onUnauthorized.
  const resolve = useCallback(async (): Promise<Session | null> => {
    if (!tokens.get()) {
      return { status: "anonymous" };
    }
    try {
      const me = await unwrap(api.GET("/api/v1/me"));
      return { status: "authenticated", me };
    } catch (error) {
      if (error instanceof ApiUnavailableError) {
        return { status: "anonymous", error: error.message };
      }
      if (!(error instanceof ApiError)) {
        throw error;
      }
      if (error.status === 401) {
        return null;
      }
      tokens.set(null);
      return { status: "anonymous", error: error.message };
    }
  }, [tokens, api]);

  const load = useCallback(async () => {
    const next = await resolve();
    if (next) {
      setSession(next);
    }
  }, [resolve]);

  useEffect(() => {
    tokens.restore();
    void resolve().then((next) => {
      if (next) {
        setSession(next);
      }
    });
  }, [tokens, resolve]);

  const signInLocal = useCallback(
    async (email: string) => {
      const response = await fetch("/api/local-login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      }).catch(() => null);
      if (!response?.ok) {
        clear("Login lokal belum aktif. Cek LOCAL_JWT_SECRET di .env.local.");
        return;
      }
      const body = (await response.json()) as { token: string };
      tokens.set(body.token);
      await load();
    },
    [tokens, clear, load],
  );

  const value = useMemo(
    () => ({ session, api, signInLocal, signOut: () => clear(), refresh: load }),
    [session, api, signInLocal, clear, load],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionContextValue {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error("useSession harus dipakai di dalam SessionProvider");
  }
  return value;
}
