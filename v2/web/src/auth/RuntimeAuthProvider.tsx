import {
  ClerkProvider,
  useAuth,
  useClerk,
  useUser,
} from "@clerk/react";
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import type { components } from "../api/schema.generated";
import {
  AuthValueProvider,
  type AuthContextValue,
  type AuthErrorCode,
  type AuthOrganization,
} from "./AuthContext";
import { clearTenantCaches } from "./tenantCache";

type RuntimeConfig = components["schemas"]["RuntimeConfig"];
type OrganizationList = components["schemas"]["OrganizationList"];

function createClerkTokenProvider(
  getToken: () => Promise<string | null | undefined>,
): () => Promise<string | null> {
  return async () => (await getToken()) ?? null;
}

class AuthBridgeError extends Error {
  constructor(
    readonly code: AuthErrorCode,
    message: string,
  ) {
    super(message);
    this.name = "AuthBridgeError";
  }
}

class OrganizationDirectoryError extends Error {
  constructor(
    readonly status: number,
    readonly body: string,
  ) {
    super(`organization directory returned ${status}`);
    this.name = "OrganizationDirectoryError";
  }
}

type RuntimeAuthProviderProps = PropsWithChildren<{
  config?: RuntimeConfig;
  fetchImpl?: typeof fetch;
}>;

const loadingValue: AuthContextValue = {
  status: "loading",
  organizations: [],
  getToken: async () => null,
  setActiveOrganization: async () => undefined,
  signOut: async () => undefined,
};

export function RuntimeAuthProvider({
  children,
  config: configOverride,
  fetchImpl = fetch,
}: RuntimeAuthProviderProps) {
  const [config, setConfig] = useState<RuntimeConfig | null>(configOverride ?? null);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (configOverride) {
      setConfig(configOverride);
      return;
    }

    const controller = new AbortController();
    fetchImpl("/api/v1/runtime-config", {
      headers: { Accept: "application/json" },
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`runtime config returned ${response.status}`);
        }
        setConfig((await response.json()) as RuntimeConfig);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(cause instanceof Error ? cause.message : "runtime config unavailable");
      });

    return () => controller.abort();
  }, [configOverride, fetchImpl]);

  if (error) {
    return (
      <AuthValueProvider
        value={{
          ...loadingValue,
          status: "error",
          error: "No se pudo cargar la configuración de autenticación.",
          errorCode: "AUTH_RUNTIME_CONFIG_UNAVAILABLE",
        }}
      >
        {children}
      </AuthValueProvider>
    );
  }

  if (!config) {
    return <AuthValueProvider value={loadingValue}>{children}</AuthValueProvider>;
  }

  if (!config.auth.configured || !config.auth.publishable_key) {
    return (
      <AuthValueProvider value={{ ...loadingValue, status: "unconfigured" }}>
        {children}
      </AuthValueProvider>
    );
  }

  return (
    <ClerkProvider
      publishableKey={config.auth.publishable_key}
      signInUrl="/sign-in"
      signUpUrl="/accept-invitation"
      afterSignOutUrl="/sign-in"
      taskUrls={{ "choose-organization": "/select-organization" }}
      appearance={{
        variables: {
          colorPrimary: "#0085db",
          colorForeground: "#2a3547",
          colorMutedForeground: "#8898aa",
          colorBackground: "#ffffff",
          colorInput: "#f8fafc",
          colorInputForeground: "#2a3547",
          borderRadius: "0.75rem",
          fontFamily: '"Plus Jakarta Sans", ui-sans-serif, system-ui, sans-serif',
        },
        elements: {
          cardBox: "clerk-card-box",
          card: "clerk-card",
          headerTitle: "clerk-title",
          headerSubtitle: "clerk-subtitle",
          formButtonPrimary: "clerk-primary-button",
          footerAction: "clerk-footer-action",
        },
      }}
    >
      <ClerkAuthBridge fetchImpl={fetchImpl}>{children}</ClerkAuthBridge>
    </ClerkProvider>
  );
}

