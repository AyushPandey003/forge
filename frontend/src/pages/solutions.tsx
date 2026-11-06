// Solutions: every piece of work, from problem statement to release.
import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowRight, Inbox, Plus, Workflow as WorkflowIcon } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Textarea } from "@kalvi-education/katalyst/components/textarea";
import { Progress } from "@kalvi-education/katalyst/components/progress";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@kalvi-education/katalyst/components/dialog";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@kalvi-education/katalyst/components/empty";
import { Tabs, TabsList, TabsTrigger } from "@kalvi-education/katalyst/components/tabs";
import { Page } from "@/components/layout/page";
import { useForgeLive } from "@/lib/use-forge-live";
import { useTitle } from "@/lib/use-title";
import {
  ROLE_LABEL,
  solutionKeys,
  solutionsApi,
  type SolutionListItem,
} from "@/lib/solutions";
import { formatTimestamp } from "@/lib/format-date";

export function SolutionsPage() {
  useForgeLive();
  useTitle("Solutions");
  const [status, setStatus] = useState("active");
  const [creating, setCreating] = useState(false);
  const list = useQuery({
    queryKey: solutionKeys.list(status),
    queryFn: () => solutionsApi.list(status === "all" ? undefined : status),
  });
  const inbox = useQuery({ queryKey: solutionKeys.inbox, queryFn: solutionsApi.inbox });
  const items = list.data?.items || [];
  const waiting = inbox.data?.items || [];

  return (
    <Page>
      <header className="enter mb-8 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Solutions</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Every piece of work, from problem statement to release, with approvals at
            each stage.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link to="/app/settings/workflows">
              <WorkflowIcon /> Workflows
            </Link>
          </Button>
          <Button onClick={() => setCreating(true)}>
            <Plus /> New solution
          </Button>
        </div>
      </header>

      {waiting.length > 0 && (
        <section className="enter mb-8 rounded-xl border bg-card p-4">
          <div className="mb-3 flex items-center gap-2 text-sm font-medium">
            <Inbox className="size-4" /> Waiting on you
            <Badge variant="secondary">{waiting.length}</Badge>
          </div>
          <ul className="divide-y">
            {waiting.map((w) => (
              <li key={`${w.solution_id}-${w.stage}-${w.gate}`}>
                <Link
                  to={`/app/solutions/${w.solution_key}?stage=${w.stage}`}
                  className="flex items-center justify-between gap-4 py-2.5 text-sm hover:text-foreground"
                >
                  <span className="min-w-0 truncate">
                    <span className="font-mono text-xs text-muted-foreground">
                      {w.solution_key}
                    </span>{" "}
                    <span className="font-medium">{w.title}</span>
                    <span className="text-muted-foreground">
                      {" "}
                      · {w.stage_label} · {w.gate_label}
                    </span>
                  </span>
                  <span className="flex shrink-0 items-center gap-2">
                    {w.reason === "stale" ? (
                      <Badge variant="outline">Changed since your review</Badge>
                    ) : (
                      <Badge>Your approval</Badge>
                    )}
                    <ArrowRight className="size-4 text-muted-foreground" />
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="enter mb-4 flex items-center justify-between">
        <Tabs value={status} onValueChange={setStatus}>
          <TabsList>
            <TabsTrigger value="active">Active</TabsTrigger>
            <TabsTrigger value="shipped">Shipped</TabsTrigger>
            <TabsTrigger value="archived">Archived</TabsTrigger>
            <TabsTrigger value="all">All</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {list.isLoading ? (
        <div className="h-40 animate-pulse rounded-xl bg-muted" />
      ) : items.length === 0 ? (
        <Empty className="rounded-xl border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <WorkflowIcon />
            </EmptyMedia>
            <EmptyTitle>No solutions yet</EmptyTitle>
            <EmptyDescription>
              Start with the problem. Forge walks it through PRD, implementation and
              release, with the right people signing off at each step.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button onClick={() => setCreating(true)}>
              <Plus /> New solution
            </Button>
          </EmptyContent>
        </Empty>
      ) : (
        <div className="grid gap-3">
          {items.map((s) => (
            <SolutionRow key={s.id} s={s} />
          ))}
        </div>
      )}

      <CreateSolutionDialog open={creating} onOpenChange={setCreating} />
    </Page>
  );
}

function SolutionRow({ s }: { s: SolutionListItem }) {
  const pct = useMemo(
    () =>
      s.status === "shipped"
        ? 100
        : Math.round((Math.max(s.stage_index, 0) / Math.max(s.stage_count, 1)) * 100),
    [s],
  );
  return (
    <Link
      to={`/app/solutions/${s.key}`}
      className="group grid gap-3 rounded-xl border bg-card p-4 transition-colors hover:bg-muted/50 sm:grid-cols-[1fr_220px]"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs text-muted-foreground">{s.key}</span>
          {s.status !== "active" && <Badge variant="secondary">{s.status}</Badge>}
          {(s.roles || []).slice(0, 2).map((r) => (
            <Badge key={r} variant="outline">
              {ROLE_LABEL[r] || r}
            </Badge>
          ))}
        </div>
        <div className="mt-1 truncate font-medium">{s.title}</div>
        {s.summary && (
          <div className="mt-0.5 line-clamp-1 text-sm text-muted-foreground">{s.summary}</div>
        )}
      </div>
      <div className="flex flex-col justify-center gap-1.5">
        <div className="flex items-center justify-between text-xs">
          <span className="font-medium">
            {s.status === "shipped" ? "Shipped" : s.stage_label}
          </span>
          <span className="text-muted-foreground">{formatTimestamp(s.updated_at)}</span>
        </div>
        <Progress value={pct} />
      </div>
    </Link>
  );
}

function CreateSolutionDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const nav = useNavigate();
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const create = useMutation({
    mutationFn: () => solutionsApi.create({ title, summary }),
    onSuccess: (s) => {
      qc.invalidateQueries({ queryKey: solutionKeys.all });
      onOpenChange(false);
      setTitle("");
      setSummary("");
      nav(`/app/solutions/${s.key}`);
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Could not create"),
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New solution</DialogTitle>
          <DialogDescription>
            Name the problem space. You'll write the problem statement next.
          </DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (title.trim()) create.mutate();
          }}
        >
          <div className="flex flex-col gap-2">
            <label htmlFor="sol-title" className="text-sm font-medium">
              Title
            </label>
            <Input
              id="sol-title"
              autoFocus
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g. Spec reviews take too long"
            />
          </div>
          <div className="flex flex-col gap-2">
            <label htmlFor="sol-summary" className="text-sm font-medium">
              Summary <span className="font-normal text-muted-foreground">(optional)</span>
            </label>
            <Textarea
              id="sol-summary"
              rows={3}
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={!title.trim() || create.isPending}>
              Create solution
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
