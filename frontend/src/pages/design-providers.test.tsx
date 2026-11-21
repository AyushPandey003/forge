import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { forgeApi, type AIProvider } from "@/lib/forge";
import { ProvidersDialog } from "./design-providers";

const existing: AIProvider = {
  id: "aip_1",
  label: "My OpenAI",
  kind: "openai",
  base_url: "https://api.openai.com/v1",
  key_hint: "…9f3a",
  has_key: true,
  default_model: "gpt-4o",
};

function mount(onSaved = vi.fn()) {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <ProvidersDialog open onOpenChange={() => {}} onSaved={onSaved} />
    </QueryClientProvider>,
  );
  return onSaved;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ProvidersDialog", () => {
  it("lists providers with only a key hint, never the key", async () => {
    vi.spyOn(forgeApi, "providers").mockResolvedValue({
      items: [existing],
      allow_private_urls: false,
      limit: 10,
    });
    mount();
    expect(await screen.findByText(/My OpenAI/)).toBeTruthy();
    expect(screen.getByText(/key …9f3a/)).toBeTruthy();
  });

  it("shows an empty state and a retryable error", async () => {
    const spy = vi
      .spyOn(forgeApi, "providers")
      .mockRejectedValueOnce(new Error("Network down"))
      .mockResolvedValue({ items: [], allow_private_urls: false, limit: 10 });
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByText(/No providers yet/)).toBeTruthy();
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it("applies a preset, requires an API key, and sends it once", async () => {
    vi.spyOn(forgeApi, "providers").mockResolvedValue({
      items: [],
      allow_private_urls: false,
      limit: 10,
    });
    const create = vi
      .spyOn(forgeApi, "createProvider")
      .mockResolvedValue({ ...existing, id: "aip_2", label: "OpenAI" });
    const onSaved = mount();
    await userEvent.click(await screen.findByRole("button", { name: /Add provider/ }));
    await userEvent.click(screen.getByRole("button", { name: "OpenAI" }));
    const submit = screen.getByRole("button", { name: "Add provider" });
    expect((screen.getByLabelText(/Base URL/) as HTMLInputElement).value).toBe(
      "https://api.openai.com/v1",
    );
    expect(submit).toBeDisabled(); // no key yet
    await userEvent.type(screen.getByLabelText(/API key/), "sk-test-123456");
    expect(submit).toBeEnabled();
    await userEvent.click(submit);
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0]).toMatchObject({
      label: "OpenAI",
      kind: "openai",
      base_url: "https://api.openai.com/v1",
      api_key: "sk-test-123456",
    });
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it("keeps the stored key when editing without typing a new one", async () => {
    vi.spyOn(forgeApi, "providers").mockResolvedValue({
      items: [existing],
      allow_private_urls: false,
      limit: 10,
    });
    const update = vi.spyOn(forgeApi, "updateProvider").mockResolvedValue(existing);
    mount();
    await userEvent.click(
      await screen.findByRole("button", { name: "Edit My OpenAI" }),
    );
    const key = screen.getByLabelText(/API key/) as HTMLInputElement;
    expect(key.value).toBe("");
    expect(key.placeholder).toContain("…9f3a");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(update).toHaveBeenCalled());
    expect(update.mock.calls[0][1]).not.toHaveProperty("api_key");
  });

  it("confirms before removing a provider", async () => {
    vi.spyOn(forgeApi, "providers").mockResolvedValue({
      items: [existing],
      allow_private_urls: false,
      limit: 10,
    });
    const del = vi.spyOn(forgeApi, "deleteProvider").mockResolvedValue(undefined);
    mount();
    await userEvent.click(
      await screen.findByRole("button", { name: "Remove My OpenAI" }),
    );
    expect(del).not.toHaveBeenCalled();
    await userEvent.click(await screen.findByRole("button", { name: "Remove" }));
    await waitFor(() => expect(del).toHaveBeenCalledWith("aip_1"));
  });

  it("allows key-less local providers only when the server permits private URLs", async () => {
    vi.spyOn(forgeApi, "providers").mockResolvedValue({
      items: [],
      allow_private_urls: true,
      limit: 10,
    });
    mount();
    await userEvent.click(await screen.findByRole("button", { name: /Add provider/ }));
    await userEvent.click(screen.getByRole("button", { name: "Custom" }));
    await userEvent.type(screen.getByLabelText("Name"), "Ollama");
    await userEvent.clear(screen.getByLabelText(/Base URL/));
    await userEvent.type(
      screen.getByLabelText(/Base URL/),
      "http://localhost:11434/v1",
    );
    expect(screen.getByRole("button", { name: "Add provider" })).toBeEnabled();
  });

  it("removes a provider from inside its edit form, then lets the user re-add it", async () => {
    const list = vi
      .spyOn(forgeApi, "providers")
      .mockResolvedValueOnce({
        items: [existing],
        allow_private_urls: false,
        limit: 10,
      })
      .mockResolvedValue({ items: [], allow_private_urls: false, limit: 10 });
    const del = vi.spyOn(forgeApi, "deleteProvider").mockResolvedValue(undefined);
    mount();
    await userEvent.click(
      await screen.findByRole("button", { name: "Edit My OpenAI" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "Remove provider" }));
    await userEvent.click(await screen.findByRole("button", { name: "Remove" }));
    await waitFor(() => expect(del).toHaveBeenCalledWith("aip_1"));
    expect(await screen.findByText(/No providers yet/)).toBeTruthy();
    expect(list).toHaveBeenCalledTimes(2);
    expect(screen.getByRole("button", { name: /Add provider/ })).toBeEnabled();
  });
});
