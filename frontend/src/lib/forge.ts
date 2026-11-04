// Forge API: design generation with Google Stitch + the Katalyst design system.
// The server runs generation as a job; clients poll it until it settles.
import { useCallback, useEffect, useRef, useState } from "react";

export type ForgeStatus = {
  enabled: boolean;
  design_system: string;
  design_md_bytes: number;
  stitch_project?: string;
};

export type ForgeJob = {
  id: string;
  status: "queued" | "running" | "done" | "needs_input" | "error";
  stage: string;
  error?: string;
  question?: string;
  suggestions?: string[];
  document_id?: string;
  version_id?: string;
  started_at: string;
  finished_at?: string;
};

export type ForgeDesign = {
  kind: "design";
  stitch_project: string;
  stitch_screen: string;
  design_system: string;
  prompt: string;
  device_type: string;
  source_doc_id?: string;
  updated_at: string;
};

export type DeviceType = "DESKTOP" | "MOBILE" | "TABLET";

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers: { "Content-Type": "application/json", ...(init.headers || {}) },
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body?.error?.message || res.statusText);
  }
  return body as T;
}

export const forgeApi = {
  status: () => call<ForgeStatus>("/v1/forge/status"),
  createDesign: (input: {
    prompt: string;
    title?: string;
    device_type?: DeviceType;
    source_doc_id?: string;
    solution_id?: string;
    stage?: string;
  }) => call<ForgeJob>("/v1/forge/designs", { method: "POST", body: JSON.stringify(input) }),
  revise: (docId: string, input: { prompt?: string; comment_ids?: string[] }) =>
    call<ForgeJob>(`/v1/forge/designs/${encodeURIComponent(docId)}/revise`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  design: (docId: string) =>
    call<{ design: ForgeDesign | null }>(`/v1/forge/designs/${encodeURIComponent(docId)}`),
  job: (id: string) => call<ForgeJob>(`/v1/forge/jobs/${encodeURIComponent(id)}`),
};

export const jobSettled = (j?: ForgeJob | null) =>
  !!j && (j.status === "done" || j.status === "error" || j.status === "needs_input");

// useForgeJob starts a job and polls it until it settles.
export function useForgeJob(onSettled?: (job: ForgeJob) => void) {
  const [job, setJob] = useState<ForgeJob | null>(null);
  const [startError, setStartError] = useState<string | null>(null);
  const timer = useRef<number | null>(null);
  const settledRef = useRef(onSettled);
  settledRef.current = onSettled;

  useEffect(() => () => {
    if (timer.current) window.clearTimeout(timer.current);
  }, []);

  const poll = useCallback((id: string) => {
    timer.current = window.setTimeout(async () => {
      try {
        const next = await forgeApi.job(id);
        setJob(next);
        if (jobSettled(next)) settledRef.current?.(next);
        else poll(id);
      } catch {
        poll(id);
      }
    }, 2500);
  }, []);

  const start = useCallback(
    async (begin: () => Promise<ForgeJob>) => {
      setStartError(null);
      try {
        const j = await begin();
        setJob(j);
        poll(j.id);
      } catch (e) {
        setStartError(e instanceof Error ? e.message : String(e));
      }
    },
    [poll],
  );

  const reset = useCallback(() => {
    if (timer.current) window.clearTimeout(timer.current);
    setJob(null);
    setStartError(null);
  }, []);

  const running = !!job && !jobSettled(job);
  return { job, running, start, reset, startError };
}
