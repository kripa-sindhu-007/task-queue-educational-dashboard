"use client";

import { useEffect } from "react";
import { markChapterRead } from "@/lib/learnProgress";

/** Side-effect-only component: marks a chapter read on mount. */
export default function ChapterReadMarker({ slug }: { slug: string }) {
  useEffect(() => {
    markChapterRead(slug);
  }, [slug]);
  return null;
}
