// Inline preview of a freshly published design, rendered through the same
// sandboxed render origin as the document page.
import { useQuery } from "@tanstack/react-query";
import { Spinner } from "@kalvi-education/katalyst/components/spinner";
import { api } from "@/api";

export function DesignPreview({
  docId,
  versionId,
}: {
  docId: string;
  versionId?: string;
}) {
  const token = useQuery({
    queryKey: ["forge", "design-preview", docId, versionId || ""],
    queryFn: async () => {
      const version =
        versionId || (await api.getDocument(docId)).current_version_id || "";
      return api.renderToken(docId, version);
    },
    staleTime: 4 * 60_000,
  });
  if (token.isError) return null; // the Open design button still works
  return (
    <div className="h-[min(60vh,520px)] overflow-hidden rounded-lg border bg-muted">
      {token.data?.url ? (
        <iframe
          title="Design preview"
          src={token.data.url}
          className="h-full w-full border-0"
          sandbox="allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox"
        />
      ) : (
        <div
          role="status"
          className="flex h-full items-center justify-center gap-2 text-sm text-muted-foreground"
        >
          <Spinner /> Loading preview…
        </div>
      )}
    </div>
  );
}
