"use client";

import { useCallback, useState } from "react";
import { BoxIcon, RefundIcon, TagIcon } from "@/components/icons";
import { Card, EmptyState, ListSkeleton, LoadMore, Notice, PageTitle } from "@/components/ui";
import { useSession } from "@/features/auth/session";
import { unwrap } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { formatDateTime, formatRupiah, formatSol } from "@/lib/format";
import { usePagedList, useResource } from "@/lib/hooks";
import { change, isWithin, lastDays, monthPeriod, share, sumBy, walkPages, type Period } from "./stats";
import { DailyBars, DailyBarsSkeleton, ExplorerPill, MetricCard, StatCard, TilesSkeleton } from "./widgets";

type Sale = components["schemas"]["ConsignorSale"];
type Deduction = components["schemas"]["Deduction"];
type Stock = components["schemas"]["ConsignorStock"];

const PAGE_SIZE = 100;
const MAX_PAGES = 10;

type Overview = {
  sales: Sale[];
  stock: Stock[];
  deductions: Deduction[];
  outstanding: number;
  complete: boolean;
  now: Date;
};

export function ConsignorDashboard() {
  const { api } = useSession();

  // Belum ada endpoint ringkasan untuk penitip, jadi angkanya dijumlah dari daftar lengkap.
  const loadOverview = useCallback(async (): Promise<Overview> => {
    let outstanding = 0;
    const [sales, stock, deductions] = await Promise.all([
      walkPages(
        (cursor) =>
          unwrap(api.GET("/api/v1/dashboard/consignor/sales", { params: { query: { limit: PAGE_SIZE, cursor } } })),
        MAX_PAGES,
      ),
      walkPages(
        (cursor) =>
          unwrap(api.GET("/api/v1/dashboard/consignor/stock", { params: { query: { limit: PAGE_SIZE, cursor } } })),
        MAX_PAGES,
      ),
      walkPages(async (cursor) => {
        const page = await unwrap(
          api.GET("/api/v1/dashboard/consignor/deductions", { params: { query: { limit: PAGE_SIZE, cursor } } }),
        );
        outstanding = page.outstanding_total_idr;
        return page;
      }, MAX_PAGES),
    ]);
    return {
      sales: sales.items,
      stock: stock.items,
      deductions: deductions.items,
      outstanding,
      complete: sales.complete && stock.complete && deductions.complete,
      now: new Date(),
    };
  }, [api]);
  const { state } = useResource(loadOverview);

  return (
    <div className="grid gap-6">
      <PageTitle title="Consignor dashboard" text="You can check every sale yourself on Solana Explorer." />

      {state.status === "error" ? <Notice tone="danger">{state.message}</Notice> : null}
      {state.status === "loading" ? <TilesSkeleton /> : null}
      {state.status === "ready" && !state.data.complete ? (
        <Notice tone="info">This summary covers the {PAGE_SIZE * MAX_PAGES} latest records.</Notice>
      ) : null}
      {state.status === "ready" ? <Tiles overview={state.data} /> : null}

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        {state.status === "loading" ? <DailyBarsSkeleton /> : null}
        {state.status === "ready" ? <WeekCard overview={state.data} /> : null}
        <Sales />
      </div>

      <div className="grid items-start gap-6 lg:grid-cols-2">
        <Deductions />
        <StockList />
      </div>
    </div>
  );
}

function shareIn(sales: Sale[], period: Period): number {
  return sumBy(
    sales.filter((sale) => isWithin(sale.sold_at, period)),
    (sale) => sale.consignor_share_idr,
  );
}

function unitsIn(sales: Sale[], period: Period): number {
  return sumBy(
    sales.filter((sale) => isWithin(sale.sold_at, period)),
    (sale) => sale.quantity,
  );
}

