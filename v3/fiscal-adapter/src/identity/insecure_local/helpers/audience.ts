export function assertFiscalAudience(audience: string): void {
  if (audience !== "fiscal") throw new Error("UNAUTHORIZED_SERVICE");
}
