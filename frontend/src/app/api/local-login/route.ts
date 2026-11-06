import { NextResponse } from "next/server";
import { isLocalAccount } from "@/features/auth/accounts";
import { signLocalToken } from "@/features/auth/local-token";

// Hanya untuk pengembangan. Di produksi route ini menjawab 404, dan API produksi memakai JWKS,
// jadi token HS256 buatan lokal tetap ditolak.
export async function POST(request: Request) {
  const secret = process.env.LOCAL_JWT_SECRET;
  const issuer = process.env.LOCAL_JWT_ISSUER;
  if (process.env.NODE_ENV === "production" || !secret || !issuer) {
    return new NextResponse(null, { status: 404 });
  }
  const body: unknown = await request.json().catch(() => null);
  const email = (body as { email?: unknown } | null)?.email;
  if (typeof email !== "string" || !isLocalAccount(email)) {
    return NextResponse.json({ error: "Akun lokal tidak dikenal." }, { status: 400 });
  }
  const token = await signLocalToken({ email, secret, issuer });
  return NextResponse.json({ token });
}
