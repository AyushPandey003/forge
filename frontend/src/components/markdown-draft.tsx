import { useRef, useState } from "react";
import { Button } from "@kalvi-education/katalyst/components/button";
import { Textarea } from "@kalvi-education/katalyst/components/textarea";

const MAX_BYTES = 2 * 1024 * 1024;
export async function readMarkdownFile(file: File): Promise<string> {
  if (!/\.(md|markdown|mdown)$/i.test(file.name)) throw new Error("Choose a .md, .markdown or .mdown file.");
  if (file.size > MAX_BYTES) throw new Error("Markdown files must be 2 MB or smaller.");
  const text = await file.text();
  if (text.includes("\0")) throw new Error("This file is not a text Markdown file.");
  return text.replace(/^\uFEFF/, "");
}

export function MarkdownDraft({ value, onChange, children }: {
  value: string;
  onChange: (markdown: string) => void;
  children: React.ReactNode;
}) {
  const [source, setSource] = useState(false);
  const [error, setError] = useState("");
  const input = useRef<HTMLInputElement>(null);
  return <>
    <div className="mb-4 flex flex-wrap items-center gap-2">
      <Button size="sm" variant={!source ? "secondary" : "ghost"} aria-pressed={!source} onClick={() => setSource(false)}>Rich text</Button>
      <Button size="sm" variant={source ? "secondary" : "ghost"} aria-pressed={source} onClick={() => setSource(true)}>Markdown source</Button>
      <Button size="sm" variant="outline" onClick={() => input.current?.click()}>Import .md</Button>
      <span className="text-xs text-muted-foreground">Import replaces the draft. Publish when ready.</span>
      <input ref={input} type="file" accept=".md,.markdown,.mdown,text/markdown" aria-label="Import Markdown file" className="sr-only" onChange={async (e) => {
        const file = e.target.files?.[0];
        e.target.value = "";
        if (!file) return;
        setError("");
        try {
          const text = await readMarkdownFile(file);
          if (value.trim() && !window.confirm("Replace the current draft with this Markdown file? Unsaved draft text will be replaced.")) return;
          onChange(text);
          setSource(true);
        } catch (err) { setError(err instanceof Error ? err.message : "Could not read file."); }
      }} />
    </div>
    {error && <p role="alert" className="mb-3 text-sm text-destructive">{error}</p>}
    {source ? <>
      <p className="mb-2 text-xs text-muted-foreground">Paste or edit raw Markdown below. Switch to Rich text to review it.</p>
      <Textarea aria-label="Markdown source" value={value} onChange={(e) => onChange(e.target.value)} className="min-h-[60vh] font-mono text-sm" spellCheck={false} />
    </> : children}
  </>;
}
