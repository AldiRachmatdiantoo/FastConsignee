import { describe, expect, it } from "vitest";
import type { Me } from "@/lib/api/client";
import { guardTarget, homePath, navLinks, roleLabel } from "./routes";

const wallet = { address: "4K1QdtVP3i1opNAymPThdcizR7YzRrsHdzbMUMooDU5a", linked_at: "2026-11-07T00:00:00Z" };

function me(role: Me["role"], linked: boolean): Me {
  return {
    id: "1",
    email: "a@fastconsignee.test",
    name: "Akun",
    role,
    store: { id: "2", name: "Toko" },
    consignor_id: null,
    wallet: linked ? wallet : null,
    wallet_required: role !== "cashier",
  };
}

describe("homePath", () => {
  it("mengarahkan tiap peran ke layar pertamanya", () => {
    expect(homePath(me("cashier", false))).toBe("/kasir");
    expect(homePath(me("store_owner", false))).toBe("/wallet");
    expect(homePath(me("consignor", false))).toBe("/wallet");
    expect(homePath(me("store_owner", true))).toBe("/dasbor");
    expect(homePath(me("consignor", true))).toBe("/dasbor");
  });
});

describe("guardTarget", () => {
  it("menahan kasir di layar kasir", () => {
    expect(guardTarget(me("cashier", false), "/kasir")).toBeNull();
    expect(guardTarget(me("cashier", false), "/dasbor")).toBe("/kasir");
    expect(guardTarget(me("cashier", false), "/wallet")).toBe("/kasir");
  });

  it("menahan pemilik dan penitip di penautan wallet sampai wallet tertaut", () => {
    expect(guardTarget(me("store_owner", false), "/dasbor")).toBe("/wallet");
    expect(guardTarget(me("consignor", false), "/kasir")).toBe("/wallet");
    expect(guardTarget(me("consignor", false), "/wallet")).toBeNull();
  });

  it("membuka layar sesuai peran setelah wallet tertaut", () => {
    expect(guardTarget(me("store_owner", true), "/dasbor")).toBeNull();
    expect(guardTarget(me("store_owner", true), "/kasir")).toBeNull();
    expect(guardTarget(me("store_owner", true), "/wallet")).toBeNull();
    expect(guardTarget(me("consignor", true), "/dasbor")).toBeNull();
    expect(guardTarget(me("consignor", true), "/kasir")).toBe("/dasbor");
  });
});

describe("roleLabel", () => {
  it("menerjemahkan peran", () => {
    expect(roleLabel("store_owner")).toBe("Pemilik toko");
  });
});

describe("layar khusus pemilik", () => {
  it("mengembalikan penitip ke dasbor", () => {
    for (const path of ["/kasir", "/refund", "/barang", "/penitip"]) {
      expect(guardTarget(me("consignor", true), path)).toBe("/dasbor");
    }
  });

  it("mengizinkan pemilik membukanya", () => {
    for (const path of ["/kasir", "/refund", "/barang", "/penitip"]) {
      expect(guardTarget(me("store_owner", true), path)).toBeNull();
    }
  });
});

describe("navLinks", () => {
  const paths = (role: Me["role"]) => navLinks(me(role, true)).map(({ href }) => href);

  it("hanya memuat layar yang boleh dibuka peran itu", () => {
    expect(paths("cashier")).toEqual(["/kasir"]);
    expect(paths("consignor")).toEqual(["/dasbor", "/wallet"]);
    expect(paths("store_owner")).toEqual(["/dasbor", "/kasir", "/refund", "/barang", "/penitip", "/wallet"]);
  });
});
