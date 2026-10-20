import type { Metadata } from "next";
import { JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { Providers } from "@/components/providers";

const brandMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["500", "600", "700"],
  variable: "--font-brand-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Spearhook — Phishing Operations Console",
  description:
    "Self-hosted phishing operations console for authorized red-team exercises.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={brandMono.variable}>
      <body className="min-h-screen antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
