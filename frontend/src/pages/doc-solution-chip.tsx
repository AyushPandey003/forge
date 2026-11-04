// Shows which solution/stage a document belongs to, and offers "Edit" for
// documents authored in Forge (Markdown source).
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { PenLine, Workflow } from "lucide-react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { solutionKeys, solutionsApi } from "@/lib/solutions";

export function DocSolutionChip({ docId, canEdit }: { docId: string; canEdit: boolean }) {
  const ctx = useQuery({
    queryKey: solutionKeys.docContext(docId),
    queryFn: () => solutionsApi.docContext(docId),
    staleTime: 60_000,
  });
  const src = useQuery({
    queryKey: [...solutionKeys.docSource(docId), "editable"],
    queryFn: () => solutionsApi.docSource(docId),
    enabled: canEdit,
    staleTime: 60_000,
    select: (d) => d.editable,
  });
  const sol = ctx.data?.solution;
  return (
    <>
      {sol && (
        <Button variant="ghost" size="sm" asChild className="hidden shrink-0 md:inline-flex">
          <Link
            to={`/app/solutions/${sol.key}?stage=${ctx.data?.artifact?.stage || ""}`}
            title={`${sol.title} · ${ctx.data?.stage_label}`}
          >
            <Workflow /> {sol.key} · {ctx.data?.stage_label}
          </Link>
        </Button>
      )}
      {canEdit && src.data && (
        <Button variant="ghost" size="sm" asChild className="shrink-0">
          <Link to={`/app/edit/${docId}`} title="Edit and publish a new version">
            <PenLine /> <span className="hidden sm:inline">Edit</span>
          </Link>
        </Button>
      )}
    </>
  );
}
