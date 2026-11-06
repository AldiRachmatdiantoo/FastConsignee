import { afterEach, describe, expect, it, vi } from "vitest";
import { readBalances, RPC_URL } from "./rpc";

const fetchMock = vi.fn();

function answer(body: unknown, status = 200) {
  fetchMock.mockResolvedValue(new Response(JSON.stringify(body), { status }));
  vi.stubGlobal("fetch", fetchMock);
}

afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
});

describe("readBalances", () => {
  it("membaca semua saldo dalam satu panggilan", async () => {
    answer({ result: { value: [{ lamports: 1500 }, { lamports: 0 }] } });

    await expect(readBalances(["a", "b"])).resolves.toEqual({ a: BigInt(1500), b: BigInt(0) });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const body = JSON.parse(String(init.body));
    expect(url).toBe(RPC_URL);
    expect(body.method).toBe("getMultipleAccounts");
    expect(body.params[0]).toEqual(["a", "b"]);
  });

  it("menganggap wallet yang belum ada di jaringan bersaldo nol", async () => {
    answer({ result: { value: [null] } });

    await expect(readBalances(["baru"])).resolves.toEqual({ baru: BigInt(0) });
  });

  it("gagal kalau RPC menjawab error", async () => {
    answer({ error: { message: "Too many requests" } }, 429);

    await expect(readBalances(["a"])).rejects.toThrow("Too many requests");
  });
});
