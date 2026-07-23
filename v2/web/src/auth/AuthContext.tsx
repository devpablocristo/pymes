import { createContext, useContext, type PropsWithChildren } from "react";

export type AuthStatus =
  | "loading"
  | "unconfigured"
  | "signed-out"
  | "signed-in"
  | "error";

export type AuthOrganization = {
  id: string;
  switchKey: string;
  name: string;
  slug?: string;
  role: "owner" | "admin" | "member";
};

export type AuthUser = {
  id: string;
  email?: string;
  displayName: string;
  avatarUrl?: string;
};

export type AuthContextValue = {
  status: AuthStatus;
  error?: string;
  sessionId?: string;
  activeOrganizationId?: string;
  organizations: AuthOrganization[];
  user?: AuthUser;
  getToken: (forceRefresh?: boolean) => Promise<string | null>;
  setActiveOrganization: (organizationId: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const unavailable = async () => null;
const unavailableAction = async () => undefined;

const defaultValue: AuthContextValue = {
  status: "loading",
  organizations: [],
  getToken: unavailable,
  setActiveOrganization: unavailableAction,
  signOut: unavailableAction,
};

const AuthContext = createContext<AuthContextValue>(defaultValue);

export function AuthValueProvider({
  children,
  value,
}: PropsWithChildren<{ value: AuthContextValue }>) {
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useProductAuth() {
  return useContext(AuthContext);
}
