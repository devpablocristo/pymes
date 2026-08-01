export type WebConfig = {
  apiBaseUrl: string;
  clerkPublishableKey: string | null;
  allowInsecureLocalAuth: boolean;
  useFakeApi: boolean;
  localOrganizationId: string | null;
  publicOrganizationSlug: string;
};

function enabled(value: string | undefined): boolean {
  return value === "true" || value === "1";
}

export function loadWebConfig(): WebConfig {
  const clerkPublishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY?.trim() || null;
  const allowInsecureLocalAuth = enabled(import.meta.env.VITE_ALLOW_INSECURE_LOCAL_AUTH);
  const useFakeApi = enabled(import.meta.env.VITE_USE_FAKE_API);
  const e2eBuild = import.meta.env.MODE === "e2e";
  if (import.meta.env.PROD && !e2eBuild && allowInsecureLocalAuth) {
    throw new Error("VITE_ALLOW_INSECURE_LOCAL_AUTH no puede habilitarse en producción.");
  }
  if (import.meta.env.PROD && !e2eBuild && useFakeApi) {
    throw new Error("VITE_USE_FAKE_API no puede habilitarse en producción.");
  }
  return {
    apiBaseUrl: import.meta.env.VITE_API_BASE_URL?.replace(/\/$/, "") || "",
    clerkPublishableKey,
    allowInsecureLocalAuth,
    useFakeApi,
    localOrganizationId: import.meta.env.VITE_PYMES_ORGANIZATION_ID?.trim() || null,
    publicOrganizationSlug: import.meta.env.VITE_PYMES_ORGANIZATION_SLUG?.trim() || "demo",
  };
}
