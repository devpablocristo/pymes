import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { lazy, Suspense, useEffect } from "react";
import { GatewayProvider } from "./api/GatewayContext";
import type { WebConfig } from "./config";

const AdminRoute = lazy(() => import("./routes/AdminRoute"));
const PublicBookingPage = lazy(() =>
  import("./pages/PublicBookingPage").then((module) => ({ default: module.PublicBookingPage })),
);
const PublicActionPage = lazy(() =>
  import("./pages/PublicActionPage").then((module) => ({ default: module.PublicActionPage })),
);

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 20_000,
      retry: 1,
      refetchOnWindowFocus: true,
    },
    mutations: {
      retry: false,
    },
  },
});

function RedirectToAgenda() {
  useEffect(() => {
    window.location.replace("/app/agenda");
  }, []);
  return <main className="public-loading">Abriendo agenda…</main>;
}

type WebRoute =
  | { kind: "health" }
  | { kind: "admin" }
  | { kind: "booking"; value: string }
  | { kind: "action"; value: string }
  | { kind: "redirect" };

function routeFor(pathname: string): WebRoute {
  if (pathname === "/healthz") {
    return { kind: "health" };
  }
  if (pathname === "/app/agenda") {
    return { kind: "admin" };
  }
  const bookingMatch = pathname.match(/^\/reservar\/([^/]+)\/?$/);
  if (bookingMatch) {
    return { kind: "booking", value: decodeURIComponent(bookingMatch[1]) };
  }
  const actionMatch = pathname.match(/^\/agenda\/accion\/([^/]+)\/?$/);
  if (actionMatch) {
    return { kind: "action", value: decodeURIComponent(actionMatch[1]) };
  }
  return { kind: "redirect" };
}

export function App({ config }: { config: WebConfig }) {
  const route = routeFor(window.location.pathname);
  const page =
    route.kind === "health" ? (
      <span className="health-response">ok</span>
    ) : route.kind === "admin" ? (
      <AdminRoute config={config} />
    ) : route.kind === "booking" ? (
      <PublicBookingPage defaultSlug={config.publicOrganizationSlug} organizationSlug={route.value} />
    ) : route.kind === "action" ? (
      <PublicActionPage token={route.value} search={window.location.search} />
    ) : (
      <RedirectToAgenda />
    );

  return (
    <QueryClientProvider client={queryClient}>
      <GatewayProvider config={config}>
        <Suspense fallback={<main className="public-loading">Cargando…</main>}>{page}</Suspense>
      </GatewayProvider>
    </QueryClientProvider>
  );
}
