import { FiscalError } from "../../usecases/domain/fiscal.js";

export interface ExactDecimal {
  coefficient: bigint;
  scale: number;
}

export function parseDecimal(value: string, maxScale = 2): ExactDecimal {
  const match = value.match(/^(0|[1-9]\d*)(?:\.(\d+))?$/);
  if (match === null || (match[2]?.length ?? 0) > maxScale) {
    throw new FiscalError("VALIDATION_ERROR", `invalid decimal scale: ${value}`);
  }
  const fraction = match[2] ?? "";
  return {
    coefficient: BigInt(`${match[1]}${fraction}`),
    scale: fraction.length,
  };
}

export function decimalNumber(value: string, maxScale = 2): number {
  const decimal = parseDecimal(value, maxScale);
  const number = Number(decimal.coefficient) / 10 ** decimal.scale;
  if (
    !Number.isFinite(number) ||
    decimal.coefficient > BigInt(Number.MAX_SAFE_INTEGER)
  ) {
    throw new FiscalError("VALIDATION_ERROR", "decimal exceeds safe ARCA range");
  }
  return number;
}

export function addDecimals(values: string[], scale = 2): bigint {
  return values.reduce(
    (sum, value) => sum + rescale(parseDecimal(value, scale), scale),
    0n,
  );
}

export function multiplyRateHalfUp(
  baseAtScale2: bigint,
  rate: string,
): bigint {
  const parsedRate = parseDecimal(rate, 1);
  const rateAtScale1 = rescale(parsedRate, 1);
  return (baseAtScale2 * rateAtScale1 + 500n) / 1000n;
}

export function centsToNumber(value: bigint): number {
  if (value > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new FiscalError("VALIDATION_ERROR", "amount exceeds safe ARCA range");
  }
  return Number(value) / 100;
}

function rescale(value: ExactDecimal, targetScale: number): bigint {
  if (value.scale > targetScale) {
    throw new FiscalError("VALIDATION_ERROR", "decimal cannot be rescaled exactly");
  }
  return value.coefficient * 10n ** BigInt(targetScale - value.scale);
}
