import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { solutionKeys } from "./solutions";

// One stream on the active solution page. Invalidation refetches server-authorized
// snapshots; it never merges remote state into an unsaved document editor.
export function useForgeLive() {
  const qc = useQueryClient();
  const [connected, setConnected] = useState(false);
  useEffect(() => {
    const source = new EventSource("/v1/forge/live", { withCredentials: true });
    let timer: ReturnType<typeof setTimeout> | undefined;
    const refresh = () => {
      if (timer) return;
      timer = setTimeout(() => {
        timer = undefined;
        void qc.invalidateQueries({ queryKey: solutionKeys.all });
      }, 200);
    };
    source.onopen = () => setConnected(true);
    source.addEventListener("invalidate", refresh);
    source.onerror = () => setConnected(false);
    return () => {
      source.close();
      clearTimeout(timer);
    };
  }, [qc]);
  return connected;
}
