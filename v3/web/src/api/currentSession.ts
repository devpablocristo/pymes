import type { components } from "./generated";

export type CurrentSession = components["schemas"]["CurrentSession"];

type TokenProvider = () => Promise<string | null>;

type Problem = {
  code?: string;
  message?: string;
};

function endpoint(baseUrl: string): string {
  return `${baseUrl.replace(/\/$/, "")}/api/v1/session`;
}

export async function getCurrentSession(
  baseUrl: string,
  getToken: TokenProvider,
): Promise<CurrentSession> {
  const token = await getToken();
  if (!token) {
    throw new Error("Clerk no entregó un token para la organización activa.");
  }
  const response = await fetch(endpoint(baseUrl), {
    method: "GET",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    },
    credentials: "omit",
    cache: "no-store",
  });
  if (!response.ok) {
    let problem: Problem = {};
    try {
      problem = (await response.json()) as Problem;
    } catch {
      // El BFF puede estar detrás de un proxy que produzca una respuesta vacía.
    }
    throw new Error(
      problem.message ||
        (problem.code
          ? `No se pudo resolver la sesión (${problem.code}).`
          : `No se pudo resolver la sesión (${response.status}).`),
    );
  }
  return (await response.json()) as CurrentSession;
}
