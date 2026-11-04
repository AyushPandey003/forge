// Design studio: describe a screen (or pick a spec), and Forge generates it in
// Google Stitch under the Katalyst design system, then publishes it as a
// reviewable document. Built from Katalyst components.
import { useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Monitor, Smartphone, Tablet, Wand2, FileText, ArrowRight } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Textarea } from "@kalvi-education/katalyst/components/textarea";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@kalvi-education/katalyst/components/card";
import { Alert, AlertDescription, AlertTitle } from "@kalvi-education/katalyst/components/alert";
import { Spinner } from "@kalvi-education/katalyst/components/spinner";
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@kalvi-education/katalyst/components/toggle-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@kalvi-education/katalyst/components/select";
import { Page } from "@/components/layout/page";
import { useTitle } from "@/lib/use-title";
import { api, docID, docTitle } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { forgeApi, useForgeJob, type DeviceType } from "@/lib/forge";
import { solutionsApi } from "@/lib/solutions";

const STARTERS = [
  "A sprint planning board with columns for Backlog, In progress, In review and Done, with assignee avatars and story points.",
  "An onboarding flow for a new engineer: welcome, connect GitHub, pick a squad, and a checklist of first-week tasks.",
  "A release notes page with a version timeline, grouped changes (Features, Fixes, Breaking) and a subscribe form.",
  "A settings page for team members and roles, with an invite dialog and a pending invites table.",
];

const NO_SPEC = "__none__";

export function DesignStudioPage() {
  useTitle("Design studio");
  const nav = useNavigate();
  const qc = useQueryClient();
  const [params] = useSearchParams();
  const [prompt, setPrompt] = useState("");
  const [title, setTitle] = useState("");
  const [device, setDevice] = useState<DeviceType>("DESKTOP");
  const [specId, setSpecId] = useState<string>(params.get("spec") || NO_SPEC);
  const solutionId = params.get("solution") || undefined;
  const solutionStage = params.get("stage") || undefined;
  const board = useQuery({
    queryKey: ["forge", "solutions", "board", solutionId || ""],
    queryFn: () => solutionsApi.board(solutionId!),
    enabled: !!solutionId,
  });

  const status = useQuery({ queryKey: ["forge", "status"], queryFn: forgeApi.status });
  const docs = useQuery({
    queryKey: [...queryKeys.documents(), ""],
    queryFn: () => api.listDocuments(),
  });
  const specs = useMemo(
    () => (docs.data?.items || []).filter((d) => !(d.labels || []).includes("design")),
    [docs.data],
  );

  const { job, running, start, reset, startError } = useForgeJob((j) => {
    if (j.status === "done" && j.document_id) {
      qc.invalidateQueries({ queryKey: queryKeys.documents() });
    }
  });

  const canGenerate =
    status.data?.enabled &&
    !running &&
    (prompt.trim().length > 0 || specId !== NO_SPEC || !!solutionId);

  function generate(text = prompt) {
    const p =
      text.trim() ||
      "Design the primary screen described in this product spec, following the Katalyst design system.";
    start(() =>
      forgeApi.createDesign({
        prompt: p,
        title: title.trim() || undefined,
        device_type: device,
        source_doc_id: specId !== NO_SPEC ? specId : undefined,
        solution_id: solutionId,
        stage: solutionStage,
      }),
    );
  }

  return (
    <Page lane="focus">
      <header className="enter mb-8">
        <div className="mb-3 flex items-center gap-2">
          <Badge variant="outline">Stitch</Badge>
          <Badge variant="secondary">{status.data?.design_system || "Katalyst"}</Badge>
        </div>
        <h1 className="text-2xl font-semibold tracking-tight">Design studio</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Describe a screen or start from a spec. Forge generates it with Google Stitch,
          constrained by the Katalyst design system, and publishes it for your team to
          review.
        </p>
      </header>

      {board.data && (
        <Alert className="mb-6">
          <AlertTitle>
            For {board.data.solution.key} · {board.data.solution.title}
          </AlertTitle>
          <AlertDescription>
            The design is added to this solution's UX designs. Leave the prompt empty to
            design from its PRD.{" "}
            <Link className="underline" to={`/app/solutions/${board.data.solution.key}`}>
              Back to solution
            </Link>
          </AlertDescription>
        </Alert>
      )}

      {status.data && !status.data.enabled && (
        <Alert className="mb-6">
          <AlertTitle>Stitch is not configured</AlertTitle>
          <AlertDescription>
            Set <code>STITCH_API_KEY</code> (or <code>STITCH_USE_GCLOUD=true</code>) on the
            server to enable design generation.
          </AlertDescription>
        </Alert>
      )}

      <Card className="enter">
        <CardContent className="flex flex-col gap-5">
          <div className="flex flex-col gap-2">
            <label htmlFor="forge-prompt" className="text-sm font-medium">
              What should we design?
            </label>
            <Textarea
              id="forge-prompt"
              rows={5}
              value={prompt}
              disabled={running}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="A dashboard for tracking spec reviews: a table of specs with owners and status, and a panel of open comments…"
              onKeyDown={(e) => {
                if ((e.metaKey || e.ctrlKey) && e.key === "Enter" && canGenerate) generate();
              }}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <span className="text-sm font-medium">Start from a spec</span>
              <Select value={specId} onValueChange={setSpecId} disabled={running}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="No spec" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NO_SPEC}>No spec</SelectItem>
                  {specs.map((d) => (
                    <SelectItem key={docID(d)} value={docID(d)}>
                      {docTitle(d)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <label htmlFor="forge-title" className="text-sm font-medium">
                Title <span className="font-normal text-muted-foreground">(optional)</span>
              </label>
              <Input
                id="forge-title"
                value={title}
                disabled={running}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Named by Stitch if empty"
              />
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <ToggleGroup
              type="single"
              variant="outline"
              value={device}
              onValueChange={(v) => v && setDevice(v as DeviceType)}
              disabled={running}
              aria-label="Device"
            >
              <ToggleGroupItem value="DESKTOP" aria-label="Desktop">
                <Monitor /> Desktop
              </ToggleGroupItem>
              <ToggleGroupItem value="TABLET" aria-label="Tablet">
                <Tablet /> Tablet
              </ToggleGroupItem>
              <ToggleGroupItem value="MOBILE" aria-label="Mobile">
                <Smartphone /> Mobile
              </ToggleGroupItem>
            </ToggleGroup>
            <Button size="lg" disabled={!canGenerate} onClick={() => generate()}>
              {running ? <Spinner /> : <Wand2 />}
              {running ? "Generating…" : "Generate design"}
            </Button>
          </div>
        </CardContent>
      </Card>

      {(job || startError) && (
        <div className="mt-6">
          <JobPanel
            job={job}
            startError={startError}
            onOpen={(id) => nav(`/app/d/${id}`)}
            onAnswer={(s) => {
              setPrompt(s);
              reset();
              generate(s);
            }}
            onReset={reset}
          />
        </div>
      )}

      {!job && (
        <section className="mt-10">
          <h2 className="mb-3 text-sm font-medium text-muted-foreground">Try a starter</h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {STARTERS.map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => setPrompt(s)}
                className="rounded-lg border bg-card p-4 text-left text-sm text-card-foreground transition-colors hover:bg-muted"
              >
                {s}
              </button>
            ))}
          </div>
        </section>
      )}

      <p className="mt-10 text-xs text-muted-foreground">
        Designs are published as documents labelled <code>design</code>. Comment on them,
        then use <span className="font-medium text-foreground">Revise with Stitch</span> on
        the document to apply the feedback as a new version.{" "}
        <Link to="/app/documents" className="underline underline-offset-4">
          Back to documents
        </Link>
      </p>
    </Page>
  );
}

