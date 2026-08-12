"use client";

import { useCallback, useEffect, useState } from "react";

const KEY_PREFIX = "learn:read:";
const EVENT = "learn:read-change";

function readKey(slug: string) {
  return `${KEY_PREFIX}${slug}`;
}

/** Mark a chapter as read in localStorage and notify listeners in this tab. */
export function markChapterRead(slug: string) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(readKey(slug), "1");
    window.dispatchEvent(new CustomEvent(EVENT));
  } catch {
    // localStorage may be unavailable (private mode, quota) — fail silently.
  }
}

function getReadSet(): Set<string> {
  const set = new Set<string>();
  if (typeof window === "undefined") return set;
  try {
    for (let i = 0; i < window.localStorage.length; i++) {
      const key = window.localStorage.key(i);
      if (key && key.startsWith(KEY_PREFIX)) {
        set.add(key.slice(KEY_PREFIX.length));
      }
    }
  } catch {
    // ignore
  }
  return set;
}

/**
 * Reactive view of which chapters have been read. Updates when a chapter is
 * marked read in this tab or in another tab (storage event).
 */
export function useReadChapters(): {
  readSlugs: Set<string>;
  isRead: (slug: string) => boolean;
} {
  // Start empty so SSR and first client render agree; hydrate in effect.
  const [readSlugs, setReadSlugs] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    const sync = () => setReadSlugs(getReadSet());
    sync();
    window.addEventListener(EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);

  const isRead = useCallback((slug: string) => readSlugs.has(slug), [readSlugs]);

  return { readSlugs, isRead };
}
