// Forge actions in the document bar.
//  - On a design document: "Revise" sends open comments (and an optional
//    instruction) to Stitch and publishes the result as a new version.
//  - On any other document: "Design" opens the studio with this doc as the spec.
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

export function ForgeDocActions({ docId, canEdit }: { docId: string; canEdit: boolean }) {
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
  if (!status.data?.enabled) return null;
  if (design.data?.design) {
    return canEdit ? <ReviseDesign docId={docId} /> : null;
  }
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

function ReviseDesign({ docId }: { docId: string }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [prompt, setPrompt] = useState("");
  const comments = useQuery({
    queryKey: queryKeys.comments(docId),
    queryFn: () => api.listComments(docId),
    enabled: open,
  });
  const openCount = (comments.data?.items || []).filter((c) => c.status === "open").length;

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
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant={running ? "secondary" : "ghost"}
          size="sm"
          className="shrink-0"
          title="Revise this design with Stitch"
        >
          {running ? <Spinner /> : <Wand2 />}
          <span className="hidden sm:inline">{running ? job?.stage || "Revising…" : "Revise"}</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80">
        <div className="flex flex-col gap-3">
          <div>
            <div className="text-sm font-medium">Revise with Stitch</div>
            <p className="text-xs text-muted-foreground">
              {openCount > 0
                ? `${openCount} open comment${openCount === 1 ? "" : "s"} will be sent as feedback.`
                : "No open comments. Describe the change you want."}
            </p>
          </div>
          <Textarea
            rows={3}
            value={prompt}
            disabled={running}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="Optional: e.g. use a denser table and move filters into a toolbar"
          />
          <Button
            disabled={running || (openCount === 0 && !prompt.trim())}
            onClick={() => {
              start(() => forgeApi.revise(docId, { prompt: prompt.trim() || undefined }));
              setOpen(false);
              toast.message("Revising in Stitch…", {
                description: "This takes 1–3 minutes. A new version will appear.",
              });
            }}
          >
            {running ? <Spinner /> : <Wand2 />} Publish new version
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
