import {
  createContext,
  type ReactNode,
  useContext,
} from "react";
import type { RequestIdentity } from "../api/SchedulingGateway";

export type Session = {
  identity: RequestIdentity;
  organizationName: string;
  organizationSlug: string;
  accountControls: ReactNode;
  permissions: readonly string[];
  local: boolean;
};

export const SessionContext = createContext<Session | null>(null);

export function useSession(): Session {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error("useSession debe utilizarse dentro de AdminAuthBoundary");
  }
  return value;
}
