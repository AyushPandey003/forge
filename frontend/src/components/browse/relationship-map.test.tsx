import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "@/api";
import { RelationshipMap } from "./relationship-map";

function mount(folderId?: string, document?: { id: string; title: string }) {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter>
        <RelationshipMap folderId={folderId} document={document} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("RelationshipMap", () => {
  it("loads on demand and shows the full location and current document", async () => {
    const folder = vi.spyOn(api, "getFolder").mockResolvedValue({
      id: "child",
      title: "Engineering",
      ancestors: [{ id: "parent", title: "Team" }],
    });
    const list = vi.spyOn(api, "listFolders").mockResolvedValue({
      has_more: false,
      items: [{ type: "document", document: { id: "doc", title: "Roadmap" } }],
    });
    mount("child", { id: "doc", title: "Roadmap" });
    expect(folder).not.toHaveBeenCalled();
    expect(list).not.toHaveBeenCalled();
    await userEvent.click(
      screen.getByRole("button", { name: "Document relationship map" }),
    );
    expect(await screen.findByRole("link", { name: "Team" })).toHaveAttribute(
      "href",
      "/app/folders/parent",
    );
    expect(
      await screen.findByRole("link", { name: /Roadmap.*You are here/ }),
    ).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Engineering" })).toHaveAttribute(
      "href",
      "/app/folders/child",
    );
  });

  it("fetches nested folders only when expanded", async () => {
    const list = vi.spyOn(api, "listFolders").mockImplementation(async (id) => ({
      has_more: false,
      items: id
        ? [{ type: "document", document: { id: "nested", title: "Nested guide" } }]
        : [{ type: "folder", folder: { id: "folder", title: "Guides" } }],
    }));
    mount();
    await userEvent.click(
      screen.getByRole("button", { name: "Document relationship map" }),
    );
    await screen.findByRole("link", { name: "Guides" });
    expect(list).toHaveBeenCalledTimes(1);
    await userEvent.click(screen.getByRole("button", { name: "Expand Guides" }));
    expect(await screen.findByRole("link", { name: "Nested guide" })).toHaveAttribute(
      "href",
      "/app/d/nested",
    );
    await userEvent.click(screen.getByRole("button", { name: "Collapse Guides" }));
    expect(screen.queryByRole("link", { name: "Nested guide" })).toBeNull();
  });

  it("offers retry after a failed load", async () => {
    vi.spyOn(api, "listFolders")
      .mockRejectedValueOnce(new Error("Offline"))
      .mockResolvedValue({ items: [], has_more: false });
    mount();
    await userEvent.click(
      screen.getByRole("button", { name: "Document relationship map" }),
    );
    await userEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByText("No items in this folder.")).toBeTruthy();
  });
});
