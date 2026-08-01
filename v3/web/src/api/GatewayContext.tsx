import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { WebConfig } from "../config";
import type { SchedulingGateway } from "./SchedulingGateway";
import { createHttpSchedulingGateway } from "./httpSchedulingGateway";

const GatewayContext = createContext<SchedulingGateway | null>(null);

async function loadFakeSchedulingGateway(): Promise<SchedulingGateway> {
  if (!import.meta.env.DEV && import.meta.env.MODE !== "e2e") {
    throw new Error("El fake de Agenda no forma parte del runtime productivo.");
  }
  const module = await import("./fakeSchedulingGateway");
  return module.getFakeSchedulingGateway();
}

export function GatewayProvider({ config, children }: { config: WebConfig; children: ReactNode }) {
  const httpGateway = useMemo(
    () => createHttpSchedulingGateway(config.apiBaseUrl),
    [config.apiBaseUrl],
  );
  const [fakeGateway, setFakeGateway] = useState<SchedulingGateway | null>(null);
  const [fakeError, setFakeError] = useState<string | null>(null);

  useEffect(() => {
    if (!config.useFakeApi) {
      setFakeGateway(null);
      setFakeError(null);
      return;
    }
    let active = true;
    void loadFakeSchedulingGateway()
      .then((gateway) => {
        if (active) {
          setFakeGateway(gateway);
          setFakeError(null);
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setFakeError(
            error instanceof Error
              ? error.message
              : "No se pudo iniciar el fake de Agenda.",
          );
        }
      });
    return () => {
      active = false;
    };
  }, [config.useFakeApi]);

  if (config.useFakeApi && fakeError) {
    return (
      <main className="public-loading" id="main-content">
        {fakeError}
      </main>
    );
  }
  const gateway = config.useFakeApi ? fakeGateway : httpGateway;
  if (!gateway) {
    return (
      <main className="public-loading" id="main-content">
        Iniciando entorno de Agenda…
      </main>
    );
  }
  return <GatewayContext.Provider value={gateway}>{children}</GatewayContext.Provider>;
}

export function useSchedulingGateway(): SchedulingGateway {
  const value = useContext(GatewayContext);
  if (!value) {
    throw new Error("useSchedulingGateway debe utilizarse dentro de GatewayProvider");
  }
  return value;
}
