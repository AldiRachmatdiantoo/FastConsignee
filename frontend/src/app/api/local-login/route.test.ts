// @vitest-environment node
import { jwtVerify } from "jose";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { POST } from "./route";

const secret = "rahasia-lokal-untuk-test-minimal-32-karakter";
const issuer = "fastconsignee-local";

function login(body: unknown) {
  return POST(
    new Request("http://localhost:3000/api/local-login", {
      method: "POST",
      body: typeof body === "string" ? body : JSON.stringify(body),
    }),
  );
}

describe("POST /api/local-login", () => {
  beforeEach(() => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("LOCAL_JWT_SECRET", secret);
    vi.stubEnv("LOCAL_JWT_ISSUER", issuer);
  });

  it("membuat JWT dengan klaim yang dibaca backend", async () => {
    const response = await login({ email: "kasir@fastconsignee.test" });
    expect(response.status).toBe(200);
    const { token } = await response.json();
    const { payload, protectedHeader } = await jwtVerify(token, new TextEncoder().encode(secret), {
      issuer,
      audience: "authenticated",
      algorithms: ["HS256"],
    });
    expect(protectedHeader.alg).toBe("HS256");
    expect(payload.sub).toBe("local:kasir@fastconsignee.test");
    expect(payload.email).toBe("kasir@fastconsignee.test");
    expect(payload.user_metadata).toEqual({ email_verified: true });
    expect(payload.exp).toBeGreaterThan(payload.iat as number);
  });

  it("tidak ada di produksi", async () => {
    vi.stubEnv("NODE_ENV", "production");
    const response = await login({ email: "kasir@fastconsignee.test" });
    expect(response.status).toBe(404);
  });

  it("tidak aktif tanpa secret", async () => {
    vi.stubEnv("LOCAL_JWT_SECRET", "");
    const response = await login({ email: "kasir@fastconsignee.test" });
    expect(response.status).toBe(404);
  });

  it("menolak akun di luar daftar", async () => {
    const response = await login({ email: "orang-lain@gmail.com" });
    expect(response.status).toBe(400);
  });

  it("menolak isi yang bukan JSON", async () => {
    const response = await login("bukan json");
    expect(response.status).toBe(400);
  });
});
