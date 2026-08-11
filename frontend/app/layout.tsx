import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import ConditionalNav from "@/components/ConditionalNav";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Task Queue — watch a distributed queue run",
  description:
    "An educational dashboard for distributed task processing: leases, failure detection, and recovery, live.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      className={`dark ${inter.variable} ${jetbrainsMono.variable}`}
      suppressHydrationWarning
    >
      <body>
        <TooltipProvider>
          <ConditionalNav />
          {children}
          <Toaster theme="dark" richColors position="bottom-right" />
        </TooltipProvider>
      </body>
    </html>
  );
}
