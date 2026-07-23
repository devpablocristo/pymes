import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import {
  EmptyState,
  LoadingState,
  RecoverableErrorState,
  SkeletonState,
} from "./ContentStates";

test("exposes loading, skeleton and empty states accessibly", () => {
  const { rerender } = render(<LoadingState label="Cargando contenido" />);
  expect(screen.getByRole("status")).toHaveTextContent("Cargando contenido");

  rerender(<SkeletonState label="Cargando panel" />);
  expect(screen.getByRole("status", { name: "Cargando panel" })).toBeInTheDocument();

  rerender(<EmptyState title="Sin actividad" body="La actividad aparecerá acá." />);
  expect(screen.getByRole("heading", { name: "Sin actividad" })).toBeInTheDocument();
});

test("offers a real retry action for recoverable errors", async () => {
  const retry = vi.fn();
  const user = userEvent.setup();
  render(
    <RecoverableErrorState
      title="No pudimos cargar"
      body="Revisá la conexión."
      retryLabel="Intentar de nuevo"
      onRetry={retry}
    />,
  );

  await user.click(screen.getByRole("button", { name: "Intentar de nuevo" }));
  expect(retry).toHaveBeenCalledOnce();
});
