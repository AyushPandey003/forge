import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { accountsApi } from "@/lib/accounts";
import { JoinPage } from "./account-links";

vi.mock("@/lib/accounts", () => ({
  tokenFromHash: () => "test-invite",
  accountsApi: { inspect: vi.fn(), register: vi.fn() },
}));
afterEach(cleanup);
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(accountsApi.inspect).mockResolvedValue({ kind: "invite", workspace: "Test", role: "member", team_id: "", expires_at: "" });
});
function mount() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<MemoryRouter><QueryClientProvider client={qc}><JoinPage /></QueryClientProvider></MemoryRouter>);
}
async function fill(password: string, confirmation: string) {
  await screen.findByLabelText("Full name");
  fireEvent.change(screen.getByLabelText("Full name"), { target: { value: "Test User" } });
  fireEvent.change(screen.getByLabelText(/Username/), { target: { value: "test-user" } });
  fireEvent.change(screen.getByLabelText(/^Password/), { target: { value: password } });
  fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: confirmation } });
  fireEvent.submit(screen.getByRole("button", { name: "Create account" }).closest("form")!);
}
it("does not submit mismatched confirmation", async () => {
  mount();
  await fill("correct-password-123", "different-password-123");
  expect(await screen.findByText("Passwords don't match.")).toBeTruthy();
  expect(accountsApi.register).not.toHaveBeenCalled();
});
it("rejects short passwords even when native form validation is bypassed", async () => {
  mount();
  await fill("short", "short");
  expect(await screen.findByText("Password must be 12–72 bytes.")).toBeTruthy();
  expect(accountsApi.register).not.toHaveBeenCalled();
});
it("keeps failed registration on the form without rechecking the link", async () => {
  vi.mocked(accountsApi.register).mockRejectedValue(new Error("Username already exists"));
  mount();
  await fill("valid-password-123", "valid-password-123");
  await waitFor(() => expect(accountsApi.register).toHaveBeenCalledWith(expect.objectContaining({ confirm_password: "valid-password-123" })));
  expect(await screen.findByText("Username already exists")).toBeTruthy();
  expect(accountsApi.inspect).toHaveBeenCalledTimes(1);
});
