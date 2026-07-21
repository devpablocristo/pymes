import { Component, type ErrorInfo, type PropsWithChildren, type ReactNode } from "react";

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
        <main className="fatal-error" role="alert">
          <h1>No pudimos cargar la aplicación.</h1>
          <p>Actualizá la página para volver a intentarlo.</p>
        </main>
      );
    }
    return this.props.children;
  }
}