function JobPanel({
  job,
  startError,
  onOpen,
  onAnswer,
  onReset,
}: {
  job: ReturnType<typeof useForgeJob>["job"];
  startError: string | null;
  onOpen: (docId: string) => void;
  onAnswer: (s: string) => void;
  onReset: () => void;
}) {
  if (startError || job?.status === "error") {
    return (
      <Alert variant="destructive">
        <AlertTitle>Generation failed</AlertTitle>
        <AlertDescription>
          {startError || job?.error}
          <div className="mt-3">
            <Button variant="outline" size="sm" onClick={onReset}>
              Try again
            </Button>
          </div>
        </AlertDescription>
      </Alert>
    );
  }
  if (!job) return null;
  if (job.status === "needs_input") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Stitch has a question</CardTitle>
          <CardDescription className="whitespace-pre-wrap">{job.question}</CardDescription>
        </CardHeader>
        {!!job.suggestions?.length && (
          <CardContent className="flex flex-wrap gap-2">
            {job.suggestions.map((s) => (
              <Button key={s} variant="outline" size="sm" onClick={() => onAnswer(s)}>
                {s}
              </Button>
            ))}
          </CardContent>
        )}
      </Card>
    );
  }
  if (job.status === "done" && job.document_id) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Design published</CardTitle>
          <CardDescription>
            It's a document now: share it, comment on it, and revise it.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Button onClick={() => onOpen(job.document_id!)}>
            <FileText /> Open design <ArrowRight />
          </Button>
          <Button variant="outline" onClick={onReset}>
            Generate another
          </Button>
        </CardContent>
      </Card>
    );
  }
  return (
    <Card>
      <CardContent className="flex items-center gap-3">
        <Spinner className="size-5" />
        <div>
          <div className="text-sm font-medium">{job.stage}</div>
          <div className="text-xs text-muted-foreground">
            Stitch usually takes 1–3 minutes. You can leave this page; the design will
            appear in your documents.
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
