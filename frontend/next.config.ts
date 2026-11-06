import type { NextConfig } from "next";

// CSP sengaja tidak memuat script-src, karena wallet menyuntikkan skrip ke halaman.
const securityHeaders = [
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "Content-Security-Policy", value: "frame-ancestors 'none'; base-uri 'self'; object-src 'none'; form-action 'self'" },
];

const nextConfig: NextConfig = {
  // Uji Playwright memakai folder sendiri supaya bisa berjalan di samping `next dev` biasa.
  distDir: process.env.NEXT_DIST_DIR ?? ".next",
  // Landing page berupa satu file statis, jadi skrip animasinya tidak melewati React.
  async rewrites() {
    return [{ source: "/", destination: "/landing.html" }];
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
