import { useEffect } from "react";

function blockShortcut(e: KeyboardEvent): boolean {
  const key = e.key.toLowerCase();
  const mod = e.ctrlKey || e.metaKey;
  if (!mod) return false;
  // Печать и сохранение страницы — самые частые способы «забрать» вопросы.
  return key === "p" || key === "s";
}

/** Доп. защита на страницах теста/результата: горячие клавиши, копирование, печать. */
export function useProtectTestContent() {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!blockShortcut(e)) return;
      e.preventDefault();
      e.stopPropagation();
    };

    const blockClipboard = (e: ClipboardEvent) => {
      e.preventDefault();
    };

    const blockContextMenu = (e: MouseEvent) => {
      e.preventDefault();
    };

    document.addEventListener("keydown", onKeyDown, true);
    document.addEventListener("copy", blockClipboard);
    document.addEventListener("cut", blockClipboard);
    document.addEventListener("contextmenu", blockContextMenu);
    document.body.classList.add("test-protected");

    return () => {
      document.removeEventListener("keydown", onKeyDown, true);
      document.removeEventListener("copy", blockClipboard);
      document.removeEventListener("cut", blockClipboard);
      document.removeEventListener("contextmenu", blockContextMenu);
      document.body.classList.remove("test-protected");
    };
  }, []);
}
