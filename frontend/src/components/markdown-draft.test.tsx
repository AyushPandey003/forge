import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MarkdownDraft, readMarkdownFile } from "./markdown-draft";

afterEach(cleanup);
it("allows pasting raw Markdown", () => {
  const change = vi.fn();
  render(<MarkdownDraft value="" onChange={change}><div>Rich editor</div></MarkdownDraft>);
  fireEvent.click(screen.getByRole("button", { name: "Markdown source" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Markdown source" }), { target: { value: "# Problem\n\n- Evidence" } });
  expect(change).toHaveBeenCalledWith("# Problem\n\n- Evidence");
});
it("validates Markdown uploads and strips BOM", async () => {
  await expect(readMarkdownFile({name:"prd.md",size:20,text:async()=>"\uFEFF# PRD"} as File)).resolves.toBe("# PRD");
  await expect(readMarkdownFile({name:"image.png",size:10} as File)).rejects.toThrow("Choose a .md");
  await expect(readMarkdownFile({name:"large.md",size:3*1024*1024} as File)).rejects.toThrow("2 MB");
  await expect(readMarkdownFile({name:"binary.md",size:10,text:async()=>"\0"} as File)).rejects.toThrow("not a text");
});
