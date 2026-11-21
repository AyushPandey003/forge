// Forge actions in the document bar.
//  - On a design document: "Revise" sends open comments (and an optional
//    instruction) to Stitch and publishes the result as a new version.
//  - On any other document: "Design" opens the studio with this doc as the spec.
import { DesignRevisionProgress } from "@/components/design-revision-progress";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Wand2 } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Textarea } from "@kalvi-education/katalyst/components/textarea";
import { Spinner } from "@kalvi-education/katalyst/components/spinner";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@kalvi-education/katalyst/components/popover";
import { api } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { forgeApi, useForgeJob } from "@/lib/forge";

export function ForgeDocActions({
  docId,
  canEdit,
}: {
  docId: string;
  canEdit: boolean;
}) {
  const nav = useNavigate();
  const status = useQuery({
    queryKey: ["forge", "status"],
    queryFn: forgeApi.status,
    staleTime: 60_000,
  });
  const design = useQuery({
    queryKey: ["forge", "design", docId],
    queryFn: () => forgeApi.design(docId),
    enabled: !!docId,
  });
  const d = design.data?.design;
  if (d) {
    const canvas = d.engine === "canvas";
    // Canvas designs revise with the user's own provider; Stitch designs need Stitch.
    if (!canEdit || (canvas ? !status.data?.custom_models : !status.data?.enabled))
      return null;
    return <ReviseDesign docId={docId} canvas={canvas} model={d.model} />;
  }
  if (!status.data?.enabled && !status.data?.custom_models) return null;
  return (
    <Button
      variant="ghost"
      size="sm"
      className="hidden shrink-0 sm:inline-flex"
      title="Generate a UI design from this spec with Stitch + Katalyst"
      onClick={() => nav(`/app/design?spec=${encodeURIComponent(docId)}`)}
    >
      <Wand2 /> Design
    </Button>
  );
}

function ReviseDesign({
  docId,
  canvas,
  model: usedModel,
}: {
  docId: string;
  canvas: boolean;
  model?: string;
}) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [prompt, setPrompt] = useState("");
  const comments = useQuery({
    queryKey: queryKeys.comments(docId),
    queryFn: () => api.listComments(docId),
    enabled: open,
  });
  const openCount = (comments.data?.items || []).filter(
    (c) => c.status === "open",
  ).length;
  const providers = useQuery({
    queryKey: queryKeys.forgeProviders(),
    queryFn: forgeApi.providers,
    enabled: open && canvas,
  });
  const providerList = providers.data?.items || [];
  const [providerId, setProviderId] = useState("");
  const [model, setModel] = useState("");
  const provider = providerList.find((p) => p.id === providerId) || providerList[0];
  const effectiveModel =
    (providerId && model.trim()) ||
    (provider ? usedModel || provider.default_model : "");
  const needsProvider = canvas && (!provider || !effectiveModel);

  const { running, start, job } = useForgeJob((j) => {
    if (j.status === "done") {
      toast.success("New design version published.");
      qc.invalidateQueries({ queryKey: queryKeys.document(docId) });
      qc.invalidateQueries({ queryKey: queryKeys.versions(docId) });
      setPrompt("");
    } else if (j.status === "error") {
      toast.error(j.error || "Revision failed");
    } else if (j.status === "needs_input") {
      toast.message("Stitch has a question", { description: j.question });
    }
  });

  return (
    <>
      {running && (
        <DesignRevisionProgress stage={job?.stage} startedAt={job?.started_at} />
      )}
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant={running ? "secondary" : "ghost"}
            size="sm"
            className="shrink-0"
            title={
              canvas
                ? "Revise this design with your model"
                : "Revise this design with Stitch"
            }
          >
            {running ? <Spinner /> : <Wand2 />}
            <span className="hidden sm:inline">{running ? "Revising…" : "Revise"}</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-80">
          <div className="flex flex-col gap-3">
            <div>
              <div className="text-sm font-medium">
                {canvas ? "Revise design" : "Revise with Stitch"}
              </div>
              <p className="text-xs text-muted-foreground">
                {openCount > 0
                  ? `${openCount} open comment${openCount === 1 ? "" : "s"} will be sent as feedback.`
                  : "No open comments. Describe the change you want."}
              </p>
            </div>
            {canvas && (
              <div className="flex flex-col gap-2">
                {providers.isPending ? (
                  <p role="status" className="text-xs text-muted-foreground">
                    Loading providers…
                  </p>
                ) : providerList.length === 0 ? (
                  <p className="text-xs text-muted-foreground">
                    Add a model provider in the Design studio to revise this design.
                  </p>
                ) : (
                  <>
                    <select
                      aria-label="Model provider"
                      className="h-9 rounded-md border bg-background px-2 text-sm"
                      value={provider?.id || ""}
                      onChange={(e) => {
                        setProviderId(e.target.value);
                        setModel("");
                      }}
                    >
                      {providerList.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.label}
                        </option>
                      ))}
                    </select>
                    <input
                      aria-label="Model"
                      className="h-9 rounded-md border bg-background px-2 text-sm"
                      value={providerId ? model : usedModel || ""}
                      placeholder={provider?.default_model || "Model name"}
                      onChange={(e) => {
                        if (!providerId && provider) setProviderId(provider.id);
                        setModel(e.target.value);
                      }}
                    />
                  </>
                )}
              </div>
            )}
            <Textarea
              rows={3}
              value={prompt}
              disabled={running}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Optional: e.g. use a denser table and move filters into a toolbar"
            />
            <Button
              disabled={running || needsProvider || (openCount === 0 && !prompt.trim())}
              onClick={() => {
                start(() =>
                  forgeApi.revise(docId, {
                    prompt: prompt.trim() || undefined,
                    ...(canvas && provider
                      ? { provider_id: provider.id, model: effectiveModel }
                      : {}),
                  }),
                );
                setOpen(false);
                toast.message(
                  canvas ? "Revising with your model…" : "Revising in Stitch…",
                  {
                    description: canvas
                      ? "A new version will appear shortly."
                      : "This takes 1–3 minutes. A new version will appear.",
                  },
                );
              }}
            >
              {running ? <Spinner /> : <Wand2 />} Publish new version
            </Button>
          </div>
        </PopoverContent>
      </Popover>
    </>
  );
}
