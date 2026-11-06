// Solution board: stages, artifacts, approval gates, members and timeline.
import { useEffect, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  AlertTriangle,
  ArrowRight,
  Check,
  CircleDashed,
  ExternalLink,
  FileText,
  Link2,
  MoreHorizontal,
  PenLine,
  Plus,
  ShieldCheck,
  Trash2,
  UserPlus,
  Wand2,
  X,
} from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Textarea } from "@kalvi-education/katalyst/components/textarea";
import { Progress } from "@kalvi-education/katalyst/components/progress";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@kalvi-education/katalyst/components/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@kalvi-education/katalyst/components/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@kalvi-education/katalyst/components/dropdown-menu";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@kalvi-education/katalyst/components/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@kalvi-education/katalyst/components/tabs";
import { Page } from "@/components/layout/page";
import { Avatar } from "@/components/ui/misc";
import { useForgeLive } from "@/lib/use-forge-live";
import { useTitle } from "@/lib/use-title";
import { cn } from "@/lib/cn";
import { formatTimestamp } from "@/lib/format-date";
import {
  ROLE_LABEL,
  solutionKeys,
  solutionsApi,
  userLabel,
  type Artifact,
  type Board,
  type ForgeEvent,
  type GateStatus,
  type Slot,
  type StageStatus,
  type User,
} from "@/lib/solutions";
import { useMe } from "@/lib/hooks";

