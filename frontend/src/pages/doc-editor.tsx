// Write a Forge document (problem statement, PRD, implementation doc, …) with
// Katalyst's Markdown editor. Each save publishes a new immutable version, so
// comments, approvals and history keep working exactly as for agent docs.
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, Eye, Save } from "lucide-react";
import {
  MarkdownEditor,
  fullMarkdownEditorConfig,
} from "@kalvi-education/katalyst/components/markdown-editor";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Badge } from "@kalvi-education/katalyst/components/badge";
import { Input } from "@kalvi-education/katalyst/components/input";
import { Spinner } from "@kalvi-education/katalyst/components/spinner";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@kalvi-education/katalyst/components/alert";
import { MarkdownDraft } from "@/components/markdown-draft";
import { useTitle } from "@/lib/use-title";
import { queryKeys } from "@/lib/queryKeys";
import { solutionKeys, solutionsApi } from "@/lib/solutions";

export function DocEditorPage() {
  const { id = "" } = useParams();
  const nav = useNavigate();
  const qc = useQueryClient();
  const source = useQuery({
    queryKey: solutionKeys.docSource(id),
    queryFn: () => solutionsApi.docSource(id),
    refetchOnWindowFocus: false,
  });
  const ctx = useQuery({
    queryKey: solutionKeys.docContext(id),
    queryFn: () => solutionsApi.docContext(id),
  });
  useTitle(source.data ? `Edit · ${source.data.title}` : "Edit");

  const [markdown, setMarkdown] = useState<string | null>(null);
  const [summary, setSummary] = useState("");
  const [base, setBase] = useState("");
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState(false);
  const loaded = useRef(false);
  // The editor normalises Markdown when it mounts; treat its first output as
  // the saved baseline so opening a doc doesn't mark it dirty.
  const [baseline, setBaseline] = useState<string | null>(null);
  const normalised = useRef(false);

  useEffect(() => {
    if (source.data && !loaded.current) {
      loaded.current = true;
      setMarkdown(source.data.markdown);
      setBaseline(source.data.markdown);
      setBase(source.data.version_id);
      normalised.current = false;
    }
  }, [source.data]);

  const dirty = markdown !== null && baseline !== null && markdown !== baseline;
  const onEdit = (md: string) => {
    if (!normalised.current) {
      normalised.current = true;
      setBaseline(md);
    }
    setMarkdown(md);
  };

  // Warn before leaving with unsaved edits.
  useEffect(() => {
    if (!dirty) return;
    const h = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", h);
    return () => window.removeEventListener("beforeunload", h);
  }, [dirty]);

  async function save(andView = false) {
    if (markdown === null) return;
    setSaving(true);
    try {
      const v = await solutionsApi.saveDocSource(id, {
        markdown,
        base_version_id: base,
        change_summary: summary.trim() || undefined,
      });
      setBase(v.id);
      setBaseline(markdown);
      setSummary("");
      qc.setQueryData(solutionKeys.docSource(id), {
        ...source.data!,
        markdown,
        version_id: v.id,
      });
      qc.invalidateQueries({ queryKey: queryKeys.document(id) });
      qc.invalidateQueries({ queryKey: queryKeys.versions(id) });
      if (ctx.data?.solution) qc.invalidateQueries({ queryKey: solutionKeys.board(ctx.data.solution.key) });
      toast.success(`Published v${v.number}`);
      if (andView) nav(`/app/d/${id}`);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Save failed";
      if (/newer version/i.test(msg)) setConflict(true);
      toast.error(msg);
    } finally {
      setSaving(false);
    }
  }

  // Cmd/Ctrl+S saves.
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (dirty && !saving) save();
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  });

  if (source.isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner />
      </div>
    );
  }
  if (source.error || !source.data) {
    return (
      <div className="mx-auto max-w-xl p-10 text-sm text-muted-foreground">
        Could not open this document for editing.
      </div>
    );
  }
  if (!source.data.editable) {
    return (
      <div className="mx-auto max-w-xl p-10">
        <Alert>
          <AlertTitle>This document isn't editable here</AlertTitle>
          <AlertDescription>
            It was published as HTML (for example by an agent or Stitch). Comment on it, or
            ask its author to publish a new version.
            <div className="mt-3">
              <Button variant="outline" size="sm" asChild>
                <Link to={`/app/d/${id}`}>Open document</Link>
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const backTo = ctx.data?.solution ? `/app/solutions/${ctx.data.solution.key}` : `/app/d/${id}`;

  return (
    <div className="flex h-full flex-col bg-background">
      <header className="flex h-14 shrink-0 items-center justify-between gap-3 border-b px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon-sm" asChild aria-label="Back">
            <Link to={backTo}>
              <ArrowLeft />
            </Link>
          </Button>
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{source.data.title}</div>
            {ctx.data?.solution && (
              <div className="truncate text-xs text-muted-foreground">
                {ctx.data.solution.key} · {ctx.data.stage_label}
              </div>
            )}
          </div>
          {dirty ? <Badge variant="outline">Unsaved</Badge> : <Badge variant="secondary">Saved</Badge>}
        </div>
        <div className="flex items-center gap-2">
          <Input
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            placeholder="What changed? (optional)"
            className="hidden w-64 md:block"
            aria-label="Change summary"
          />
          <Button variant="ghost" size="sm" asChild>
            <Link to={`/app/d/${id}`}>
              <Eye /> View
            </Link>
          </Button>
          <Button size="sm" disabled={!dirty || saving} onClick={() => save()}>
            {saving ? <Spinner /> : <Save />} Publish version
          </Button>
        </div>
      </header>
      {conflict && (
        <div className="border-b bg-muted px-4 py-2 text-sm">
          Someone published a newer version while you were editing. Copy your changes, then{" "}
          <button className="underline" onClick={() => window.location.reload()}>
            reload
          </button>
          .
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mx-auto max-w-[860px] px-4 py-8">
          {markdown !== null && (
            <MarkdownDraft value={markdown} onChange={(md) => { normalised.current = true; setMarkdown(md); }}>
            <MarkdownEditor
              value={markdown}
              onChange={onEdit}
              config={fullMarkdownEditorConfig}
              variant="borderless"
              aria-label={source.data.title}
              contentClassName="min-h-[60vh]"
            />
            </MarkdownDraft>
          )}
        </div>
      </div>
    </div>
  );
}
