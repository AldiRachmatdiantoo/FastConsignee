"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { AppShell } from "@/components/app-shell";
import { Loading } from "@/components/ui";
import { guardTarget } from "@/features/auth/routes";
import { useSession } from "@/features/auth/session";
import { UnavailableScreen } from "@/features/auth/unavailable-screen";

export default function AppLayout({ children }: { children: ReactNode }) {
  const { session, refresh } = useSession();
  const router = useRouter();
  const pathname = usePathname();

  let target: string | null = null;
  if (session.status === "anonymous") {
    target = "/login";
  }
  if (session.status === "authenticated") {
    target = guardTarget(session.me, pathname);
  }

  useEffect(() => {
    if (target) {
      router.replace(target);
    }
  }, [target, router]);

  if (session.status === "unavailable") {
    return <UnavailableScreen message={session.error} onRetry={refresh} />;
  }
  if (session.status !== "authenticated" || target) {
    return <Loading />;
  }
  return <AppShell me={session.me}>{children}</AppShell>;
}
