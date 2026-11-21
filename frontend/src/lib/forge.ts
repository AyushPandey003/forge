// Forge API: design generation with Google Stitch + the Katalyst design system.
// The server runs generation as a job; clients poll it until it settles.
import { useCallback, useEffect, useRef, useState } from "react";

export type ForgeStatus = {
  enabled: boolean;
  /** True when users can bring their own model provider. */
  custom_models?: boolean;
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
  engine?: "stitch" | "canvas";
  model?: string;
  theme?: string;
  stitch_project?: string;
  stitch_screen?: string;
  design_system?: string;
  prompt: string;
  device_type: string;
  source_doc_id?: string;
  updated_at: string;
};

export type DeviceType = "DESKTOP" | "MOBILE" | "TABLET";

export type ProviderKind = "openai" | "anthropic";
export type AIProvider = {
  id: string;
  label: string;
  kind: ProviderKind;
  base_url: string;
  key_hint: string;
  has_key: boolean;
  default_model: string;
};
export type ProviderInput = {
  label: string;
  kind: ProviderKind;
  base_url: string;
  /** Omit (or leave empty) on update to keep the stored key. */
  api_key?: string;
  default_model: string;
};
export type DesignTheme = { id: string; name: string; colors: Record<string, string> };

/** Presets for the providers most people use. Any OpenAI-compatible URL also works. */
export const PROVIDER_PRESETS: {
  id: string;
  label: string;
  kind: ProviderKind;
  base_url: string;
  model: string;
  hint: string;
}[] = [
  {
    id: "openrouter",
    label: "OpenRouter",
    kind: "openai",
    base_url: "https://openrouter.ai/api/v1",
    model: "google/gemini-2.0-flash-001",
    hint: "One key, hundreds of models.",
  },
  {
    id: "openai",
    label: "OpenAI",
    kind: "openai",
    base_url: "https://api.openai.com/v1",
    model: "gpt-4o",
    hint: "GPT models.",
  },
  {
    id: "anthropic",
    label: "Anthropic",
    kind: "anthropic",
    base_url: "https://api.anthropic.com",
    model: "claude-sonnet-4-5",
    hint: "Claude models.",
  },
  {
    id: "gemini",
    label: "Google Gemini",
    kind: "openai",
    base_url: "https://generativelanguage.googleapis.com/v1beta/openai",
    model: "gemini-3.8-flash",
    hint: "Gemini via its OpenAI-compatible API.",
  },
  {
    id: "custom",
    label: "Custom",
    kind: "openai",
    base_url: "",
    model: "",
    hint: "Any OpenAI-compatible endpoint.",
  },
];

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers: { "Content-Type": "application/json", ...(init.headers || {}) },
  });
  if (res.status === 204) return undefined as T;
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
    provider_id?: string;
    model?: string;
    theme?: string;
    one_screen?: boolean;
  }) =>
    call<ForgeJob>("/v1/forge/designs", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  themes: () =>
    call<{ items: DesignTheme[]; default: string }>("/v1/forge/design/themes"),
  providers: () =>
    call<{ items: AIProvider[]; allow_private_urls: boolean; limit: number }>(
      "/v1/forge/ai/providers",
    ),
  createProvider: (input: ProviderInput) =>
    call<AIProvider>("/v1/forge/ai/providers", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  updateProvider: (id: string, input: ProviderInput) =>
    call<AIProvider>(`/v1/forge/ai/providers/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  deleteProvider: (id: string) =>
    call<void>(`/v1/forge/ai/providers/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
  models: (id: string) =>
    call<{ models: string[]; error?: string }>(
      `/v1/forge/ai/providers/${encodeURIComponent(id)}/models`,
    ),
  revise: (
    docId: string,
    input: {
      prompt?: string;
      comment_ids?: string[];
      provider_id?: string;
      model?: string;
      screen_id?: string;
    },
  ) =>
    call<ForgeJob>(`/v1/forge/designs/${encodeURIComponent(docId)}/revise`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  design: (docId: string) =>
    call<{ design: ForgeDesign | null }>(
      `/v1/forge/designs/${encodeURIComponent(docId)}`,
    ),
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

  useEffect(
    () => () => {
      if (timer.current) window.clearTimeout(timer.current);
    },
    [],
  );

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
