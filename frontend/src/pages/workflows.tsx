// Workflow editor: the rules every solution follows. Stages, the artifacts each
// stage expects, and the approval gates that close it. New solutions snapshot
// the workflow, so editing never changes in-flight work.
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowDown, ArrowUp, Copy, Plus, Save, Trash2 } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Textarea } from "@kalvi-education/katalyst/components/textarea";
import { Switch } from "@kalvi-education/katalyst/components/switch";
import { Checkbox } from "@kalvi-education/katalyst/components/checkbox";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@kalvi-education/katalyst/components/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@kalvi-education/katalyst/components/select";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@kalvi-education/katalyst/components/accordion";
import { Page } from "@/components/layout/page";
import { useTitle } from "@/lib/use-title";
import {
  ROLE_LABEL,
  solutionKeys,
  solutionsApi,
  type ArtifactKind,
  type Gate,
  type Stage,
  type WorkflowRecord,
} from "@/lib/solutions";

const ROLES = Object.keys(ROLE_LABEL);
const slug = (s: string) =>
  s.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "") || "item";

export function WorkflowsPage() {
  useTitle("Workflows");
  const qc = useQueryClient();
  const list = useQuery({ queryKey: solutionKeys.workflows, queryFn: solutionsApi.workflows });
  const [selected, setSelected] = useState<string>("");
  const [draft, setDraft] = useState<WorkflowRecord | null>(null);

  const items = useMemo(() => list.data?.items || [], [list.data]);
  useEffect(() => {
    if (!selected && items.length) setSelected(items[0].id);
  }, [items, selected]);
  useEffect(() => {
    const w = items.find((x) => x.id === selected);
    if (w) setDraft(structuredClone(w));
  }, [selected, list.data]); // eslint-disable-line react-hooks/exhaustive-deps

  const original = useMemo(() => items.find((x) => x.id === selected), [items, selected]);
  const dirty = !!draft && JSON.stringify(draft) !== JSON.stringify(original);

  const save = useMutation({
    mutationFn: (w: Partial<WorkflowRecord>) => solutionsApi.saveWorkflow(w),
    onSuccess: (w) => {
      qc.invalidateQueries({ queryKey: solutionKeys.workflows });
      setSelected(w.id);
      toast.success("Workflow saved. New solutions will use it.");
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : "Save failed"),
  });

  const setStages = (fn: (s: Stage[]) => Stage[]) =>
    setDraft((d) => (d ? { ...d, definition: { stages: fn(d.definition.stages) } } : d));

  return (
    <Page>
      <header className="enter mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <div className="mb-1 text-sm text-muted-foreground">
            <Link to="/app/solutions" className="hover:text-foreground">
              Solutions
            </Link>{" "}
            / Workflows
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">Workflows</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            Define the stages work moves through, what each stage needs, and who signs off.
            Solutions keep the version they started with.
          </p>
        </div>
        <div className="flex gap-2">
          {draft && (
            <Button
              variant="outline"
              onClick={() =>
                save.mutate({
                  name: `${draft.name} (copy)`,
                  description: draft.description,
                  definition: draft.definition,
                })
              }
            >
              <Copy /> Duplicate
            </Button>
          )}
          <Button disabled={!dirty || save.isPending} onClick={() => draft && save.mutate(draft)}>
            <Save /> Save changes
          </Button>
        </div>
      </header>

      <div className="grid gap-6 lg:grid-cols-[240px_1fr]">
        <aside className="flex flex-col gap-1">
          {items.map((w) => (
            <button
              key={w.id}
              type="button"
              onClick={() => setSelected(w.id)}
              className={
                "rounded-lg px-3 py-2 text-left text-sm transition-colors " +
                (w.id === selected ? "bg-muted font-medium" : "hover:bg-muted/60")
              }
            >
              <div className="flex items-center gap-2">
                {w.name}
                {w.is_default && <Badge variant="secondary">Default</Badge>}
              </div>
              <div className="text-xs text-muted-foreground">
                {w.definition.stages.map((s) => s.label).join(" → ")}
              </div>
            </button>
          ))}
        </aside>

        {draft && (
          <div className="flex min-w-0 flex-col gap-4">
            <Card>
              <CardContent className="grid gap-3 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium">Name</label>
                  <Input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
                </div>
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium">Description</label>
                  <Input
                    value={draft.description}
                    onChange={(e) => setDraft({ ...draft, description: e.target.value })}
                  />
                </div>
              </CardContent>
            </Card>

            <Accordion type="multiple" defaultValue={[draft.definition.stages[0]?.id]} className="flex flex-col gap-3">
              {draft.definition.stages.map((st, i) => (
                <AccordionItem key={i} value={st.id} className="rounded-xl border bg-card px-4">
                  <AccordionTrigger>
                    <span className="flex items-center gap-2">
                      <span className="font-mono text-xs text-muted-foreground">{i + 1}</span>
                      {st.label}
                      <span className="text-xs font-normal text-muted-foreground">
                        {st.artifacts.length} artifacts · {st.gates.length} gates
                      </span>
                    </span>
                  </AccordionTrigger>
                  <AccordionContent>
                    <StageEditor
                      stage={st}
                      index={i}
                      count={draft.definition.stages.length}
                      onChange={(ns) => setStages((all) => all.map((x, j) => (j === i ? ns : x)))}
                      onMove={(dir) =>
                        setStages((all) => {
                          const next = [...all];
                          const [x] = next.splice(i, 1);
                          next.splice(i + dir, 0, x);
                          return next;
                        })
                      }
                      onDelete={() => setStages((all) => all.filter((_, j) => j !== i))}
                    />
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
            <Button
              variant="outline"
              onClick={() =>
                setStages((all) => [
                  ...all,
                  { id: `stage_${all.length + 1}`, label: "New stage", artifacts: [], gates: [] },
                ])
              }
            >
              <Plus /> Add stage
            </Button>
          </div>
        )}
      </div>
    </Page>
  );
}

function StageEditor({
  stage,
  index,
  count,
  onChange,
  onMove,
  onDelete,
}: {
  stage: Stage;
  index: number;
  count: number;
  onChange: (s: Stage) => void;
  onMove: (dir: -1 | 1) => void;
  onDelete: () => void;
}) {
  const setArtifact = (i: number, a: ArtifactKind) =>
    onChange({ ...stage, artifacts: stage.artifacts.map((x, j) => (j === i ? a : x)) });
  const setGate = (i: number, g: Gate) =>
    onChange({ ...stage, gates: stage.gates.map((x, j) => (j === i ? g : x)) });

  return (
    <div className="flex flex-col gap-5 pb-2">
      <div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto]">
        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-medium">Label</label>
          <Input
            value={stage.label}
            onChange={(e) => onChange({ ...stage, label: e.target.value })}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-medium">ID</label>
          <Input
            value={stage.id}
            onChange={(e) => onChange({ ...stage, id: slug(e.target.value) })}
            className="font-mono"
          />
        </div>
        <div className="flex items-end gap-1">
          <Button variant="ghost" size="icon-sm" disabled={index === 0} onClick={() => onMove(-1)} title="Move up">
            <ArrowUp />
          </Button>
          <Button variant="ghost" size="icon-sm" disabled={index === count - 1} onClick={() => onMove(1)} title="Move down">
            <ArrowDown />
          </Button>
          <Button variant="ghost" size="icon-sm" disabled={count === 1} onClick={onDelete} title="Delete stage">
            <Trash2 />
          </Button>
        </div>
      </div>
      <Textarea
        rows={2}
        value={stage.description || ""}
        onChange={(e) => onChange({ ...stage, description: e.target.value })}
        placeholder="What this stage is for"
      />

      <section>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-medium">Artifacts</h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              onChange({
                ...stage,
                artifacts: [
                  ...stage.artifacts,
                  { kind: `artifact_${stage.artifacts.length + 1}`, label: "New artifact", required: false, multiple: false },
                ],
              })
            }
          >
            <Plus /> Add
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          {stage.artifacts.map((a, i) => (
            <div key={i} className="grid items-center gap-2 rounded-lg border p-2 sm:grid-cols-[1fr_140px_auto_auto_auto_auto]">
              <Input
                value={a.label}
                onChange={(e) => setArtifact(i, { ...a, label: e.target.value })}
                aria-label="Artifact label"
              />
              <Input
                value={a.kind}
                onChange={(e) => setArtifact(i, { ...a, kind: slug(e.target.value) })}
                className="font-mono text-xs"
                aria-label="Artifact kind"
              />
              <Toggle label="Required" checked={a.required} onChange={(v) => setArtifact(i, { ...a, required: v })} />
              <Toggle label="Many" checked={a.multiple} onChange={(v) => setArtifact(i, { ...a, multiple: v })} />
              <Toggle label="Link" checked={!!a.external} onChange={(v) => setArtifact(i, { ...a, external: v })} />
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => onChange({ ...stage, artifacts: stage.artifacts.filter((_, j) => j !== i) })}
                title="Remove"
              >
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
      </section>

      <section>
        <div className="mb-2 flex items-center justify-between">
          <h3 className="text-sm font-medium">Approval gates</h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              onChange({
                ...stage,
                gates: [
                  ...stage.gates,
                  {
                    id: `gate_${stage.gates.length + 1}`,
                    label: "New gate",
                    question: "Looks good.",
                    approver_roles: ["solution_approver"],
                    policy: "any",
                    on_change: "flag",
                    allow_override: true,
                  },
                ],
              })
            }
          >
            <Plus /> Add
          </Button>
        </div>
        <div className="flex flex-col gap-3">
          {stage.gates.map((g, i) => (
            <Card key={i} size="sm">
              <CardHeader>
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-sm">{g.label}</CardTitle>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => onChange({ ...stage, gates: stage.gates.filter((_, j) => j !== i) })}
                    title="Remove gate"
                  >
                    <Trash2 />
                  </Button>
                </div>
                <CardDescription>A stage passes when every gate passes.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-3 sm:grid-cols-2">
                <Field label="Label">
                  <Input value={g.label} onChange={(e) => setGate(i, { ...g, label: e.target.value })} />
                </Field>
                <Field label="Approver says">
                  <Input value={g.question} onChange={(e) => setGate(i, { ...g, question: e.target.value })} />
                </Field>
                <Field label="Policy">
                  <Select value={g.policy} onValueChange={(v) => setGate(i, { ...g, policy: v as Gate["policy"] })}>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="any">Any one approver</SelectItem>
                      <SelectItem value="all">Every approver</SelectItem>
                      <SelectItem value="n">At least N approvers</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
                {g.policy === "n" ? (
                  <Field label="N">
                    <Input
                      type="number"
                      min={1}
                      value={g.min_approvals || 1}
                      onChange={(e) => setGate(i, { ...g, min_approvals: Math.max(1, Number(e.target.value)) })}
                    />
                  </Field>
                ) : (
                  <div />
                )}
                <Field label="When an artifact changes after approval">
                  <Select value={g.on_change} onValueChange={(v) => setGate(i, { ...g, on_change: v as Gate["on_change"] })}>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="flag">Keep the approval, flag it as changed</SelectItem>
                      <SelectItem value="revoke">Revoke the approval</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
                <Field label="Owners can override">
                  <div className="flex h-9 items-center">
                    <Switch
                      checked={g.allow_override}
                      onCheckedChange={(v) => setGate(i, { ...g, allow_override: v })}
                    />
                  </div>
                </Field>
                <div className="sm:col-span-2">
                  <div className="mb-1.5 text-xs font-medium">Who can approve</div>
                  <div className="flex flex-wrap gap-x-4 gap-y-2">
                    {ROLES.map((r) => (
                      <label key={r} className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={g.approver_roles.includes(r)}
                          onCheckedChange={(v) =>
                            setGate(i, {
                              ...g,
                              approver_roles: v
                                ? [...g.approver_roles, r]
                                : g.approver_roles.filter((x) => x !== r),
                            })
                          }
                        />
                        {ROLE_LABEL[r]}
                      </label>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </section>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs font-medium">{label}</label>
      {children}
    </div>
  );
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <Checkbox checked={checked} onCheckedChange={(v) => onChange(!!v)} />
      {label}
    </label>
  );
}
