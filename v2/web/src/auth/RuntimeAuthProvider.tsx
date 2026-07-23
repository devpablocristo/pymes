import {
  ClerkProvider,
  useAuth,
  useClerk,
  useUser,
} from "@clerk/react";
import {
  createHttpClient,
  type HttpClient,
} from "@devpablocristo/platform-http";
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
  type AuthOrganization,
} from "./AuthContext";
import { clearTenantCaches } from "./tenantCache";

type RuntimeConfig = components["schemas"]["RuntimeConfig"];
type OrganizationList = components["schemas"]["OrganizationList"];

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
          error,
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
  const [directoryError, setDirectoryError] = useState<string>();
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
    const client = createHttpClient({
      baseURL: "",
      fetch: fetchImpl,
      resolveHeaders: async () => {
        const token = await auth.getToken({ skipCache: true });
        if (!token) {
          throw new Error("session token unavailable");
        }
        return {
          Accept: "application/json",
          Authorization: `Bearer ${token}`,
        };
      },
    });
    setDirectoryStatus("loading");
    setDirectoryError(undefined);
    loadOrganizationDirectory(client, controller.signal)
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
        setDirectoryError(
          cause instanceof Error ? cause.message : "organization directory unavailable",
        );
      });

    return () => controller.abort();
  }, [auth.getToken, auth.isLoaded, auth.isSignedIn, auth.sessionId, fetchImpl]);

  const value = useMemo<AuthContextValue>(() => {
    if (!auth.isLoaded || (auth.isSignedIn && directoryStatus !== "ready")) {
      if (directoryStatus === "error") {
        return {
          ...loadingValue,
          status: "error",
          error: directoryError || "organization directory unavailable",
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
  client: HttpClient,
  signal: AbortSignal,
): Promise<OrganizationList["items"]> {
  const organizations: OrganizationList["items"] = [];
  const seenCursors = new Set<string>();
  let cursor: string | undefined;

  for (;;) {
    const query = new URLSearchParams({ limit: "100" });
    if (cursor) query.set("cursor", cursor);
    const response = await client.request<OrganizationList>(
      `/api/v1/organizations?${query.toString()}`,
      {
        signal,
        skipJSONContentType: true,
      },
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
