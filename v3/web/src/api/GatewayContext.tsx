import { createContext, type ReactNode, useContext, useMemo } from "react";
import type { WebConfig } from "../config";
import type { SchedulingGateway } from "./SchedulingGateway";
import { getFakeSchedulingGateway } from "./fakeSchedulingGateway";
import { createHttpSchedulingGateway } from "./httpSchedulingGateway";

const GatewayContext = createContext<SchedulingGateway | null>(null);

export function GatewayProvider({ config, children }: { config: WebConfig; children: ReactNode }) {
  const gateway = useMemo(
    () => (config.useFakeApi ? getFakeSchedulingGateway() : createHttpSchedulingGateway(config.apiBaseUrl)),
    [config.apiBaseUrl, config.useFakeApi],
  );
  return <GatewayContext.Provider value={gateway}>{children}</GatewayContext.Provider>;
}

export function useSchedulingGateway(): SchedulingGateway {
  const value = useContext(GatewayContext);
  if (!value) {
    throw new Error("useSchedulingGateway debe utilizarse dentro de GatewayProvider");
  }
  return value;
}
