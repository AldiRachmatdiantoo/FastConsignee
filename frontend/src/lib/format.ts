const rupiah = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
const dateTime = new Intl.DateTimeFormat("id-ID", { dateStyle: "medium", timeStyle: "short" });
const dayName = new Intl.DateTimeFormat("id-ID", { weekday: "short", day: "numeric", month: "short" });
const LAMPORTS_PER_SOL = BigInt(1_000_000_000);
const ZERO = BigInt(0);

export function formatRupiah(value: number): string {
  return rupiah.format(value);
}

// Lamport dikirim sebagai string, jadi hitungannya memakai BigInt supaya presisinya utuh.
export function formatSol(lamports: string | bigint): string {
  const value = BigInt(lamports);
  const absolute = value < ZERO ? -value : value;
  const whole = absolute / LAMPORTS_PER_SOL;
  const fraction = (absolute % LAMPORTS_PER_SOL).toString().padStart(9, "0").replace(/0+$/, "");
  return `${value < ZERO ? "-" : ""}${whole}${fraction ? `,${fraction}` : ""} SOL`;
}

export function formatPercent(bps: number): string {
  return `${(bps / 100).toLocaleString("id-ID")}%`;
}

export function formatDateTime(iso: string): string {
  return dateTime.format(new Date(iso));
}

export function formatDay(date: Date): string {
  return dayName.format(date);
}

export function secondsUntil(iso: string, now: number): number {
  return Math.max(0, Math.ceil((new Date(iso).getTime() - now) / 1000));
}

export function formatCountdown(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${String(seconds % 60).padStart(2, "0")}`;
}
