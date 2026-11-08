"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Notice } from "@/components/ui";
import { LOCAL_ACCOUNTS } from "./accounts";
import { googleEnabled, signInWithGoogle } from "./google";
import { homePath } from "./routes";
import { useSession } from "./session";
import "./login.css";

const POINTS = [
  "Running on Solana Devnet",
  "One transaction for every share",
  "Every sale verifiable on Explorer",
];

function Caption({ amber = false, children }: { amber?: boolean; children: string }) {
  return (
    <p className={amber ? "login__caption login__caption--amber" : "login__caption"}>
      <span className="login__wave">
        <i />
        <i />
        <i />
        <i />
      </span>
      {children}
    </p>
  );
}

function Scene() {
  return (
    <>
      <div className="login__media" aria-hidden="true">
        <picture>
          <source
            type="image/avif"
            srcSet="/landing/hero-800.avif 800w, /landing/hero-1200.avif 1200w, /landing/hero-1600.avif 1600w"
            sizes="(max-width: 63.99rem) 100vw, 84vw"
          />
          <img src="/landing/hero-1200.webp" alt="" fetchPriority="high" />
        </picture>
      </div>
      <div className="login__shade" aria-hidden="true" />
      <div className="login__talk" aria-hidden="true">
        <svg className="login__lines">
          <ellipse className="login__arc login__arc--a" pathLength={1} />
          <ellipse className="login__arc login__arc--b" pathLength={1} />
          <ellipse className="login__flow login__flow--a" pathLength={1} />
          <ellipse className="login__flow login__flow--b" pathLength={1} />
        </svg>
        <div className="login__chip login__chip--1">
          <Caption>The buyer pays once</Caption>
        </div>
        <div className="login__chip login__chip--2">
          <Caption amber>Consignor share sent</Caption>
        </div>
      </div>
    </>
  );
}

export function LoginScreen() {
  const { session, signInLocal } = useSession();
  const router = useRouter();
  const [pending, setPending] = useState<string | null>(null);
  const [googleError, setGoogleError] = useState<string | null>(null);
  const localLogin = process.env.NEXT_PUBLIC_LOCAL_LOGIN === "true";

  useEffect(() => {
    if (session.status === "authenticated") {
      router.replace(homePath(session.me));
    }
  }, [session, router]);

  // Saat di-refresh, browser mengembalikan posisi scroll terakhir, dan di HP yang pendek nav jadi tertinggal di atas layar.
  // Halaman login selalu dibuka dari atas.
  useEffect(() => {
    const previous = history.scrollRestoration;
    history.scrollRestoration = "manual";
    window.scrollTo(0, 0);
    return () => {
      history.scrollRestoration = previous;
    };
  }, []);

  async function choose(email: string) {
    setPending(email);
    await signInLocal(email);
    setPending(null);
  }

  async function chooseGoogle() {
    setPending("google");
    setGoogleError(await signInWithGoogle());
    setPending(null);
  }

  return (
    <main className="login">
      <Scene />

      {/* Tautan biasa, karena `/` adalah file statis lewat rewrite dan tidak bisa dibuka lewat navigasi klien. */}
      <nav className="login__nav" aria-label="Main">
        <div className="login__bar">
          <a className="login__brand" href="/">
            <svg className="login__mark" viewBox="0 0 40 28" aria-hidden="true">
              <path className="login__mark-a" d="M3 19C10 5 17 5 21 14s11 9 16-6" />
              <path className="login__mark-b" d="M3 10c7 13 14 13 18 4s11-10 16 5" />
            </svg>
            FastConsignee
          </a>
          <a className="login__back" href="/">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M19 12H5M11 6l-6 6 6 6" />
            </svg>
            <span className="login__back-text">Back to home</span>
          </a>
        </div>
      </nav>

      <div className="login__body">
        <section className="login__intro">
          <h1 className="login__title">Every sale, split instantly.</h1>
          <div className="login__lower">
            <p className="login__lead">
              A cashier for consignment shops. The buyer pays once, and each consignor’s share and the shop’s commission go out in one Solana transaction.
            </p>
            <ul className="login__points">
              {POINTS.map((point) => (
                <li key={point}>
                  <svg viewBox="0 0 16 16" aria-hidden="true">
                    <path d="M3 8.5l3 3 7-7" />
                  </svg>
                  {point}
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="login__card" aria-labelledby="login-heading">
          <h2 className="login__heading" id="login-heading">
            Sign in
          </h2>
          {"error" in session && session.error ? <Notice tone="danger">{session.error}</Notice> : null}
          <div className="login__group">
            <button
              type="button"
              className="login__google"
              disabled={!googleEnabled || pending !== null}
              onClick={() => void chooseGoogle()}
            >
              {pending === "google" ? "Opening Google..." : "Sign in with Google"}
            </button>
            {googleEnabled ? null : <p className="login__hint">Google sign-in is not connected yet.</p>}
            {googleError ? <Notice tone="danger">{googleError}</Notice> : null}
          </div>
          {localLogin ? (
            <div className="login__local">
              <p className="login__hint">Local mode. Pick a seed account to sign in.</p>
              <ul className="login__accounts">
                {LOCAL_ACCOUNTS.map((account) => (
                  <li key={account.email}>
                    <button
                      type="button"
                      className="login__account"
                      disabled={pending !== null}
                      onClick={() => void choose(account.email)}
                    >
                      <span className="login__account-name">{account.name}</span>
                      <span className="login__account-role">{pending === account.email ? "Signing in..." : account.role}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </section>
      </div>
    </main>
  );
}
