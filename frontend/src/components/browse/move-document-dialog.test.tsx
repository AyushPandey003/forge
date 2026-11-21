import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "@/api";
import { MoveDocumentDialog } from "./move-document-dialog";

function mount(parentFolder?: { id: string; title: string }) {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MoveDocumentDialog
        documentId="doc"
        title="Roadmap"
        parentFolder={parentFolder}
      />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("MoveDocumentDialog", () => {
  it("does not load until opened and prevents moving to the current location", async () => {
    const list = vi
      .spyOn(api, "listFolders")
      .mockResolvedValue({ items: [], has_more: false });
    mount();
    expect(list).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Move document" }));
    await screen.findByText("The document is already in this location.");
    expect(screen.getByRole("button", { name: "Move here" })).toBeDisabled();
  });

  it("browses a destination and moves only on confirmation", async () => {
    vi.spyOn(api, "listFolders").mockImplementation(async (id) => ({
      items: id
        ? []
        : [{ type: "folder", folder: { id: "target", title: "Engineering" } }],
      has_more: false,
    }));
    const move = vi
      .spyOn(api, "moveDocumentToFolder")
      .mockResolvedValue({ document_id: "doc", moved: true });
    mount();
    await userEvent.click(screen.getByRole("button", { name: "Move document" }));
    await userEvent.click(await screen.findByRole("button", { name: "Engineering" }));
    await screen.findByText("No subfolders. You can move the document here.");
    expect(move).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Move here" }));
    expect(move).toHaveBeenCalledWith("target", "doc");
  });

  it("can move a nested document to top level and shows failures without closing", async () => {
    vi.spyOn(api, "listFolders").mockResolvedValue({ items: [], has_more: false });
    const move = vi
      .spyOn(api, "moveDocumentToRoot")
      .mockRejectedValueOnce(new Error("Forbidden"));
    mount({ id: "source", title: "Source" });
    await userEvent.click(screen.getByRole("button", { name: "Move document" }));
    await screen.findByText("No subfolders. You can move the document here.");
    await userEvent.click(screen.getByRole("button", { name: "Move here" }));
    expect(move).toHaveBeenCalledWith("source", "doc");
    expect(await screen.findByRole("alert")).toHaveTextContent("Forbidden");
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("surfaces permission warnings returned by a successful move", async () => {
    vi.spyOn(api, "listFolders").mockImplementation(async (id) => ({
      items: id ? [] : [{ type: "folder", folder: { id: "target", title: "Shared" } }],
      has_more: false,
    }));
    vi.spyOn(api, "moveDocumentToFolder").mockResolvedValue({
      document_id: "doc",
      moved: true,
      permission_warning: { message: "Team members may gain access." },
    });
    mount();
    await userEvent.click(screen.getByRole("button", { name: "Move document" }));
    await userEvent.click(await screen.findByRole("button", { name: "Shared" }));
    await screen.findByText("No subfolders. You can move the document here.");
    await userEvent.click(screen.getByRole("button", { name: "Move here" }));
    expect(await screen.findByText("Team members may gain access.")).toBeTruthy();
  });
});