function ClerkAuthBridge({
  children,
  fetchImpl,
}: PropsWithChildren<{ fetchImpl: typeof fetch }>) {
  const auth = useAuth();
  const clerk = useClerk();
  const { user } = useUser();
  const [organizations, setOrganizations] = useState<AuthOrganization[]>([]);
  const [directoryStatus, setDirectoryStatus] = useState<
    "idle" | "loading" | "ready" | "error"
  >("idle");
  const [directoryError, setDirectoryError] = useState<AuthBridgeError>();
  const principalRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (!auth.isLoaded) return;
    const principal = auth.isSignedIn
      ? `${auth.userId ?? ""}:${auth.orgId ?? ""}`
      : "signed-out";
    if (principalRef.current !== undefined && principalRef.current !== principal) {
      clearTenantCaches();
    }
    principalRef.current = principal;
  }, [auth.isLoaded, auth.isSignedIn, auth.orgId, auth.userId]);

  useEffect(() => {
    if (!auth.isLoaded || !auth.isSignedIn) {
      setOrganizations([]);
      setDirectoryStatus("idle");
      setDirectoryError(undefined);
      return;
    }

    const controller = new AbortController();
    setDirectoryStatus("loading");
    setDirectoryError(undefined);
    const sessionTokenProvider = createClerkTokenProvider(
      () => auth.getToken(),
    );
    const refreshedSessionTokenProvider = createClerkTokenProvider(
      () => auth.getToken({ skipCache: true }),
    );
    loadAuthorizedOrganizationDirectory(
      sessionTokenProvider,
      refreshedSessionTokenProvider,
      fetchImpl,
      controller.signal,
    )
      .then((response) => {
        const admitted = response.map<AuthOrganization>((organization) => {
          if (!organization.switch_key) {
            throw new Error("organization switch key unavailable");
          }
          return {
            id: organization.id,
            switchKey: organization.switch_key,
            name: organization.name,
            slug: organization.slug || undefined,
            role: organization.role,
          };
        });
        setOrganizations(admitted);
        setDirectoryStatus("ready");
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setOrganizations([]);
        setDirectoryStatus("error");
        setDirectoryError(toAuthBridgeError(cause));
      });

    return () => controller.abort();
  }, [auth.getToken, auth.isLoaded, auth.isSignedIn, auth.sessionId, fetchImpl]);

  const value = useMemo<AuthContextValue>(() => {
    if (!auth.isLoaded || (auth.isSignedIn && directoryStatus !== "ready")) {
      if (directoryStatus === "error") {
        return {
          ...loadingValue,
          status: "error",
          error:
            directoryError?.message ||
            "No se pudo cargar el directorio de organizaciones.",
          errorCode: directoryError?.code || "AUTH_DIRECTORY_UNAVAILABLE",
        };
      }
      return loadingValue;
    }
    if (!auth.isSignedIn) {
      return {
        ...loadingValue,
        status: "signed-out",
      };
    }

    const activeOrganization = organizations.find(
      (organization) => organization.switchKey === auth.orgId,
    );
    return {
      status: "signed-in",
      sessionId: auth.sessionId ?? undefined,
      activeOrganizationId: activeOrganization?.id,
      organizations,
      user: user
        ? {
            id: user.id,
            email: user.primaryEmailAddress?.emailAddress,
            displayName: user.fullName || user.primaryEmailAddress?.emailAddress || "Usuario",
            avatarUrl: user.imageUrl,
          }
        : undefined,
      getToken: async (forceRefresh = false) =>
        auth.getToken({ skipCache: forceRefresh }),
      setActiveOrganization: async (organizationId: string) => {
        const organization = organizations.find((candidate) => candidate.id === organizationId);
        if (!organization) {
          throw new Error("organization is not admitted locally");
        }
        clearTenantCaches();
        await clerk.setActive({ organization: organization.switchKey });
        await auth.getToken({ skipCache: true });
      },
      signOut: async () => {
        clearTenantCaches();
        await clerk.signOut({ redirectUrl: "/sign-in" });
      },
    };
  }, [auth, clerk, directoryError, directoryStatus, organizations, user]);

  return <AuthValueProvider value={value}>{children}</AuthValueProvider>;
}

