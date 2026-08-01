export interface PoolErrorSource {
  on(event: "error", listener: (error: Error) => void): unknown;
}

export interface DatabasePoolEvent {
  type: "fiscal_database_pool_error";
  code: string;
}

export function observePoolErrors(
  source: PoolErrorSource,
  report: (event: DatabasePoolEvent) => void,
): void {
  source.on("error", (error) => {
    const candidate =
      typeof error === "object" &&
      error !== null &&
      "code" in error &&
      typeof (error as { code: unknown }).code === "string"
        ? (error as { code: string }).code
        : "DATABASE_CONNECTION_LOST";
    report({
      type: "fiscal_database_pool_error",
      code: /^[A-Z0-9_]{1,64}$/.test(candidate)
        ? candidate
        : "DATABASE_CONNECTION_LOST",
    });
  });
}
