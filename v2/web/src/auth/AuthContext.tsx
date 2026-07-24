import { createContext, useContext, type PropsWithChildren } from "react";

export type AuthStatus =
  | "loading"
  | "unconfigured"
  | "signed-out"
  | "signed-in"
  | "error";

export type AuthErrorCode =
  | "AUTH_RUNTIME_CONFIG_UNAVAILABLE"
  | "AUTH_SESSION_TOKEN_UNAVAILABLE"
  | "AUTH_SESSION_REJECTED"
  | "AUTH_DIRECTORY_UNAVAILABLE";

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
  productRole: "owner" | "user";
  error?: string;
  errorCode?: AuthErrorCode;
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
  productRole: "user",
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
