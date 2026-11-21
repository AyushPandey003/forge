import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { DesignRevisionProgress } from "./design-revision-progress";

afterEach(cleanup);

it("shows the real stage and can collapse without blocking the document", () => {
  const { rerender } = render(
    <DesignRevisionProgress
      stage="Revising Dashboard"
      startedAt={new Date().toISOString()}
    />,
  );
  expect(screen.getByRole("status")).toHaveTextContent("Revising Dashboard");
  const toggle = screen.getByRole("button", { name: /Reimagining your design/ });
  fireEvent.click(toggle);
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  expect(screen.queryByRole("status")).toBeNull();
  fireEvent.click(toggle);
  rerender(<DesignRevisionProgress stage="Publishing to Forge" />);
  expect(screen.getByRole("status")).toHaveTextContent("Publishing to Forge");
});
