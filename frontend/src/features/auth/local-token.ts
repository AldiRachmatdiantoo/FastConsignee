import { SignJWT } from "jose";

const AUDIENCE = "authenticated";
const LIFETIME = "8h";

type LocalTokenOptions = {
  email: string;
  secret: string;
  issuer: string;
};

// signLocalToken meniru bentuk JWT Supabase supaya backend memprosesnya lewat jalur yang sama.
export function signLocalToken({ email, secret, issuer }: LocalTokenOptions): Promise<string> {
  return new SignJWT({ email, user_metadata: { email_verified: true } })
    .setProtectedHeader({ alg: "HS256", typ: "JWT" })
    .setSubject(`local:${email}`)
    .setIssuer(issuer)
    .setAudience(AUDIENCE)
    .setIssuedAt()
    .setExpirationTime(LIFETIME)
    .sign(new TextEncoder().encode(secret));
}