function Tiles({ overview }: { overview: Overview }) {
  const { sales, stock, deductions, outstanding, now } = overview;
  const [today, yesterday] = lastDays(now, 2);
  const month = monthPeriod(now);
  const lastMonth = monthPeriod(now, -1);

  const totalShare = sumBy(sales, (sale) => sale.consignor_share_idr);
  const totalPrice = sumBy(sales, (sale) => sale.line_total_idr);
  const shareToday = shareIn(sales, today);
  const shareMonth = shareIn(sales, month);

  const units = sumBy(sales, (sale) => sale.quantity);
  const refunded = sumBy(sales, (sale) => sale.refunded_quantity);
  const unitsToday = unitsIn(sales, today);
  const unitsMonth = unitsIn(sales, month);

  const active = stock.filter((item) => item.active);
  const settled = deductions.filter((deduction) => deduction.status === "settled").length;
  const open = deductions.length - settled;

  return (
    <>
      <div className="grid gap-4 md:grid-cols-3">
        <StatCard
          title="Today's share"
          value={formatRupiah(shareToday)}
          delta={{ value: change(shareToday, shareIn(sales, yesterday)), baseline: "yesterday" }}
          percent={share(shareToday, shareMonth)}
          meterLabel="Portion of this month's share"
        />
        <StatCard
          title="This month's share"
          value={formatRupiah(shareMonth)}
          delta={{ value: change(shareMonth, shareIn(sales, lastMonth)), baseline: "last month" }}
          percent={share(shareMonth, totalShare)}
          meterLabel="Portion of all your shares"
        />
        <StatCard
          title="Total share"
          value={formatRupiah(totalShare)}
          note="Before refund deductions"
          percent={share(totalShare, totalPrice)}
          meterLabel="Portion of the sale price"
        />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard
          icon={<TagIcon />}
          title="Items sold"
          value={`${units} pcs`}
          caption={`${refunded} units refunded`}
          parts={[
            { label: "Today", value: `${unitsToday} pcs`, percent: share(unitsToday, units) },
            { label: "This month", value: `${unitsMonth} pcs`, percent: share(unitsMonth, units) },
          ]}
        />
        <MetricCard
          icon={<BoxIcon />}
          title="Stock left"
          value={`${sumBy(active, (item) => item.stock_available)} pcs`}
          caption="Across active items"
          parts={[
            { label: "Active items", value: String(active.length), percent: share(active.length, stock.length) },
            {
              label: "Inactive items",
              value: String(stock.length - active.length),
              percent: share(stock.length - active.length, stock.length),
            },
          ]}
        />
        <MetricCard
          icon={<RefundIcon />}
          title="Unpaid deductions"
          value={formatRupiah(outstanding)}
          caption="Taken from the next sales"
          parts={[
            { label: "Still open", value: String(open), percent: share(open, deductions.length) },
            { label: "Paid off", value: String(settled), percent: share(settled, deductions.length) },
          ]}
        />
      </div>
    </>
  );
}

function WeekCard({ overview }: { overview: Overview }) {
  const days = lastDays(overview.now, 7).map((day) => ({ label: day.label, value: shareIn(overview.sales, day) }));
  const week = sumBy(days, ({ value }) => value);
  return (
    <DailyBars
      hero={formatRupiah(week)}
      heroLabel={`Average ${formatRupiah(Math.round(week / days.length))} per day`}
      days={days.map((day) => ({ ...day, text: formatRupiah(day.value) }))}
    />
  );
}

