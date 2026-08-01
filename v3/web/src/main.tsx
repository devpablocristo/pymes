import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import { loadWebConfig } from "./config";
import "./styles.css";

function ConfigurationFailure({ error }: { error: unknown }) {
  return (
    <main className="fatal-state" id="main-content">
      <p className="eyebrow">Configuración inválida</p>
      <h1>La aplicación no puede iniciar</h1>
      <p>{error instanceof Error ? error.message : "Revisá la configuración del entorno."}</p>
    </main>
  );
}

const root = ReactDOM.createRoot(document.getElementById("root")!);

try {
  const config = loadWebConfig();
  root.render(
    <React.StrictMode>
      <App config={config} />
    </React.StrictMode>,
  );
} catch (error) {
  root.render(<ConfigurationFailure error={error} />);
}
