import bs58 from "bs58";
import { describe, expect, it, vi } from "vitest";
import { ApiError, ApiUnavailableError, createApiClient, type Problem } from "@/lib/api/client";
import { linkErrorMessage, linkWallet, shortAddress } from "./link-wallet";

const address = "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a";
const signature = new Uint8Array(64).fill(7);

function json(body: unknown, status: number) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function fakeApi(link: () => Response) {
  const requests: { path: string; body: Record<string, string> }[] = [];
  const fetcher = async (request: Request) => {
    const path = new URL(request.url).pathname;
    requests.push({ path, body: await request.clone().json() });
    if (path.endsWith("/nonce")) {
      return json({ nonce: "abc123", message: "pesan untuk ditandatangani", expires_at: "2026-11-07T00:05:00Z" }, 201);
    }
    return link();
  };
  const api = createApiClient({
    getToken: () => "token",
    onUnauthorized: vi.fn(),
    baseUrl: "http://api.test",
    fetch: fetcher as unknown as typeof fetch,
  });
  return { api, requests };
}

function problem(code: Problem["code"]): ApiError {
  return new ApiError({ type: "about:blank", title: "Failed", status: 422, code, request_id: "req-1" });
}

describe("linkWallet", () => {
  it("menandatangani pesan dari API apa adanya, lalu mengirim signature base58", async () => {
    const { api, requests } = fakeApi(() => json({ address, linked_at: "2026-11-07T00:00:00Z" }, 200));
    const signMessage = vi.fn<(message: Uint8Array) => Promise<Uint8Array>>(async () => signature);

    const linked = await linkWallet({ api, address, signMessage });

    expect(new TextDecoder().decode(signMessage.mock.calls[0][0])).toBe("pesan untuk ditandatangani");
    expect(requests[0]).toEqual({ path: "/api/v1/wallet/nonce", body: { address } });
    expect(requests[1]).toEqual({
      path: "/api/v1/wallet/link",
      body: { address, nonce: "abc123", signature: bs58.encode(signature) },
    });
    expect(linked.address).toBe(address);
  });

  it("tidak mengirim apa pun kalau wallet menolak menandatangani", async () => {
    const { api, requests } = fakeApi(() => json({}, 200));
    const signMessage = vi.fn(async () => {
      throw new Error("User rejected the request.");
    });

    await expect(linkWallet({ api, address, signMessage })).rejects.toThrow("User rejected");
    expect(requests.map((request) => request.path)).toEqual(["/api/v1/wallet/nonce"]);
  });

  it("meneruskan penolakan API sebagai ApiError", async () => {
    const { api } = fakeApi(() =>
      json({ title: "Failed", status: 422, code: "SIGNATURE_INVALID", request_id: "req-1" }, 422),
    );

    const error = await linkWallet({ api, address, signMessage: async () => signature }).catch((e) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.code).toBe("SIGNATURE_INVALID");
  });
});

describe("linkErrorMessage", () => {
  it("memberi langkah berikutnya untuk tiap kode", () => {
    expect(linkErrorMessage(problem("NONCE_INVALID"))).toContain("Try signing again");
    expect(linkErrorMessage(problem("SIGNATURE_INVALID"))).toContain("active wallet account");
    expect(linkErrorMessage(problem("WALLET_ALREADY_LINKED"))).toContain("another wallet");
  });

  it("memakai pesan API untuk kode lain", () => {
    expect(linkErrorMessage(problem("RATE_LIMITED"))).toBe("Failed");
  });

  it("membedakan API mati dari penolakan wallet", () => {
    expect(linkErrorMessage(new ApiUnavailableError())).toContain("The API cannot be reached");
    expect(linkErrorMessage(new Error("User rejected"))).toContain("No balance changed");
  });
});

describe("shortAddress", () => {
  it("memendekkan alamat", () => {
    expect(shortAddress(address)).toBe("4K1Qdt...ooDU5a");
  });
});