async function loadOrganizationDirectory(
  getToken: () => Promise<string | null>,
  fetchImpl: typeof fetch,
  signal: AbortSignal,
): Promise<OrganizationList["items"]> {
  const organizations: OrganizationList["items"] = [];
  const seenCursors = new Set<string>();
  let cursor: string | undefined;

  for (;;) {
    const query = new URLSearchParams({ limit: "100" });
    if (cursor) query.set("cursor", cursor);
    const token = await requireSessionToken(getToken);
    const response = await fetchOrganizationDirectoryPage(
      fetchImpl,
      `/api/v1/organizations?${query.toString()}`,
      token,
      signal,
    );
    organizations.push(...response.items);

    const nextCursor = response.page.next_cursor?.trim() || undefined;
    if (!nextCursor) return organizations;
    if (seenCursors.has(nextCursor)) {
      throw new Error("organization directory returned a repeated cursor");
    }
    seenCursors.add(nextCursor);
    cursor = nextCursor;
  }
}

async function loadAuthorizedOrganizationDirectory(
  getToken: () => Promise<string | null>,
  refreshToken: () => Promise<string | null>,
  fetchImpl: typeof fetch,
  signal: AbortSignal,
): Promise<OrganizationList["items"]> {
  try {
    return await loadOrganizationDirectory(getToken, fetchImpl, signal);
  } catch (cause: unknown) {
    if (!isInvalidSessionToken(cause)) {
      throw cause;
    }
  }

  try {
    return await loadOrganizationDirectory(refreshToken, fetchImpl, signal);
  } catch (cause: unknown) {
    if (isInvalidSessionToken(cause)) {
      throw new AuthBridgeError(
        "AUTH_SESSION_REJECTED",
        "La API rechazó la sesión de Clerk.",
      );
    }
    throw cause;
  }
}

async function fetchOrganizationDirectoryPage(
  fetchImpl: typeof fetch,
  path: string,
  token: string,
  signal: AbortSignal,
): Promise<OrganizationList> {
  const response = await fetchImpl(path, {
    method: "GET",
    cache: "no-store",
    credentials: "same-origin",
    signal,
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
  });
  const body = await response.text();
  if (!response.ok) {
    throw new OrganizationDirectoryError(response.status, body);
  }
  try {
    const payload = JSON.parse(body) as OrganizationList;
    if (!Array.isArray(payload.items) || !payload.page) {
      throw new Error("invalid organization directory payload");
    }
    return payload;
  } catch {
    throw new AuthBridgeError(
      "AUTH_DIRECTORY_UNAVAILABLE",
      "No se pudo cargar el directorio de organizaciones.",
    );
  }
}

async function requireSessionToken(
  getToken: () => Promise<string | null>,
): Promise<string> {
  let token: string | null;
  try {
    token = await getToken();
  } catch {
    throw new AuthBridgeError(
      "AUTH_SESSION_TOKEN_UNAVAILABLE",
      "No se pudo obtener una sesión válida de Clerk.",
    );
  }
  if (!token) {
    throw new AuthBridgeError(
      "AUTH_SESSION_TOKEN_UNAVAILABLE",
      "No se pudo obtener una sesión válida de Clerk.",
    );
  }
  return token;
}

function isInvalidSessionToken(cause: unknown): boolean {
  if (
    !(cause instanceof OrganizationDirectoryError) ||
    cause.status !== 401 ||
    !cause.body
  ) {
    return false;
  }
  try {
    const payload = JSON.parse(cause.body) as {
      error?: { code?: string };
    };
    return payload.error?.code === "AUTH_INVALID_TOKEN";
  } catch {
    return false;
  }
}

function toAuthBridgeError(cause: unknown): AuthBridgeError {
  if (cause instanceof AuthBridgeError) {
    return cause;
  }
  return new AuthBridgeError(
    "AUTH_DIRECTORY_UNAVAILABLE",
    "No se pudo cargar el directorio de organizaciones.",
  );
}
