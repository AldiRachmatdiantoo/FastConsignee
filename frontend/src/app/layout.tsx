import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import type { ReactNode } from "react";
import { SessionProvider } from "@/features/auth/session";
import "./globals.css";

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });
// Font ini hanya dipakai untuk alamat wallet, jadi tidak perlu berebut unduhan di awal halaman.
const jetbrains = JetBrains_Mono({ subsets: ["latin"], variable: "--font-jetbrains", preload: false });

export const metadata: Metadata = {
  title: "FastConsignee",
  description: "A consignment cashier that splits every payment in one Solana transaction.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${jetbrains.variable}`}>
      <body className="min-h-dvh antialiased">
        <SessionProvider>{children}</SessionProvider>
      </body>
    </html>
  );
}
