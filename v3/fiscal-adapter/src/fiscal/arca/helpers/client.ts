import type {
  ExplicitSDKBaseClient,
  ExplicitSDKClient,
  SDKLegacyPointOfSale,
  SDKLegacyPointOfSaleClient,
  SDKPointOfSale,
  SDKPointOfSaleListingClient,
} from "../models/sdk.js";

/**
 * Compatibility boundary for the published 2.4 SDK and the next explicit API.
 *
 * Version 2.4 already exposes FEParamGetPtosVenta through the root client but
 * not through its explicit-numbering subpath. Once the next SDK version is
 * published, this adapter automatically prefers `listPointsOfSale`; until
 * then it uses only the root client's read-only `getPuntosVenta` operation.
 */
export function compatibleExplicitClient(
  explicitClient: ExplicitSDKBaseClient,
  legacyPointOfSaleClient: SDKLegacyPointOfSaleClient,
): ExplicitSDKClient {
  return {
    authorize: (request) => explicitClient.authorize(request),
    consult: (reference) => explicitClient.consult(reference),
    async listPointsOfSale(): Promise<SDKPointOfSale[]> {
      if (supportsPointOfSaleListing(explicitClient)) {
        return (await explicitClient.listPointsOfSale()).map(
          normalizePointOfSale,
        );
      }
      return (await legacyPointOfSaleClient.getPuntosVenta()).map(
        normalizeLegacyPointOfSale,
      );
    },
  };
}

function supportsPointOfSaleListing(
  client: ExplicitSDKBaseClient,
): client is ExplicitSDKBaseClient & SDKPointOfSaleListingClient {
  return (
    "listPointsOfSale" in client &&
    typeof (client as { listPointsOfSale?: unknown }).listPointsOfSale ===
      "function"
  );
}

function normalizeLegacyPointOfSale(
  value: SDKLegacyPointOfSale,
): SDKPointOfSale {
  const blocked = normalizedString(value.Bloqueado).toUpperCase();
  if (blocked !== "S" && blocked !== "N") {
    throw invalidPointOfSaleResponse();
  }
  const deactivatedOn = normalizedString(value.FchBaja);
  return normalizePointOfSale({
    number: value.Nro,
    emissionType: normalizedString(value.EmisionTipo),
    blocked: blocked === "S",
    ...(deactivatedOn === "" || deactivatedOn.toUpperCase() === "NULL"
      ? {}
      : { deactivatedOn }),
  });
}

function normalizePointOfSale(value: SDKPointOfSale): SDKPointOfSale {
  if (
    !Number.isSafeInteger(value.number) ||
    value.number <= 0 ||
    normalizedString(value.emissionType) === "" ||
    typeof value.blocked !== "boolean" ||
    (value.deactivatedOn !== undefined &&
      normalizedString(value.deactivatedOn) === "")
  ) {
    throw invalidPointOfSaleResponse();
  }
  return {
    number: value.number,
    emissionType: normalizedString(value.emissionType),
    blocked: value.blocked,
    ...(value.deactivatedOn === undefined
      ? {}
      : { deactivatedOn: normalizedString(value.deactivatedOn) }),
  };
}

function invalidPointOfSaleResponse(): Error {
  const error = new Error("ARCA returned an invalid point-of-sale response");
  error.name = "ExplicitPointOfSaleError";
  Object.assign(error, { code: "INVALID_POINT_OF_SALE_RESPONSE" });
  return error;
}

function normalizedString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
