import { describe, expect, it } from "vitest";
import type { components } from "@/lib/api/schema";
import { EMPTY_FORM, formFromItem, toCreate, toUpdate } from "./item-form";

const item = {
  id: "i1",
  name: "Dompet Pandan",
  price_idr: 45000,
  commission_bps: 1250,
  consignor: { id: "c1", name: "Sari Anyaman" },
  stock_on_hand: 7,
  active: false,
  certificate_enabled: true,
} as components["schemas"]["Item"];

describe("item form", () => {
  it("mengubah persen ke basis point saat membuat barang", () => {
    const body = toCreate({ ...EMPTY_FORM, name: "  Tas Rotan ", price: "185000", commission: "12.5", consignorId: "c1", stock: "3" });
    expect(body).toEqual({
      name: "Tas Rotan",
      price_idr: 185000,
      commission_bps: 1250,
      consignor_id: "c1",
      stock_on_hand: 3,
      certificate_enabled: false,
    });
  });

  it("tidak kehilangan pecahan basis point karena pembulatan biner", () => {
    expect(toCreate({ ...EMPTY_FORM, commission: "19.99" }).commission_bps).toBe(1999);
  });

  it("mengisi form dari barang lalu mengirim nilai yang sama", () => {
    const form = formFromItem(item);
    expect(form.commission).toBe("12.5");
    expect(toUpdate(form)).toEqual({
      name: "Dompet Pandan",
      price_idr: 45000,
      commission_bps: 1250,
      stock_on_hand: 7,
      active: false,
      certificate_enabled: true,
    });
  });
});
