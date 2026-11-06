import type { ButtonHTMLAttributes, ReactNode } from "react";

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
  success: "bg-accent text-accent-foreground",
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

export function ComingSoon({ title, text }: { title: string; text: string }) {
  return (
    <section>
      <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
      <p className="mt-2 text-muted-foreground">{text}</p>
    </section>
  );
}