export function SolutionBoardPage() {
  useForgeLive();
  const { key = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const board = useQuery({
    queryKey: solutionKeys.board(key),
    queryFn: () => solutionsApi.board(key),
  });
  useTitle(board.data ? `${board.data.solution.key} · ${board.data.solution.title}` : "Solution");
  const b = board.data;
  const selected = params.get("stage") || b?.solution.current_stage || "";
  const stage = b?.stages.find((s) => s.id === selected) || b?.stages.find((s) => s.state === "current");

  if (board.isLoading) {
    return (
      <Page>
        <div className="h-64 animate-pulse rounded-xl bg-muted" />
      </Page>
    );
  }
  if (!b || !stage) {
    return (
      <Page lane="focus">
        <p className="text-sm text-muted-foreground">
          Solution not found, or you don't have access.{" "}
          <Link to="/app/solutions" className="underline">
            Back to solutions
          </Link>
        </p>
      </Page>
    );
  }

  return (
    <Page>
      <SolutionHeader b={b} />
      <StageStepper
        b={b}
        selected={stage.id}
        onSelect={(id) => setParams(id === b.solution.current_stage ? {} : { stage: id })}
      />
      <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="flex min-w-0 flex-col gap-6">
          <StagePanel b={b} stage={stage} />
        </div>
        <aside className="flex flex-col gap-6">
          <MembersCard b={b} />
          <TimelineCard solutionId={b.solution.id} solutionKey={b.solution.key} />
        </aside>
      </div>
    </Page>
  );
}

// ---- Header ------------------------------------------------------------------

function SolutionHeader({ b }: { b: Board }) {
  const qc = useQueryClient();
  const s = b.solution;
  const [editing, setEditing] = useState(false);
  const patch = useMutation({
    mutationFn: (p: Parameters<typeof solutionsApi.patch>[1]) => solutionsApi.patch(s.id, p),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: solutionKeys.all });
      setEditing(false);
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Update failed"),
  });
  return (
    <header className="enter mb-6">
      <div className="mb-2 flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/app/solutions" className="hover:text-foreground">
          Solutions
        </Link>
        <span>/</span>
        <span className="font-mono text-xs">{s.key}</span>
        {s.status !== "active" && <Badge variant="secondary">{s.status}</Badge>}
      </div>
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight">{s.title}</h1>
          {s.summary && <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{s.summary}</p>}
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <div className="hidden w-40 sm:block">
            <div className="mb-1 flex justify-between text-xs text-muted-foreground">
              <span>Progress</span>
              <span>{Math.round(b.progress * 100)}%</span>
            </div>
            <Progress value={b.progress * 100} />
          </div>
          {b.can_manage && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon" aria-label="Solution actions">
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => setEditing(true)}>
                  <PenLine /> Edit details
                </DropdownMenuItem>
                {s.folder_id && (
                  <DropdownMenuItem asChild>
                    <Link to={`/app/folders/${s.folder_id}`}>
                      <FileText /> Open documents folder
                    </Link>
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                {s.status !== "archived" ? (
                  <DropdownMenuItem onSelect={() => patch.mutate({ status: "archived" })}>
                    Archive
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem onSelect={() => patch.mutate({ status: "active" })}>
                    Restore
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </div>
      <EditDetailsDialog
        open={editing}
        onOpenChange={setEditing}
        title={s.title}
        summary={s.summary}
        pending={patch.isPending}
        onSave={(title, summary) => patch.mutate({ title, summary })}
      />
    </header>
  );
}

function EditDetailsDialog(props: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  title: string;
  summary: string;
  pending: boolean;
  onSave: (title: string, summary: string) => void;
}) {
  const [title, setTitle] = useState(props.title);
  const [summary, setSummary] = useState(props.summary);
  useEffect(() => {
    if (props.open) {
      setTitle(props.title);
      setSummary(props.summary);
    }
  }, [props.open, props.title, props.summary]);
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit solution</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Input value={title} onChange={(e) => setTitle(e.target.value)} aria-label="Title" />
          <Textarea rows={3} value={summary} onChange={(e) => setSummary(e.target.value)} aria-label="Summary" />
        </div>
        <DialogFooter>
          <Button disabled={!title.trim() || props.pending} onClick={() => props.onSave(title, summary)}>
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---- Stepper -----------------------------------------------------------------

function StageStepper({
  b,
  selected,
  onSelect,
}: {
  b: Board;
  selected: string;
  onSelect: (id: string) => void;
}) {
  return (
    <nav aria-label="Stages" className="enter overflow-x-auto">
      <ol className="flex min-w-max items-stretch gap-2">
        {b.stages.map((st, i) => {
          const gates = st.gate_status || [];
          const approved = gates.filter((g) => g.state === "approved" || g.state === "overridden").length;
          const shipped = b.solution.status === "shipped";
          const done = st.state === "done" || shipped;
          return (
            <li key={st.id} className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => onSelect(st.id)}
                aria-current={st.id === selected ? "step" : undefined}
                className={cn(
                  "flex min-w-44 flex-col items-start gap-1 rounded-lg border px-3 py-2 text-left transition-colors",
                  st.id === selected ? "border-foreground/40 bg-card" : "bg-transparent hover:bg-muted/60",
                )}
              >
                <span className="flex items-center gap-2 text-sm font-medium">
                  <StageDot state={done ? "done" : st.state} ready={st.ready} />
                  {st.label}
                </span>
                <span className="text-xs text-muted-foreground">
                  {done
                    ? "Complete"
                    : st.state === "current"
                      ? `${approved}/${gates.length} approvals${st.missing?.length ? ` · ${st.missing.length} missing` : ""}`
                      : "Upcoming"}
                </span>
              </button>
              {i < b.stages.length - 1 && <ArrowRight className="size-4 shrink-0 text-muted-foreground" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

function StageDot({ state, ready }: { state: string; ready?: boolean }) {
  if (state === "done")
    return (
      <span className="flex size-4 items-center justify-center rounded-full bg-primary text-primary-foreground">
        <Check className="size-3" />
      </span>
    );
  if (state === "current")
    return (
      <span
        className={cn(
          "size-4 rounded-full border-2",
          ready ? "border-emerald-500 bg-emerald-500/20" : "border-foreground",
        )}
      />
    );
  return <CircleDashed className="size-4 text-muted-foreground" />;
}

// ---- Stage panel -------------------------------------------------------------

function StagePanel({ b, stage }: { b: Board; stage: StageStatus }) {
  const qc = useQueryClient();
  const s = b.solution;
  const isCurrent = stage.state === "current";
  const isLast = stage.index === b.stages.length - 1;
  const advance = useMutation({
    mutationFn: () => solutionsApi.advance(s.id),
    onSuccess: (nb) => {
      qc.setQueryData(solutionKeys.board(s.key), nb);
      qc.invalidateQueries({ queryKey: solutionKeys.all });
      toast.success(isLast ? "Solution shipped 🎉" : `Moved to ${nb.stages.find((x) => x.state === "current")?.label}`);
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not advance"),
  });
  const [moveOpen, setMoveOpen] = useState(false);

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <CardTitle className="flex items-center gap-2">
                {stage.label}
                {stage.state === "done" && <Badge variant="secondary">Complete</Badge>}
                {stage.state === "upcoming" && <Badge variant="outline">Upcoming</Badge>}
                {isCurrent && stage.ready && <Badge>Ready</Badge>}
              </CardTitle>
              {stage.description && <CardDescription>{stage.description}</CardDescription>}
            </div>
            <div className="flex items-center gap-2">
              {b.can_manage && stage.id !== s.current_stage && (
                <Button variant="outline" size="sm" onClick={() => setMoveOpen(true)}>
                  Move solution here
                </Button>
              )}
              {isCurrent && b.can_manage && s.status !== "shipped" && (
                <Button
                  size="sm"
                  disabled={!stage.ready || advance.isPending}
                  title={stage.ready ? "" : "Complete required artifacts and approvals first"}
                  onClick={() => advance.mutate()}
                >
                  {isLast ? "Mark shipped" : "Advance"} <ArrowRight />
                </Button>
              )}
            </div>
          </div>
          {isCurrent && !!stage.missing?.length && (
            <p className="mt-2 flex items-center gap-1.5 text-xs text-muted-foreground">
              <AlertTriangle className="size-3.5" /> Missing: {stage.missing.join(", ")}
            </p>
          )}
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {stage.slots.map((slot) => (
            <SlotRow key={slot.kind} b={b} stage={stage} slot={slot} />
          ))}
        </CardContent>
      </Card>

      {(stage.gate_status || []).map((g) => (
        <GateCard key={g.id} b={b} stage={stage} gate={g} />
      ))}

      <MoveStageDialog
        open={moveOpen}
        onOpenChange={setMoveOpen}
        solutionId={s.id}
        solutionKey={s.key}
        stage={stage}
      />
    </>
  );
}

function MoveStageDialog({
  open,
  onOpenChange,
  solutionId,
  solutionKey,
  stage,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  solutionId: string;
  solutionKey: string;
  stage: StageStatus;
}) {
  const qc = useQueryClient();
  const [reason, setReason] = useState("");
  const move = useMutation({
    mutationFn: () => solutionsApi.setStage(solutionId, stage.id, reason),
    onSuccess: (nb) => {
      qc.setQueryData(solutionKeys.board(solutionKey), nb);
      qc.invalidateQueries({ queryKey: solutionKeys.all });
      onOpenChange(false);
      setReason("");
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not move"),
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Move to {stage.label}?</DialogTitle>
          <DialogDescription>
            This changes the solution's current stage. It is recorded in the timeline with your
            reason.
          </DialogDescription>
        </DialogHeader>
        <Textarea
          rows={3}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="Why? e.g. Implementation review found a gap in the PRD."
        />
        <DialogFooter>
          <Button disabled={!reason.trim() || move.isPending} onClick={() => move.mutate()}>
            Move solution
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---- Artifact slots ----------------------------------------------------------

function SlotRow({ b, stage, slot }: { b: Board; stage: StageStatus; slot: Slot }) {
  const qc = useQueryClient();
  const s = b.solution;
  const items = slot.items || [];
  const canAdd = slot.multiple || items.length === 0;
  const [linkOpen, setLinkOpen] = useState(false);
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: solutionKeys.board(s.key) });
    qc.invalidateQueries({ queryKey: solutionKeys.events(s.id) });
  };
  const create = useMutation({
    mutationFn: () => solutionsApi.addArtifact(s.id, { stage: stage.id, kind: slot.kind, create: true }),
    onSuccess: (a) => {
      invalidate();
      if (a.document_id) window.location.assign(`/app/edit/${a.document_id}`);
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not create"),
  });
  const remove = useMutation({
    mutationFn: (id: string) => solutionsApi.removeArtifact(s.id, id),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not remove"),
  });

  return (
    <div className="rounded-lg border p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium">
            {slot.label}
            {slot.required && <span className="text-xs font-normal text-muted-foreground">Required</span>}
          </div>
          {slot.description && <div className="text-xs text-muted-foreground">{slot.description}</div>}
        </div>
        {canAdd && stage.state !== "done" && (
          <div className="flex gap-1.5">
            {slot.kind === "design" ? (
              <Button variant="outline" size="sm" asChild>
                <Link to={`/app/design?solution=${s.id}&stage=${stage.id}`}>
                  <Wand2 /> Generate
                </Link>
              </Button>
            ) : slot.external ? (
              <Button variant="outline" size="sm" onClick={() => setLinkOpen(true)}>
                <Link2 /> Add link
              </Button>
            ) : (
              <Button variant="outline" size="sm" disabled={create.isPending} onClick={() => create.mutate()}>
                <Plus /> {slot.template ? "Write" : "New doc"}
              </Button>
            )}
            {!slot.external && (
              <Button variant="ghost" size="sm" onClick={() => setLinkOpen(true)} title="Attach an existing document">
                Attach
              </Button>
            )}
          </div>
        )}
      </div>
      {items.length > 0 && (
        <ul className="mt-3 flex flex-col gap-1.5">
          {items.map((a) => (
            <ArtifactItem
              key={a.id}
              a={a}
              canRemove={b.can_manage}
              onRemove={() => remove.mutate(a.id)}
            />
          ))}
        </ul>
      )}
      <AttachDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        external={!!slot.external}
        label={slot.label}
        onSubmit={async (v) => {
          try {
            await solutionsApi.addArtifact(s.id, {
              stage: stage.id,
              kind: slot.kind,
              title: v.title,
              ...(slot.external ? { url: v.value } : { document_id: parseDocID(v.value) }),
            });
            invalidate();
            setLinkOpen(false);
          } catch (e) {
            toast.error(e instanceof Error ? e.message : "Could not attach");
          }
        }}
      />
    </div>
  );
}

function parseDocID(v: string) {
  const m = v.match(/doc_[a-z0-9]+/);
  return m ? m[0] : v.trim();
}

function ArtifactItem({ a, canRemove, onRemove }: { a: Artifact; canRemove: boolean; onRemove: () => void }) {
  const isDoc = !!a.document_id;
  return (
    <li className="group flex items-center justify-between gap-2 rounded-md bg-muted/50 px-2.5 py-1.5 text-sm">
      {isDoc ? (
        <Link to={`/app/d/${a.document_id}`} className="flex min-w-0 items-center gap-2 hover:underline">
          <FileText className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{a.title}</span>
          {!!a.version_number && <span className="shrink-0 text-xs text-muted-foreground">v{a.version_number}</span>}
        </Link>
      ) : (
        <a href={a.url} target="_blank" rel="noreferrer" className="flex min-w-0 items-center gap-2 hover:underline">
          <ExternalLink className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{a.title || a.url}</span>
        </a>
      )}
      <span className="flex shrink-0 items-center gap-1">
        {isDoc && (
          <Button variant="ghost" size="icon-xs" asChild title="Edit">
            <Link to={`/app/edit/${a.document_id}`}>
              <PenLine />
            </Link>
          </Button>
        )}
        {canRemove && (
          <Button
            variant="ghost"
            size="icon-xs"
            className="opacity-0 group-hover:opacity-100"
            title="Remove from solution"
            onClick={onRemove}
          >
            <Trash2 />
          </Button>
        )}
      </span>
    </li>
  );
}

function AttachDialog({
  open,
  onOpenChange,
  external,
  label,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  external: boolean;
  label: string;
  onSubmit: (v: { value: string; title: string }) => void;
}) {
  const [value, setValue] = useState("");
  const [title, setTitle] = useState("");
  useEffect(() => {
    if (open) {
      setValue("");
      setTitle("");
    }
  }, [open]);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{external ? `Add ${label}` : `Attach a document to ${label}`}</DialogTitle>
          <DialogDescription>
            {external ? "Paste a URL." : "Paste a document link or ID (doc_…)."}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Input
            autoFocus
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder={external ? "https://…" : "http://localhost:8080/app/d/doc_…"}
          />
          <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Title (optional)" />
        </div>
        <DialogFooter>
          <Button disabled={!value.trim()} onClick={() => onSubmit({ value, title })}>
            Add
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---- Gates -------------------------------------------------------------------

const GATE_STATE: Record<GateStatus["state"], { label: string; variant: "default" | "secondary" | "outline" | "destructive" }> = {
  pending: { label: "Awaiting approval", variant: "outline" },
  approved: { label: "Approved", variant: "default" },
  overridden: { label: "Overridden", variant: "secondary" },
  changes_requested: { label: "Changes requested", variant: "destructive" },
  rejected: { label: "Rejected", variant: "destructive" },
};

const DECISION_LABEL: Record<string, string> = {
  approved: "approved",
  changes_requested: "requested changes",
  rejected: "rejected",
};

function GateCard({ b, stage, gate }: { b: Board; stage: StageStatus; gate: GateStatus }) {
  const qc = useQueryClient();
  const s = b.solution;
  const [mode, setMode] = useState<null | "changes_requested" | "rejected" | "override">(null);
  const [comment, setComment] = useState("");
  const decide = useMutation({
    mutationFn: (input: { decision: string; comment?: string; override?: boolean }) =>
      solutionsApi.decide(s.id, stage.id, gate.id, input),
    onSuccess: (nb) => {
      qc.setQueryData(solutionKeys.board(s.key), nb);
      qc.invalidateQueries({ queryKey: solutionKeys.events(s.id) });
      qc.invalidateQueries({ queryKey: solutionKeys.inbox });
      setMode(null);
      setComment("");
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not record decision"),
  });
  const st = GATE_STATE[gate.state];
  const decisions = gate.decisions || [];
  const approvers = gate.approvers || [];
  const me = useMe();
  const myID = (me.data?.user || me.data?.principal)?.id;
  const mine = decisions.find((d) => d.user.id === myID);
  const active = stage.state === "current";

  return (
    <Card size="sm">
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2 text-sm">
              <ShieldCheck className="size-4" /> {gate.label}
            </CardTitle>
            <CardDescription>
              “{gate.question}” · {policyText(gate)} ·{" "}
              {gate.approver_roles.map((r) => ROLE_LABEL[r] || r).join(", ")}
            </CardDescription>
          </div>
          <Badge variant={st.variant}>
            {st.label}
            {gate.state === "pending" && gate.required > 1 ? ` · ${gate.approved}/${gate.required}` : ""}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {approvers.length === 0 && (
          <p className="text-xs text-muted-foreground">
            No one holds an approver role for this gate yet. Add one in Members.
          </p>
        )}
        {decisions.length > 0 && (
          <ul className="flex flex-col gap-2">
            {decisions.map((d) => (
              <li key={d.id} className="flex gap-2.5 text-sm">
                <Avatar src={d.user.picture_url} name={userLabel(d.user)} />
                <div className="min-w-0">
                  <div>
                    <span className="font-medium">{userLabel(d.user)}</span>{" "}
                    <span className="text-muted-foreground">
                      {d.override ? "overrode the gate" : DECISION_LABEL[d.decision]} ·{" "}
                      {formatTimestamp(d.created_at)}
                    </span>
                    {d.stale && (
                      <Badge variant="outline" className="ml-2" title={(d.stale_reasons || []).join(", ")}>
                        {d.counts ? "Changed since" : "Outdated"}
                      </Badge>
                    )}
                  </div>
                  {d.comment && <p className="whitespace-pre-wrap text-muted-foreground">{d.comment}</p>}
                  {d.stale && !!d.stale_reasons?.length && (
                    <p className="text-xs text-muted-foreground">{d.stale_reasons.join(" · ")}</p>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}

        {active &&
          (gate.can_decide ||
            (b.can_manage && gate.allow_override && gate.state !== "approved" && gate.state !== "overridden")) && (
          <div className="flex flex-col gap-2 border-t pt-3">
            {mode && (
              <Textarea
                rows={3}
                autoFocus
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                placeholder={
                  mode === "override"
                    ? "Reason for overriding (recorded in the timeline)"
                    : "What needs to change?"
                }
              />
            )}
            <div className="flex flex-wrap gap-2">
              {gate.can_decide && !mode && (
                <>
                  <Button
                    size="sm"
                    disabled={decide.isPending}
                    onClick={() => decide.mutate({ decision: "approved", comment: "" })}
                  >
                    <Check /> {mine?.decision === "approved" && mine.stale ? "Re-approve" : "Approve"}
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setMode("changes_requested")}>
                    Request changes
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => setMode("rejected")}>
                    Reject
                  </Button>
                </>
              )}
              {!gate.can_decide && !mode && b.can_manage && gate.allow_override && gate.state !== "approved" && gate.state !== "overridden" && (
                <Button size="sm" variant="ghost" onClick={() => setMode("override")}>
                  Override gate
                </Button>
              )}
              {mode && (
                <>
                  <Button
                    size="sm"
                    variant={mode === "override" ? "default" : "destructive"}
                    disabled={!comment.trim() || decide.isPending}
                    onClick={() =>
                      decide.mutate(
                        mode === "override"
                          ? { decision: "approved", comment, override: true }
                          : { decision: mode, comment },
                      )
                    }
                  >
                    {mode === "override" ? "Override" : mode === "rejected" ? "Reject" : "Request changes"}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => setMode(null)}>
                    <X /> Cancel
                  </Button>
                </>
              )}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function policyText(g: GateStatus) {
  if (g.policy === "all") return "everyone must approve";
  if (g.policy === "n") return `${g.min_approvals} approvals needed`;
  return "one approval needed";
}

// ---- Members -----------------------------------------------------------------

function MembersCard({ b }: { b: Board }) {
  const qc = useQueryClient();
  const s = b.solution;
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("solution_approver");
  const [adding, setAdding] = useState(false);
  const meta = useQuery({ queryKey: solutionKeys.meta, queryFn: solutionsApi.meta, staleTime: Infinity });
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: solutionKeys.board(s.key) });
    qc.invalidateQueries({ queryKey: solutionKeys.events(s.id) });
  };
  const add = useMutation({
    mutationFn: () => solutionsApi.addMember(s.id, email.trim(), role),
    onSuccess: () => {
      invalidate();
      setEmail("");
      setAdding(false);
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not add member"),
  });
  const remove = useMutation({
    mutationFn: (m: { id: string; role: string }) => solutionsApi.removeMember(s.id, m.id, m.role),
    onSuccess: invalidate,
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not remove"),
  });

  // Group roles per user.
  const people = useMemo(() => {
    const map = new Map<string, { user: User; roles: string[] }>();
    for (const m of b.members || []) {
      const e = map.get(m.user.id) || { user: m.user, roles: [] };
      e.roles.push(m.role);
      map.set(m.user.id, e);
    }
    return [...map.values()];
  }, [b.members]);

  return (
    <Card size="sm">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm">Members</CardTitle>
          {b.can_manage && (
            <Button variant="ghost" size="icon-sm" title="Add member" onClick={() => setAdding((v) => !v)}>
              <UserPlus />
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {adding && (
          <form
            className="flex flex-col gap-2 rounded-lg border p-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (email.trim()) add.mutate();
            }}
          >
            <Input
              autoFocus
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="name@company.com"
            />
            <Select value={role} onValueChange={setRole}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(meta.data?.roles || []).map((r) => (
                  <SelectItem key={r.id} value={r.id}>
                    {r.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button size="sm" type="submit" disabled={!email.trim() || add.isPending}>
              Add
            </Button>
          </form>
        )}
        <ul className="flex flex-col gap-2.5">
          {people.map(({ user, roles }) => (
            <li key={user.id} className="flex items-start gap-2.5 text-sm">
              <Avatar src={user.picture_url} name={userLabel(user)} />
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{userLabel(user)}</div>
                <div className="mt-1 flex flex-wrap gap-1">
                  {roles.map((r) =>
                    b.can_manage ? (
                      <DropdownMenu key={r}>
                        <DropdownMenuTrigger asChild>
                          <button type="button">
                            <Badge variant="outline">{ROLE_LABEL[r] || r}</Badge>
                          </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start">
                          <DropdownMenuLabel>{ROLE_LABEL[r] || r}</DropdownMenuLabel>
                          <DropdownMenuItem
                            variant="destructive"
                            onSelect={() => remove.mutate({ id: user.id, role: r })}
                          >
                            Remove role
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    ) : (
                      <Badge key={r} variant="outline">
                        {ROLE_LABEL[r] || r}
                      </Badge>
                    ),
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}

// ---- Timeline ----------------------------------------------------------------

function TimelineCard({ solutionId }: { solutionId: string; solutionKey: string }) {
  const events = useQuery({
    queryKey: solutionKeys.events(solutionId),
    queryFn: () => solutionsApi.events(solutionId),
  });
  const items = events.data?.items || [];
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="text-sm">Timeline</CardTitle>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="all">
          <TabsList className="mb-3">
            <TabsTrigger value="all">All</TabsTrigger>
            <TabsTrigger value="decisions">Decisions</TabsTrigger>
          </TabsList>
          <TabsContent value="all">
            <EventList items={items} />
          </TabsContent>
          <TabsContent value="decisions">
            <EventList items={items.filter((e) => e.kind.startsWith("gate.") || e.kind === "stage.changed")} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}

function EventList({ items }: { items: ForgeEvent[] }) {
  if (!items.length) return <p className="text-xs text-muted-foreground">Nothing yet.</p>;
  return (
    <ol className="flex max-h-[420px] flex-col gap-3 overflow-y-auto pr-1">
      {items.map((e) => (
        <li key={e.id} className="text-xs">
          <div>
            <span className="font-medium">{userLabel(e.actor)}</span>{" "}
            <span className="text-muted-foreground">{describeEvent(e)}</span>
          </div>
          <div className="text-muted-foreground/80">{formatTimestamp(e.created_at)}</div>
        </li>
      ))}
    </ol>
  );
}

function describeEvent(e: ForgeEvent): string {
  const d = e.data as Record<string, string | number | boolean | undefined>;
  switch (e.kind) {
    case "solution.created":
      return `created the solution`;
    case "solution.updated":
      return d.status ? `set status to ${d.status}` : "edited the details";
    case "member.added":
      return `added a ${ROLE_LABEL[String(d.role)] || d.role}`;
    case "member.removed":
      return `removed a ${ROLE_LABEL[String(d.role)] || d.role}`;
    case "artifact.added":
      return `added ${d.title || d.kind}`;
    case "artifact.removed":
      return `removed ${d.title || d.kind}`;
    case "artifact.versioned":
      return `published v${d.version} of ${d.title}`;
    case "gate.approved":
      return `approved ${d.stage} · ${d.gate}`;
    case "gate.changes_requested":
      return `requested changes on ${d.stage} · ${d.gate}`;
    case "gate.rejected":
      return `rejected ${d.stage} · ${d.gate}`;
    case "gate.overridden":
      return `overrode ${d.stage} · ${d.gate}: “${d.comment}”`;
    case "stage.changed":
      return d.manual ? `moved the solution to ${d.stage}: “${d.reason}”` : `advanced to ${d.stage}`;
    default:
      return e.kind;
  }
}
