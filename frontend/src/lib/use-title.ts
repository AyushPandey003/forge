import { useEffect } from "react";

import { BRAND } from "./brand";

const BASE = BRAND.name;

// Sets the browser tab title to "<title> · slashdocs" (or just "slashdocs" when no
// page title is given), restoring the previous title on unmount so tabs stay
// distinguishable as the user navigates.
export function useTitle(title?: string) {
  useEffect(() => {
    const prev = document.title;
    document.title = title ? `${title} · ${BASE}` : BASE;
    return () => {
      document.title = prev;
    };
  }, [title]);
}