function Sales() {
  const { api } = useSession();
  const load = useCallback(
    (cursor?: string) => unwrap(api.GET("/api/v1/dashboard/consignor/sales", { params: { query: { limit: 10, cursor } } })),
    [api],
  );
  const { items, cursor, status, message, loadMore } = usePagedList<Sale>(load);

  return (
    <Card flush>
      <h2 className="border-b border-border px-5 py-4 font-semibold">Sales</h2>
      {message ? (
        <div className="px-5 pt-3">
          <Notice tone="danger">{message}</Notice>
        </div>
      ) : null}
      {status === "loading" ? <ListSkeleton rows={3} lines={3} icon rowClassName="px-5" /> : null}
      {status === "ready" && items.length === 0 ? <EmptyState>No items sold yet.</EmptyState> : null}
      <ul className="divide-y divide-border text-sm empty:hidden">
        {items.map((sale) => (
          <li key={sale.order_line_id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3">
            <span className="grid size-10 flex-none place-items-center rounded-full bg-accent text-accent-foreground">
              <TagIcon />
            </span>
            <div className="min-w-0 flex-1 basis-52">
              <p className="font-semibold">
                {sale.item_name} × {sale.quantity}
              </p>
              <p className="text-muted-foreground">{formatDateTime(sale.sold_at)}</p>
              <p className="text-muted-foreground">
                Price {formatRupiah(sale.line_total_idr)} · commission {formatRupiah(sale.commission_idr)}
              </p>
              {sale.refunded_quantity > 0 ? (
                <p className="text-muted-foreground">{sale.refunded_quantity} units refunded</p>
              ) : null}
              {sale.order_deduction_idr > 0 ? (
                <p className="text-muted-foreground">Deducted {formatRupiah(sale.order_deduction_idr)} for an earlier refund</p>
              ) : null}
            </div>
            <div className="sm:text-right">
              <p className="font-semibold">{formatRupiah(sale.consignor_share_idr)}</p>
              <p className="font-mono text-xs text-muted-foreground">{formatSol(sale.order_transfer_lamports)} per order</p>
            </div>
            <ExplorerPill href={sale.explorer_url} />
          </li>
        ))}
      </ul>
      <div className="px-5 pb-4 empty:hidden">
        <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
      </div>
    </Card>
  );
}

function Deductions() {
  const { api } = useSession();
  const [outstanding, setOutstanding] = useState(0);

  const load = useCallback(
    async (cursor?: string) => {
      const page = await unwrap(
        api.GET("/api/v1/dashboard/consignor/deductions", { params: { query: { limit: 20, cursor } } }),
      );
      setOutstanding(page.outstanding_total_idr);
      return page;
    },
    [api],
  );
  const { items, cursor, status, message, loadMore } = usePagedList<Deduction>(load);

  return (
    <Card>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="font-semibold">Refund deductions</h2>
        <p className="text-sm text-muted-foreground">
          Unpaid <span className="font-semibold text-foreground">{formatRupiah(outstanding)}</span>
        </p>
      </div>
      {message ? <Notice tone="danger">{message}</Notice> : null}
      {status === "loading" ? <ListSkeleton rows={3} className="mt-2" /> : null}
      {status === "ready" && items.length === 0 ? <EmptyState>No deductions.</EmptyState> : null}
      <ul className="mt-2 divide-y divide-border text-sm empty:hidden">
        {items.map((deduction) => (
          <li key={deduction.id} className="flex items-baseline justify-between gap-3 py-3">
            <div>
              <p className="font-semibold">{deduction.item_name}</p>
              <p className="text-muted-foreground">{formatDateTime(deduction.created_at)}</p>
            </div>
            <div className="text-right">
              <p className="font-semibold">{formatRupiah(deduction.amount_idr)}</p>
              <p className="text-muted-foreground">
                {deduction.status === "settled" ? "Paid" : `Remaining ${formatRupiah(deduction.remaining_idr)}`}
              </p>
            </div>
          </li>
        ))}
      </ul>
      <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
    </Card>
  );
}

function StockList() {
  const { api } = useSession();
  const load = useCallback(
    (cursor?: string) => unwrap(api.GET("/api/v1/dashboard/consignor/stock", { params: { query: { limit: 20, cursor } } })),
    [api],
  );
  const { items, cursor, status, message, loadMore } = usePagedList<Stock>(load);

  return (
    <Card>
      <h2 className="font-semibold">Consigned stock</h2>
      {message ? <Notice tone="danger">{message}</Notice> : null}
      {status === "loading" ? <ListSkeleton rows={3} className="mt-2" /> : null}
      {status === "ready" && items.length === 0 ? <EmptyState>No consigned items yet.</EmptyState> : null}
      <ul className="mt-2 divide-y divide-border text-sm empty:hidden">
        {items.map((stock) => (
          <li key={stock.item_id} className="flex items-baseline justify-between gap-3 py-3">
            <div>
              <p className="font-semibold">{stock.item_name}</p>
              <p className="text-muted-foreground">
                {formatRupiah(stock.price_idr)}
                {stock.active ? "" : " · inactive"}
              </p>
            </div>
            <p className="font-semibold whitespace-nowrap">Stock {stock.stock_available}</p>
          </li>
        ))}
      </ul>
      <LoadMore visible={cursor !== null} busy={status === "more"} onClick={() => void loadMore()} />
    </Card>
  );
}
