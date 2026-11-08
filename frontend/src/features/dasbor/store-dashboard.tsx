"use client";

import { useCallback } from "react";
import { CoinsIcon, ReceiptIcon, UsersIcon } from "@/components/icons";
import { Card, EmptyState, ListSkeleton, LoadMore, Notice, PageTitle } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDateTime, formatRupiah } from "@/lib/format";
import { usePagedList, useResource } from "@/lib/hooks";
import { change, isWithin, lastDays, monthPeriod, share, sumBy, walkPages, type DayPeriod, type Period } from "./stats";
import { DailyBars, DailyBarsSkeleton, ExplorerPill, MetricCard, StatCard, TilesSkeleton } from "./widgets";

type OrderSummary = components["schemas"]["OrderSummary"];
type Summary = components["schemas"]["StoreSummary"];

type DayTotal = { label: string; sales: number; commission: number; count: number };

type Overview = {
  total: Summary;
  month: Summary;
  lastMonth: Summary;
  days: DayTotal[];
  complete: boolean;
};

const PAGE_SIZE = 100;
const MAX_PAGES = 10;

function consignorShare(summary: Summary): number {
  return summary.total_sales_idr - summary.total_commission_idr;
}

function dayTotal(day: DayPeriod, orders: OrderSummary[]): DayTotal {
  const confirmed = orders.filter((order) => isWithin(order.confirmed_at ?? order.created_at, day));
  return {
    label: day.label,
    sales: sumBy(confirmed, (order) => order.total_idr),
    commission: sumBy(confirmed, (order) => order.commission_idr),
    count: confirmed.length,
  };
}

export function StoreDashboard() {
  const { api } = useSession();

  const loadOverview = useCallback(async (): Promise<Overview> => {
    const now = new Date();
    const days = lastDays(now, 7);
    // Filter `from` memakai waktu order dibuat, jadi diambil sehari lebih awal untuk order yang terkonfirmasi lewat tengah malam.
    const from = lastDays(now, 8)[7].from;
    const summary = (period?: Period) =>
      unwrap(api.GET("/api/v1/dashboard/store/summary", { params: { query: period ?? {} } }));
    const [total, month, lastMonth, orders] = await Promise.all([
      summary(),
      summary(monthPeriod(now)),
      summary(monthPeriod(now, -1)),
      walkPages(
        (cursor) =>
          unwrap(api.GET("/api/v1/orders", { params: { query: { status: "confirmed", from, limit: PAGE_SIZE, cursor } } })),
        MAX_PAGES,
      ),
    ]);
    return {
      total,
      month,
      lastMonth,
      days: days.map((day) => dayTotal(day, orders.items)),
      complete: orders.complete,
    };
  }, [api]);
  const { state } = useResource(loadOverview);

  return (
    <div className="grid gap-6">
      <PageTitle title="Store dashboard" text="This summary can lag by up to 30 seconds." />

      {state.status === "error" ? <Notice tone="danger">{state.message}</Notice> : null}
      {state.status === "loading" ? <TilesSkeleton /> : null}
      {state.status === "ready" && !state.data.complete ? (
        <Notice tone="info">Daily figures cover the {PAGE_SIZE * MAX_PAGES} latest transactions.</Notice>
      ) : null}
      {state.status === "ready" ? <Tiles overview={state.data} /> : null}

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        {state.status === "loading" ? <DailyBarsSkeleton /> : null}
        {state.status === "ready" ? <WeekCard overview={state.data} /> : null}
        <Orders />
      </div>
    </div>
  );
}

