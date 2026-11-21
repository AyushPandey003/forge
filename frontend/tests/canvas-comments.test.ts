// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { installCanvasComments } from "../bridge/canvas-comments";

afterEach(() => {
  document.body.innerHTML = "";
});

it("injects selection controls into canvas screens and validates the sending frame", () => {
  document.body.innerHTML =
    '<script id="forge-canvas-data" type="application/json">{}</script><div id="world"><iframe title="Dashboard"></iframe></div>';
  const frame = document.querySelector("iframe")!;
  frame.srcdoc = "<html><body><p>Hello</p></body></html>";
  const comment = vi.fn();
  const activate = vi.fn();
  const paint = installCanvasComments(document, comment, activate);
  const post = vi.spyOn(frame.contentWindow!, "postMessage");
  paint?.([{ id: "c1", quote: "[Screen: Dashboard] Hello", author: "Ayush" }], "c1");
  expect(post).toHaveBeenCalledWith(
    {
      type: "forge:paint-comments",
      items: [{ id: "c1", quote: "Hello", author: "Ayush" }],
      active: "c1",
    },
    "*",
  );
  window.dispatchEvent(
    new MessageEvent("message", {
      source: frame.contentWindow,
      data: { type: "forge:activate-comment", id: "c1" },
    }),
  );
  expect(activate).toHaveBeenCalledWith("c1");
  expect(frame.srcdoc).toContain("Comment on selected text");
  window.dispatchEvent(
    new MessageEvent("message", {
      data: { type: "forge:screen-comment", quote: "Hello" },
    }),
  );
  expect(comment).not.toHaveBeenCalled();
  window.dispatchEvent(
    new MessageEvent("message", {
      source: frame.contentWindow,
      data: { type: "forge:screen-comment", quote: "Hello" },
    }),
  );
  expect(comment).toHaveBeenCalledWith("[Screen: Dashboard] Hello");
});
