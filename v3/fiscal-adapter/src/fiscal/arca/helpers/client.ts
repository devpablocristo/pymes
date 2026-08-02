import type {
  ExplicitSDKBaseClient,
  ExplicitSDKClient,
  ExplicitSDKSequenceClient,
  SDKLastAuthorizedVoucher,
  SDKPointOfSale,
  SDKVoucherSequenceReference,
} from "../models/sdk.js";

/**
 * Keeps all Fiscal Adapter calls behind the published explicit-numbering
 * entrypoint while validating its normalized point-of-sale response.
 */
export function validatedExplicitClient(
  explicitClient: ExplicitSDKBaseClient & ExplicitSDKSequenceClient,
): ExplicitSDKClient {
  return {
    authorize: (request) => explicitClient.authorize(request),
    consult: (reference) => explicitClient.consult(reference),
    async lastAuthorizedVoucher(
      reference: SDKVoucherSequenceReference,
    ): Promise<SDKLastAuthorizedVoucher> {
      const result = await explicitClient.lastAuthorizedVoucher(reference);
      if (
        result.pointOfSale !== reference.pointOfSale ||
        result.voucherType !== reference.voucherType ||
        !Number.isSafeInteger(result.voucherNumber) ||
        result.voucherNumber < 0
      ) {
        throw invalidLastAuthorizedVoucherResponse();
      }
      return result;
    },
    async listPointsOfSale(): Promise<SDKPointOfSale[]> {
      return (await explicitClient.listPointsOfSale()).map(
        normalizePointOfSale,
      );
    },
  };
}

function invalidLastAuthorizedVoucherResponse(): Error {
  const error = new Error(
    "ARCA returned an invalid last-authorized voucher response",
  );
  error.name = "ExplicitPointOfSaleError";
  Object.assign(error, {
    code: "INVALID_LAST_AUTHORIZED_VOUCHER_RESPONSE",
  });
  return error;
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
