"use client";

import { useCallback, useEffect, useState } from "react";
import { errorMessage } from "@/lib/api/messages";

export type Resource<T> = { status: "loading" } | { status: "ready"; data: T } | { status: "error"; message: string };

// useResource memuat data saat `load` berubah. Bungkus `load` dengan useCallback.
export function useResource<T>(load: () => Promise<T>) {
  const [state, setState] = useState<Resource<T>>({ status: "loading" });
  const [version, setVersion] = useState(0);

  useEffect(() => {
    let active = true;
    load().then(
      (data) => {
        if (active) {
          setState({ status: "ready", data });
        }
      },
      (error: unknown) => {
        if (active) {
          setState({ status: "error", message: errorMessage(error) });
        }
      },
    );
    return () => {
      active = false;
    };
  }, [load, version]);

  const reload = useCallback(() => setVersion((value) => value + 1), []);
  return { state, reload };
}

type Page<T> = { data: T[]; next_cursor: string | null };

type PagedState<T> = {
  items: T[];
  cursor: string | null;
  status: "loading" | "ready" | "more" | "error";
  message: string | null;
};

// usePagedList mengikuti pagination cursor API. Bungkus `load` dengan useCallback.
export function usePagedList<T>(load: (cursor?: string) => Promise<Page<T>>) {
  const [state, setState] = useState<PagedState<T>>({ items: [], cursor: null, status: "loading", message: null });
  const [version, setVersion] = useState(0);

  useEffect(() => {
    let active = true;
    load().then(
      (page) => {
        if (active) {
          setState({ items: page.data, cursor: page.next_cursor, status: "ready", message: null });
        }
      },
      (error: unknown) => {
        if (active) {
          setState({ items: [], cursor: null, status: "error", message: errorMessage(error) });
        }
      },
    );
    return () => {
      active = false;
    };
  }, [load, version]);

  const loadMore = useCallback(async () => {
    if (!state.cursor) {
      return;
    }
    setState((current) => ({ ...current, status: "more" }));
    try {
      const page = await load(state.cursor);
      setState((current) => ({
        items: [...current.items, ...page.data],
        cursor: page.next_cursor,
        status: "ready",
        message: null,
      }));
    } catch (error) {
      setState((current) => ({ ...current, status: "ready", message: errorMessage(error) }));
    }
  }, [load, state.cursor]);

  const reload = useCallback(() => setVersion((value) => value + 1), []);
  return { ...state, loadMore, reload };
}

// usePolling menjalankan `tick` berulang. Putaran berikutnya baru dijadwalkan setelah yang lama selesai.
export function usePolling(tick: () => Promise<void>, intervalMs: number, active: boolean) {
  useEffect(() => {
    if (!active) {
      return;
    }
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const run = async () => {
      await tick().catch(() => {});
      if (!stopped) {
        timer = setTimeout(() => void run(), intervalMs);
      }
    };
    timer = setTimeout(() => void run(), intervalMs);
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [tick, intervalMs, active]);
}

// useNow memberi waktu sekarang yang diperbarui tiap detik untuk hitung mundur.
export function useNow(active: boolean): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!active) {
      return;
    }
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [active]);

  return now;
}

export function useDebounced<T>(value: T, delayMs: number): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return settled;
}