function Tiles({ overview }: { overview: Overview }) {
  const { total, month, lastMonth, days } = overview;
  const [today, yesterday] = days;
  const sales = total.total_sales_idr;
  const consignors = consignorShare(total);

  return (
    <>
      <div className="grid gap-4 md:grid-cols-3">
        <StatCard
          title="Sales today"
          value={formatRupiah(today.sales)}
          delta={{ value: change(today.sales, yesterday.sales), baseline: "yesterday" }}
          percent={share(today.sales, month.total_sales_idr)}
          meterLabel="Portion of this month's sales"
        />
        <StatCard
          title="Sales this month"
          value={formatRupiah(month.total_sales_idr)}
          delta={{ value: change(month.total_sales_idr, lastMonth.total_sales_idr), baseline: "last month" }}
          percent={share(month.total_sales_idr, sales)}
          meterLabel="Portion of all sales"
        />
        <StatCard
          title="Total sales"
          value={formatRupiah(sales)}
          note={`${total.transaction_count} confirmed transactions`}
          percent={share(consignors, sales)}
          meterLabel="Portion paid to consignors"
        />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard
          icon={<CoinsIcon />}
          title="Shop commission"
          value={formatRupiah(total.total_commission_idr)}
          caption={`${share(total.total_commission_idr, sales)}% of sales`}
          parts={[
            {
              label: "Today",
              value: formatRupiah(today.commission),
              percent: share(today.commission, total.total_commission_idr),
            },
            {
              label: "This month",
              value: formatRupiah(month.total_commission_idr),
              percent: share(month.total_commission_idr, total.total_commission_idr),
            },
          ]}
        />
        <MetricCard
          icon={<ReceiptIcon />}
          title="Transactions"
          value={String(total.transaction_count)}
          caption={`Average ${formatRupiah(Math.round(sales / Math.max(total.transaction_count, 1)))}`}
          parts={[
            {
              label: "Today",
              value: String(today.count),
              percent: share(today.count, total.transaction_count),
            },
            {
              label: "This month",
              value: String(month.transaction_count),
              percent: share(month.transaction_count, total.transaction_count),
            },
          ]}
        />
        <MetricCard
          icon={<UsersIcon />}
          title="Consignor share"
          value={formatRupiah(consignors)}
          caption="Before refund deductions"
          parts={[
            {
              label: "Today",
              value: formatRupiah(today.sales - today.commission),
              percent: share(today.sales - today.commission, consignors),
            },
            {
              label: "This month",
              value: formatRupiah(consignorShare(month)),
              percent: share(consignorShare(month), consignors),
            },
          ]}
        />
      </div>
    </>
  );
}

function WeekCard({ overview }: { overview: Overview }) {
  const week = sumBy(overview.days, ({ sales }) => sales);
  return (
    <DailyBars
      hero={formatRupiah(week)}
      heroLabel={`Average ${formatRupiah(Math.round(week / overview.days.length))} per day`}
      days={overview.days.map(({ label, sales }) => ({ label, value: sales, text: formatRupiah(sales) }))}
    />
  );
}

function Orders() {
  const { api } = useSession();
  const load = useCallback(
    (cursor?: string) => unwrap(api.GET("/api/v1/orders", { params: { query: { status: "confirmed", limit: 10, cursor } } })),
    [api],
  );
  const orders = usePagedList<OrderSummary>(load);

  return (
    <Card flush>
      <h2 className="border-b border-border px-5 py-4 font-semibold">Confirmed transactions</h2>
      {orders.message ? (
        <div className="px-5 pt-3">
          <Notice tone="danger">{orders.message}</Notice>
        </div>
      ) : null}
      {orders.status === "loading" ? <ListSkeleton rows={3} icon rowClassName="px-5" /> : null}
      {orders.status === "ready" && orders.items.length === 0 ? <EmptyState>No transactions yet.</EmptyState> : null}
      <ul className="divide-y divide-border text-sm empty:hidden">
        {orders.items.map((order) => (
          <li key={order.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3">
            <span className="grid size-10 flex-none place-items-center rounded-full bg-accent text-accent-foreground">
              <ReceiptIcon />
            </span>
            <div className="min-w-0 flex-1 basis-40">
              <p className="font-semibold">{formatRupiah(order.total_idr)}</p>
              <p className="text-muted-foreground">
                {order.item_count} items · commission {formatRupiah(order.commission_idr)}
              </p>
            </div>
            <p className="flex items-center gap-2 text-muted-foreground">
              <span aria-hidden="true" className="size-2 flex-none rounded-full bg-success" />
              {formatDateTime(order.confirmed_at ?? order.created_at)}
            </p>
            {order.explorer_url ? <ExplorerPill href={order.explorer_url} /> : null}
          </li>
        ))}
      </ul>
      <div className="px-5 pb-4 empty:hidden">
        <LoadMore visible={orders.cursor !== null} busy={orders.status === "more"} onClick={() => void orders.loadMore()} />
      </div>
    </Card>
  );
}
