"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { Loading } from "@/components/ui";
import { homePath } from "@/features/auth/routes";
import { useSession } from "@/features/auth/session";

export default function Home() {
  const { session } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (session.status === "anonymous") {
      router.replace("/login");
    }
    if (session.status === "authenticated") {
      router.replace(homePath(session.me));
    }
  }, [session, router]);

  return <Loading />;
}
