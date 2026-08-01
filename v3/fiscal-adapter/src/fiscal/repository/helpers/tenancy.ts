import { FiscalError } from "../../usecases/domain/fiscal.js";

export function opaqueReference(value: unknown): value is string {
  return (
    typeof value === "string" &&
    /^[A-Za-z0-9:_./-]{1,255}$/.test(value)
  );
}

export function voucherOrganization(voucherKey: string): string {
  const organizationId = voucherKey.split("/", 1)[0] ?? "";
  if (!opaqueReference(organizationId)) {
    throw new FiscalError("VALIDATION_ERROR");
  }
  return organizationId;
}
