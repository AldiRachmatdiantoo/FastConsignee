import type { ReactNode } from "react";
import { ArrowDownIcon, ArrowUpIcon } from "@/components/icons";
import { Card } from "@/components/ui";
import type { Change } from "./stats";

export function Meter({ percent }: { percent: number }) {
  const width = percent > 0 ? Math.max(percent, 2) : 0;
  return (
    <div aria-hidden="true" className="h-1.5 overflow-hidden rounded-full bg-primary/15">
      <div className="h-full rounded-full bg-linear-to-r from-chart-2 to-primary" style={{ width: `${width}%` }} />
    </div>
  );
}

function deltaText(value: Change, baseline: string): string {
  if (value.direction === "flat") {
    return `Sama dengan ${baseline}`;
  }
  if (value.percent === null) {
    return `Belum ada pembanding dari ${baseline}`;
  }
  return `${value.direction === "up" ? "Naik" : "Turun"} ${value.percent}% dari ${baseline}`;
}

type StatCardProps = {
  title: string;
  value: string;
  percent: number;
  meterLabel: string;
  note?: string;
  delta?: { value: Change; baseline: string };
};

// StatCard menampilkan satu angka utama, arah perubahannya, dan porsinya terhadap periode yang lebih besar.
export function StatCard({ title, value, percent, meterLabel, note, delta }: StatCardProps) {
  const direction = delta?.value.direction;
  return (
    <Card className="grid grid-cols-1 content-start gap-3">
      <h2 className="font-semibold">{title}</h2>
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
        <p className="flex min-w-0 items-center gap-2 text-2xl font-medium tracking-tight">
          {direction === "up" ? <ArrowUpIcon className="size-6 flex-none text-success" /> : null}
          {direction === "down" ? <ArrowDownIcon className="size-6 flex-none text-destructive" /> : null}
          <span className="min-w-0 break-words">{value}</span>
        </p>
        <p className="text-sm text-muted-foreground">{percent}%</p>
      </div>
      <p className="text-sm text-muted-foreground">{delta ? deltaText(delta.value, delta.baseline) : note}</p>
      <Meter percent={percent} />
      <p className="text-xs text-muted-foreground">{meterLabel}</p>
    </Card>
  );
}

type MetricCardProps = {
  icon: ReactNode;
  title: string;
  value: string;
  caption: string;
  parts: { label: string; value: string; percent: number }[];
};

// MetricCard menampilkan satu angka dengan dua rincian di bawahnya.
export function MetricCard({ icon, title, value, caption, parts }: MetricCardProps) {
  return (
    <Card flush className="grid grid-cols-1 content-start gap-4">
      <div className="flex items-start justify-between gap-3 px-5 pt-5">
        <span className="grid size-11 flex-none place-items-center rounded-full bg-accent text-accent-foreground">{icon}</span>
        <div className="min-w-0 text-right">
          <h2 className="text-sm text-muted-foreground">{title}</h2>
          <p className="text-2xl font-medium tracking-tight break-words">{value}</p>
          <p className="text-sm text-muted-foreground">{caption}</p>
        </div>
      </div>
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(7rem,1fr))] gap-4 border-t border-border px-5 pt-4 pb-5">
        {parts.map((part) => (
          <div key={part.label} className="grid content-start gap-2 text-center text-sm">
            <dt className="text-muted-foreground">{part.label}</dt>
            <dd className="grid gap-2 font-medium break-words">
              {part.value}
              <Meter percent={part.percent} />
            </dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}

type DailyBarsProps = {
  hero: string;
  heroLabel: string;
  days: { label: string; value: number; text: string }[];
};

// DailyBars memberi tiap hari nilai tertulis, jadi batangnya hanya bantuan membandingkan.
export function DailyBars({ hero, heroLabel, days }: DailyBarsProps) {
  const highest = Math.max(...days.map(({ value }) => value));
  return (
    <Card className="grid grid-cols-1 content-start gap-4">
      <h2 className="font-semibold">7 hari terakhir</h2>
      <div>
        <p className="text-3xl font-medium tracking-tight break-words">{hero}</p>
        <p className="text-sm text-muted-foreground">{heroLabel}</p>
      </div>
      <ul className="grid gap-3 text-sm">
        {days.map((day) => (
          <li key={day.label} className="grid gap-1.5">
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-muted-foreground">{day.label}</span>
              <span className="font-medium">{day.text}</span>
            </div>
            <Meter percent={highest > 0 ? Math.round((day.value / highest) * 100) : 0} />
          </li>
        ))}
      </ul>
    </Card>
  );
}

export function ExplorerPill({ href }: { href: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="inline-flex min-h-8 items-center rounded-md bg-primary px-3 text-xs font-semibold text-primary-foreground hover:bg-primary/90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      Explorer
    </a>
  );
}
