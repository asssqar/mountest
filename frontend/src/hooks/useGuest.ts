import { useEffect, useState } from "react";
import type { Guest } from "../api/types";

const KEY = "mountest_guest";

export function loadGuest(): Guest | null {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Guest;
    if (parsed && parsed.id && parsed.firstName && parsed.lastName) return parsed;
    return null;
  } catch {
    return null;
  }
}

export function saveGuest(g: Guest) {
  localStorage.setItem(KEY, JSON.stringify(g));
}

export function clearGuest() {
  localStorage.removeItem(KEY);
}

export function useGuest() {
  const [guest, setGuest] = useState<Guest | null>(() => loadGuest());

  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === KEY) setGuest(loadGuest());
    };
    window.addEventListener("storage", handler);
    return () => window.removeEventListener("storage", handler);
  }, []);

  return {
    guest,
    setGuest: (g: Guest | null) => {
      if (g) saveGuest(g);
      else clearGuest();
      setGuest(g);
    },
  };
}
