"use client";

import { useSession } from "@/features/auth/session";
import { ConsignorDashboard } from "./consignor-dashboard";
import { StoreDashboard } from "./store-dashboard";

export function DasborScreen() {
  const { session } = useSession();

  if (session.status !== "authenticated") {
    return null;
  }
  return session.me.role === "consignor" ? <ConsignorDashboard /> : <StoreDashboard />;
}
