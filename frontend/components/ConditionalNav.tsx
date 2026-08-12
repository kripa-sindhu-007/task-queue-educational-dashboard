"use client";

import { usePathname } from "next/navigation";
import SiteNav from "@/components/SiteNav";

/**
 * Renders the global primary nav on every route EXCEPT the landing page (`/`),
 * which owns the full viewport with its immersive hero.
 */
export default function ConditionalNav() {
  const pathname = usePathname();
  if (pathname === "/") return null;
  return <SiteNav />;
}
