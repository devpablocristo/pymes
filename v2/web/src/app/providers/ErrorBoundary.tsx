import { Component, type ErrorInfo, type PropsWithChildren, type ReactNode } from "react";
import { FatalErrorState } from "../states/ContentStates";

type State = { failed: boolean };

export class ErrorBoundary extends Component<PropsWithChildren, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Pymes v2 render failed", error, info.componentStack);
  }

  render(): ReactNode {
    if (this.state.failed) {
      return (
        <FatalErrorState
          title="Pymes no pudo iniciar"
          body="Recargá la página. Si el problema continúa, contactá a soporte."
          reloadLabel="Recargar"
        />
      );
    }
    return this.props.children;
  }
}
