import { act, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { AuthContextValue } from "./AuthContext";
import { useProductAuth } from "./AuthContext";
import { RuntimeAuthProvider } from "./RuntimeAuthProvider";

const clerkMocks = vi.hoisted(() => {
  const getToken = vi.fn();
  const setActive = vi.fn();
  const signOut = vi.fn();
  const useOrganizationList = vi.fn();

  return {
    getToken,
    setActive,
    signOut,
    useOrganizationList,
    auth: {
      isLoaded: true,
      isSignedIn: true,
      sessionId: "sess_clerk_01",
      orgId: "org_clerk_norte" as string | undefined,
      getToken,
    },
    clerk: {
      setActive,
      signOut,
    },
    user: {
      id: "user_clerk_01",
      primaryEmailAddress: { emailAddress: "ana@example.test" },
      fullName: "Ana Pérez",
      imageUrl: "https://images.example.test/ana.png",
    },
  };
});

vi.mock("@clerk/react", () => ({
  ClerkProvider: ({ children }: { children: ReactNode }) => children,
  useAuth: () => clerkMocks.auth,
  useClerk: () => clerkMocks.clerk,
  useUser: () => ({ user: clerkMocks.user }),
  useOrganizationList: clerkMocks.useOrganizationList,
}));

const configuredRuntime = {
  auth: {
    provider: "clerk" as const,
    configured: true,
    publishable_key: "pk_test_pymes_v2",
  },
};

const organizationDirectory = {
  items: [
    {
      id: "11111111-1111-4111-8111-111111111111",
      switch_key: "org_clerk_norte",
      name: "Comercio Norte",
      slug: "comercio-norte",
      status: "active",
      role: "owner",
      sync_status: "synced",
    },
    {
      id: "22222222-2222-4222-8222-222222222222",
      switch_key: "org_clerk_sur",
      name: "Comercio Sur",
      slug: "comercio-sur",
      status: "active",
      role: "member",
      sync_status: "synced",
    },
  ],
  page: { total: 2 },
};

let observedAuth: AuthContextValue;

function AuthProbe() {
  const auth = useProductAuth();
  observedAuth = auth;

  return (
    <>
      <output data-testid="auth-status">{auth.status}</output>
      <output data-testid="active-organization">
        {auth.activeOrganizationId ?? "none"}
      </output>
      <output data-testid="organizations">{JSON.stringify(auth.organizations)}</output>
      <output data-testid="auth-error">{auth.error ?? ""}</output>
    </>
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderProvider(fetchImpl: typeof fetch) {
  return render(
    <RuntimeAuthProvider config={configuredRuntime} fetchImpl={fetchImpl}>
      <AuthProbe />
    </RuntimeAuthProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  clerkMocks.auth.isLoaded = true;
  clerkMocks.auth.isSignedIn = true;
  clerkMocks.auth.sessionId = "sess_clerk_01";
  clerkMocks.auth.orgId = "org_clerk_norte";
  clerkMocks.getToken.mockResolvedValue("jwt-from-clerk");
  clerkMocks.setActive.mockResolvedValue(undefined);
  clerkMocks.signOut.mockResolvedValue(undefined);
  clerkMocks.useOrganizationList.mockReturnValue({
    data: [
      {
        organization: {
          id: "org_clerk_not_admitted",
          name: "Organización sólo en Clerk",
        },
      },
    ],
  });
});

describe("RuntimeAuthProvider local organization authority", () => {
  test("loads the admitted directory with platform-http and maps local IDs and switch keys", async () => {
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl.mockResolvedValue(jsonResponse(organizationDirectory));

    renderProvider(fetchImpl);

    await waitFor(() => {
      expect(screen.getByTestId("auth-status")).toHaveTextContent("signed-in");
    });

    expect(fetchImpl).toHaveBeenCalledOnce();
    const [url, init] = fetchImpl.mock.calls[0];
    expect(url).toBe("/api/v1/organizations?limit=100");
    expect(init?.method).toBe("GET");
    const headers = new Headers(init?.headers);
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("Authorization")).toBe("Bearer jwt-from-clerk");
    expect(clerkMocks.useOrganizationList).not.toHaveBeenCalled();

    expect(observedAuth.organizations).toEqual([
      {
        id: "11111111-1111-4111-8111-111111111111",
        switchKey: "org_clerk_norte",
        name: "Comercio Norte",
        slug: "comercio-norte",
        role: "owner",
      },
      {
        id: "22222222-2222-4222-8222-222222222222",
        switchKey: "org_clerk_sur",
        name: "Comercio Sur",
        slug: "comercio-sur",
        role: "member",
      },
    ]);
    expect(observedAuth.activeOrganizationId).toBe(
      "11111111-1111-4111-8111-111111111111",
    );
  });

  test("follows every local organization-directory cursor", async () => {
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl
      .mockResolvedValueOnce(
        jsonResponse({
          items: [organizationDirectory.items[0]],
          page: { total: 2, next_cursor: "cursor-2" },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          items: [organizationDirectory.items[1]],
          page: { total: 2 },
        }),
      );

    renderProvider(fetchImpl);

    await waitFor(() => expect(observedAuth.status).toBe("signed-in"));
    expect(observedAuth.organizations).toHaveLength(2);
    expect(fetchImpl).toHaveBeenNthCalledWith(
      1,
      "/api/v1/organizations?limit=100",
      expect.any(Object),
    );
    expect(fetchImpl).toHaveBeenNthCalledWith(
      2,
      "/api/v1/organizations?limit=100&cursor=cursor-2",
      expect.any(Object),
    );
  });

  test("switches Clerk with the opaque switch key, refreshes the token, and clears tenant caches", async () => {
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl.mockResolvedValue(jsonResponse(organizationDirectory));
    const cacheCleared = vi.fn();
    window.addEventListener("pymes:tenant-cache-cleared", cacheCleared);

    renderProvider(fetchImpl);
    await waitFor(() => expect(observedAuth.status).toBe("signed-in"));

    await act(async () => {
      await observedAuth.setActiveOrganization(
        "22222222-2222-4222-8222-222222222222",
      );
    });

    expect(clerkMocks.setActive).toHaveBeenCalledWith({
      organization: "org_clerk_sur",
    });
    expect(clerkMocks.getToken).toHaveBeenLastCalledWith({ skipCache: true });
    expect(cacheCleared).toHaveBeenCalledOnce();

    window.removeEventListener("pymes:tenant-cache-cleared", cacheCleared);
  });

  test("clears tenant caches before switching even when token refresh fails", async () => {
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl.mockResolvedValue(jsonResponse(organizationDirectory));
    const cacheCleared = vi.fn();
    window.addEventListener("pymes:tenant-cache-cleared", cacheCleared);

    renderProvider(fetchImpl);
    await waitFor(() => expect(observedAuth.status).toBe("signed-in"));
    clerkMocks.getToken.mockRejectedValueOnce(new Error("refresh failed"));

    await expect(
      act(async () => {
        await observedAuth.setActiveOrganization(
          "22222222-2222-4222-8222-222222222222",
        );
      }),
    ).rejects.toThrow("refresh failed");

    expect(cacheCleared).toHaveBeenCalledOnce();
    expect(cacheCleared.mock.invocationCallOrder[0]).toBeLessThan(
      clerkMocks.setActive.mock.invocationCallOrder[0],
    );

    window.removeEventListener("pymes:tenant-cache-cleared", cacheCleared);
  });

  test("clears tenant caches before signing out", async () => {
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl.mockResolvedValue(jsonResponse(organizationDirectory));
    const cacheCleared = vi.fn();
    window.addEventListener("pymes:tenant-cache-cleared", cacheCleared);

    renderProvider(fetchImpl);
    await waitFor(() => expect(observedAuth.status).toBe("signed-in"));

    await act(async () => observedAuth.signOut());

    expect(cacheCleared).toHaveBeenCalledOnce();
    expect(cacheCleared.mock.invocationCallOrder[0]).toBeLessThan(
      clerkMocks.signOut.mock.invocationCallOrder[0],
    );

    window.removeEventListener("pymes:tenant-cache-cleared", cacheCleared);
  });

  test("does not expose a Clerk-active organization that is absent from the local directory", async () => {
    clerkMocks.auth.orgId = "org_clerk_not_admitted";
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl.mockResolvedValue(jsonResponse(organizationDirectory));

    renderProvider(fetchImpl);

    await waitFor(() => expect(observedAuth.status).toBe("signed-in"));
    expect(observedAuth.activeOrganizationId).toBeUndefined();
    expect(screen.getByTestId("active-organization")).toHaveTextContent("none");
    expect(
      observedAuth.organizations.some(
        (organization) => organization.switchKey === "org_clerk_not_admitted",
      ),
    ).toBe(false);
  });

  test.each([
    {
      name: "a 401 response",
      response: () =>
        Promise.resolve(
          jsonResponse(
            {
              error: {
                code: "AUTH_INVALID_TOKEN",
                message: "token rejected",
              },
            },
            401,
          ),
        ),
      expectedError: "token rejected",
    },
    {
      name: "a directory transport failure",
      response: () => Promise.reject(new Error("directory offline")),
      expectedError: "directory offline",
    },
  ])("fails closed when the organization directory returns $name", async ({
    response,
    expectedError,
  }) => {
    const fetchImpl = vi.fn<typeof fetch>();
    fetchImpl.mockImplementation(response);

    renderProvider(fetchImpl);

    await waitFor(() => {
      expect(screen.getByTestId("auth-status")).toHaveTextContent("error");
    });
    expect(observedAuth.organizations).toEqual([]);
    expect(screen.getByTestId("auth-error")).toHaveTextContent(expectedError);
  });
});
