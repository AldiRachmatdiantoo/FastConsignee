import type { Metadata } from "next";

export const metadata: Metadata = { title: "Page not found" };

export default function NotFound() {
  return (
    <main className="mx-auto grid min-h-dvh max-w-lg content-center justify-items-start gap-3 px-4 py-8">
      <p className="font-semibold text-accent-foreground">FastConsignee</p>
      <h1 className="text-2xl font-bold tracking-tight">Page not found</h1>
      <p className="text-muted-foreground">The address may be wrong, or the link has expired.</p>
      {/* Beranda adalah file statis, jadi tautannya bukan next/link. */}
      <a className="font-medium underline underline-offset-4" href="/">
        Back to home
      </a>
    </main>
  );
}
