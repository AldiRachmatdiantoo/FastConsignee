import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Me } from "@/lib/api/client";
import { AppShell } from "./app-shell";

const signOut = vi.fn();
let pathname = "/dasbor";

vi.mock("next/navigation", () => ({ usePathname: () => pathname }));
vi.mock("@/features/auth/session", () => ({ useSession: () => ({ signOut }) }));

function me(role: Me["role"]): Me {
  return {
    id: "1",
    email: "a@fastconsignee.test",
    name: "Pemilik Demo",
    role,
    store: { id: "2", name: "Toko Titip Nusantara" },
    consignor_id: null,
    wallet: { address: "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a", linked_at: "2026-11-07T00:00:00Z" },
    wallet_required: true,
  } as Me;
}

function show(role: Me["role"]) {
  render(
    <AppShell me={me(role)}>
      <p>Isi layar</p>
    </AppShell>,
  );
}

describe("AppShell", () => {
  beforeEach(() => {
    pathname = "/dasbor";
    signOut.mockClear();
  });

  it("mengelompokkan menu dan menandai layar yang terbuka", () => {
    pathname = "/barang";
    show("store_owner");

    const menu = within(screen.getByRole("navigation", { name: "Menu", hidden: true }));
    expect(menu.getByText("Penjualan")).toBeInTheDocument();
    expect(menu.getByText("Kelola")).toBeInTheDocument();
    expect(menu.getByRole("link", { name: "Barang", hidden: true })).toHaveAttribute("aria-current", "page");
    expect(menu.getByRole("link", { name: "Kasir", hidden: true })).not.toHaveAttribute("aria-current");
    expect(screen.getByText("Isi layar")).toBeVisible();
  });

  it("tidak menampilkan menu untuk kasir yang hanya punya satu layar", () => {
    pathname = "/kasir";
    show("cashier");

    expect(screen.queryByRole("navigation", { name: "Menu", hidden: true })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Buka menu" })).not.toBeInTheDocument();
    expect(screen.getByText("Toko Titip Nusantara")).toBeVisible();
  });

  it("hanya menampilkan layar yang boleh dibuka penitip", () => {
    show("consignor");

    const menu = within(screen.getByRole("navigation", { name: "Menu", hidden: true }));
    expect(menu.getAllByRole("link", { hidden: true }).map((link) => link.textContent)).toEqual(["Dasbor", "Wallet"]);
  });

  it("membuka menu lalu menutupnya dengan Escape", async () => {
    const user = userEvent.setup();
    show("store_owner");
    const open = screen.getByRole("button", { name: "Buka menu" });
    expect(open).toHaveAttribute("aria-expanded", "false");

    await user.click(open);
    expect(open).toHaveAttribute("aria-expanded", "true");
    expect(document.activeElement).toHaveAccessibleName("Tutup menu");

    await user.keyboard("{Escape}");
    expect(open).toHaveAttribute("aria-expanded", "false");
    expect(open).toHaveFocus();
  });

  it("menutup menu setelah memilih layar", async () => {
    const user = userEvent.setup();
    show("store_owner");
    const open = screen.getByRole("button", { name: "Buka menu" });

    await user.click(open);
    await user.click(screen.getByRole("link", { name: "Barang", hidden: true }));
    expect(open).toHaveAttribute("aria-expanded", "false");
  });

  it("menyediakan tombol keluar", async () => {
    const user = userEvent.setup();
    show("consignor");

    await user.click(screen.getByRole("button", { name: "Keluar" }));
    expect(signOut).toHaveBeenCalledOnce();
  });
});
