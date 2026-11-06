import { describe, expect, it, vi } from "vitest";
import { ApiError, ApiUnavailableError, createApiClient, unwrap } from "./client";

const problem = {
  type: "about:blank",
  title: "Tidak boleh",
  status: 403,
  code: "ROLE_NOT_ASSIGNED",
  detail: "Akun ini belum terdaftar di toko mana pun.",
  request_id: "req-1",
};

function json(body: unknown, status: number, type = "application/json") {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": type } });
}

function client(fetcher: typeof fetch, token: string | null = "token-kasir", onUnauthorized = vi.fn()) {
  return createApiClient({ getToken: () => token, onUnauthorized, baseUrl: "http://api.test", fetch: fetcher });
}

describe("createApiClient", () => {
  it("mengirim token di header Authorization", async () => {
    const fetcher = vi.fn<(request: Request) => Promise<Response>>(async () => json({ id: "1" }, 200));
    await client(fetcher as unknown as typeof fetch).GET("/api/v1/me");
    expect(fetcher.mock.calls[0][0].headers.get("Authorization")).toBe("Bearer token-kasir");
  });

  it("tidak mengirim header Authorization tanpa token", async () => {
    const fetcher = vi.fn<(request: Request) => Promise<Response>>(async () => json({ status: "ok" }, 200));
    await client(fetcher as unknown as typeof fetch, null).GET("/healthz");
    expect(fetcher.mock.calls[0][0].headers.has("Authorization")).toBe(false);
  });

  it("memberi tahu sesi saat API menjawab 401", async () => {
    const onUnauthorized = vi.fn();
    const fetcher = vi.fn(async () => json({ ...problem, status: 401, code: "UNAUTHENTICATED" }, 401));
    await client(fetcher as unknown as typeof fetch, "kedaluwarsa", onUnauthorized).GET("/api/v1/me");
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });
});

describe("unwrap", () => {
  it("mengembalikan isi jawaban", async () => {
    const fetcher = vi.fn(async () => json({ name: "Kasir Demo" }, 200));
    const me = await unwrap(client(fetcher as unknown as typeof fetch).GET("/api/v1/me"));
    expect(me.name).toBe("Kasir Demo");
  });

  it("mengubah Problem menjadi ApiError dengan kode dari API", async () => {
    const fetcher = vi.fn(async () => json(problem, 403, "application/problem+json"));
    const error = await unwrap(client(fetcher as unknown as typeof fetch).GET("/api/v1/me")).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.code).toBe("ROLE_NOT_ASSIGNED");
    expect(error.message).toBe(problem.detail);
  });

  it("menganggap jaringan putus sebagai API tidak tersedia", async () => {
    const fetcher = vi.fn(async () => {
      throw new TypeError("Failed to fetch");
    });
    const error = await unwrap(client(fetcher as unknown as typeof fetch).GET("/api/v1/me")).catch((e) => e);
    expect(error).toBeInstanceOf(ApiUnavailableError);
  });

  it("menganggap jawaban tanpa Problem sebagai API tidak tersedia", async () => {
    const fetcher = vi.fn(async () => new Response("Bad Gateway", { status: 502 }));
    const error = await unwrap(client(fetcher as unknown as typeof fetch).GET("/api/v1/me")).catch((e) => e);
    expect(error).toBeInstanceOf(ApiUnavailableError);
  });
});
