import { render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";

function BrokenView(): never {
  throw new Error("broken view");
}

test("renders a stable fallback when a child crashes", () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
  render(
    <ErrorBoundary>
      <BrokenView />
    </ErrorBoundary>,
  );

  expect(screen.getByRole("alert")).toHaveTextContent("No pudimos cargar la aplicación.");
  consoleError.mockRestore();
});
