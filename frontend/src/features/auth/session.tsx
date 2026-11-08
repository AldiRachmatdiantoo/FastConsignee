"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { ApiError, ApiUnavailableError, createApiClient, unwrap, type Api, type Me } from "@/lib/api/client";
import { hasGoogleSession, signOutGoogle, watchGoogleSession } from "./google";

const STORAGE_KEY = "fastconsignee.token";

export type Session =
  | { status: "loading" }
  | { status: "anonymous"; error?: string }
  | { status: "unavailable"; error: string }
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

  const dropToken = useCallback(() => {
    tokens.set(null);
    void signOutGoogle();
  }, [tokens]);

  const clear = useCallback(
    (error?: string) => {
      dropToken();
      setSession({ status: "anonymous", error });
    },
    [dropToken],
  );

  const api = useMemo(
    () =>
      createApiClient({
        getToken: tokens.get,
        onUnauthorized: () => clear("Your session ended. Please sign in again."),
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
        return { status: "unavailable", error: error.message };
      }
      if (!(error instanceof ApiError)) {
        throw error;
      }
      if (error.status === 401) {
        return null;
      }
      dropToken();
      return { status: "anonymous", error: error.message };
    }
  }, [tokens, api, dropToken]);

  const load = useCallback(async () => {
    const next = await resolve();
    if (next) {
      setSession(next);
    }
  }, [resolve]);

  useEffect(() => {
    let stop = () => {};
    let left = false;

    async function start() {
      tokens.restore();
      if (hasGoogleSession()) {
        const unsubscribe = await watchGoogleSession(tokens.set);
        if (left) {
          unsubscribe();
          return;
        }
        stop = unsubscribe;
      }
      const next = await resolve();
      if (next && !left) {
        setSession(next);
      }
    }

    void start();
    return () => {
      left = true;
      stop();
    };
  }, [tokens, resolve]);

  const signInLocal = useCallback(
    async (email: string) => {
      const response = await fetch("/api/local-login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      }).catch(() => null);
      if (!response?.ok) {
        clear("Local sign-in is not active. Check LOCAL_JWT_SECRET in .env.local.");
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
    throw new Error("useSession must be used inside SessionProvider");
  }
  return value;
}
