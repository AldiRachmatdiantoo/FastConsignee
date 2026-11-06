import type { components } from "@/lib/api/schema";

type Item = components["schemas"]["Item"];
type ItemCreate = components["schemas"]["ItemCreate"];
type ItemUpdate = components["schemas"]["ItemUpdate"];

export type ItemForm = {
  name: string;
  price: string;
  commission: string;
  consignorId: string;
  stock: string;
  active: boolean;
  certificate: boolean;
};

export const EMPTY_FORM: ItemForm = {
  name: "",
  price: "",
  commission: "20",
  consignorId: "",
  stock: "1",
  active: true,
  certificate: false,
};

export function formFromItem(item: Item): ItemForm {
  return {
    name: item.name,
    price: String(item.price_idr),
    commission: String(item.commission_bps / 100),
    consignorId: item.consignor.id,
    stock: String(item.stock_on_hand),
    active: item.active,
    certificate: item.certificate_enabled,
  };
}

// Komisi diisi dalam persen, sedangkan API memakai basis point.
function toBps(percent: string): number {
  return Math.round(Number(percent) * 100);
}

export function toCreate(form: ItemForm): ItemCreate {
  return {
    name: form.name.trim(),
    price_idr: Number(form.price),
    commission_bps: toBps(form.commission),
    consignor_id: form.consignorId,
    stock_on_hand: Number(form.stock),
    certificate_enabled: form.certificate,
  };
}

export function toUpdate(form: ItemForm): ItemUpdate {
  return {
    name: form.name.trim(),
    price_idr: Number(form.price),
    commission_bps: toBps(form.commission),
    stock_on_hand: Number(form.stock),
    active: form.active,
    certificate_enabled: form.certificate,
  };
}
