import {
  createHttpClient,
  type HttpClient,
} from "@devpablocristo/platform-http";
import {
  createContext,
  useContext,
  useMemo,
  type PropsWithChildren,
} from "react";
import { useProductAuth } from "../auth/AuthContext";

const ProductApiContext = createContext<HttpClient | null>(null);

export function ProductApiProvider({
  children,
  client: clientOverride,
}: PropsWithChildren<{ client?: HttpClient }>) {
  const auth = useProductAuth();
  const client = useMemo(
    () =>
      clientOverride ??
      createHttpClient({
        baseURL: "",
        resolveHeaders: async () => {
          const token = await auth.getToken();
          if (!token) {
            throw new Error("session token unavailable");
          }
          return {
            Accept: "application/json",
            Authorization: `Bearer ${token}`,
          };
        },
      }),
    [auth.getToken, clientOverride],
  );

  return (
    <ProductApiContext.Provider value={client}>
      {children}
    </ProductApiContext.Provider>
  );
}

export function useProductApi() {
  const client = useContext(ProductApiContext);
  if (!client) {
    throw new Error("ProductApiProvider is missing");
  }
  return client;
}
