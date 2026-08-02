export function bearerToken(authorization: string | undefined): string {
  if (authorization === undefined || !authorization.startsWith("Bearer ")) {
    throw new Error("missing bearer token");
  }
  const token = authorization.slice("Bearer ".length);
  if (token.length < 1 || token.includes(" ")) {
    throw new Error("invalid bearer token");
  }
  return token;
}

export function decodeJSON<T>(value: string): T {
  return JSON.parse(decodeBase64URL(value).toString("utf8")) as T;
}

export function decodeBase64URL(value: string): Buffer {
  if (!nonEmpty(value) || !/^[A-Za-z0-9_-]+$/.test(value)) {
    throw new Error("invalid base64url");
  }
  const decoded = Buffer.from(value, "base64url");
  if (decoded.toString("base64url") !== value) {
    throw new Error("non-canonical base64url");
  }
  return decoded;
}

export function nonEmpty(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

export function opaqueReference(value: unknown): value is string {
  return (
    typeof value === "string" &&
    /^[A-Za-z0-9:_./-]{1,255}$/.test(value)
  );
}

export function optionalOpaqueReference(
  value: unknown,
): value is string | undefined {
  return value === undefined || opaqueReference(value);
}

export function isRecord(
  value: unknown,
): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
