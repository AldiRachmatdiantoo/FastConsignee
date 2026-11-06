import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary";
};

const BUTTON_STYLES = {
  primary: "bg-primary text-primary-foreground hover:bg-primary/90",
  secondary: "border border-border bg-card text-foreground hover:bg-muted",
};

export function Button({ variant = "primary", className = "", type = "button", ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={`inline-flex min-h-11 items-center justify-center rounded-lg px-4 font-semibold focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-60 ${BUTTON_STYLES[variant]} ${className}`}
      {...props}
    />
  );
}

const NOTICE_STYLES = {
  danger: "bg-destructive/10 text-destructive",
  success: "bg-success/10 text-success",
  info: "bg-accent text-accent-foreground",
};

export function Notice({ tone, children }: { tone: keyof typeof NOTICE_STYLES; children: ReactNode }) {
  return (
    <p role={tone === "danger" ? "alert" : "status"} className={`rounded-lg px-3 py-2 text-sm ${NOTICE_STYLES[tone]}`}>
      {children}
    </p>
  );
}

export function Loading() {
  return (
    <p role="status" className="grid min-h-dvh place-items-center text-muted-foreground">
      Memuat...
    </p>
  );
}

export function InlineLoading() {
  return (
    <p role="status" className="py-6 text-center text-sm text-muted-foreground">
      Memuat...
    </p>
  );
}

type CardProps = { className?: string; flush?: boolean; children: ReactNode };

// Kartu `flush` tidak punya padding, supaya garis pemisah di dalamnya bisa selebar kartu.
export function Card({ className = "", flush = false, children }: CardProps) {
  return (
    <section className={`rounded-2xl border border-border/60 bg-card shadow-md ${flush ? "" : "p-5"} ${className}`}>
      {children}
    </section>
  );
}

export function PageTitle({ title, text }: { title: string; text?: string }) {
  return (
    <header>
      <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
      {text ? <p className="mt-2 text-muted-foreground">{text}</p> : null}
    </header>
  );
}

export function ExplorerLink({ href, children = "Lihat di Solana Explorer" }: { href: string; children?: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="font-semibold text-accent-foreground underline underline-offset-4"
    >
      {children}
    </a>
  );
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-sm text-muted-foreground">{children}</p>;
}

export function LoadMore({ visible, busy, onClick }: { visible: boolean; busy: boolean; onClick: () => void }) {
  if (!visible) {
    return null;
  }
  return (
    <div className="mt-4 text-center">
      <Button variant="secondary" disabled={busy} onClick={onClick}>
        {busy ? "Memuat..." : "Muat lagi"}
      </Button>
    </div>
  );
}

const FIELD_STYLES =
  "mt-1 min-h-11 w-full rounded-lg border border-input bg-background px-3 text-foreground focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring disabled:opacity-60";

export function TextField({ label, ...props }: InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <input className={FIELD_STYLES} {...props} />
    </label>
  );
}

export function SelectField({
  label,
  children,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement> & { label: string; children: ReactNode }) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <select className={FIELD_STYLES} {...props}>
        {children}
      </select>
    </label>
  );
}
