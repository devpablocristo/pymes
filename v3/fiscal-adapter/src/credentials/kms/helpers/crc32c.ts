const CASTAGNOLI_REVERSED_POLYNOMIAL = 0x82f63b78;

const table = new Uint32Array(256);
for (let index = 0; index < table.length; index += 1) {
  let checksum = index;
  for (let bit = 0; bit < 8; bit += 1) {
    checksum = (checksum & 1) === 1
      ? (checksum >>> 1) ^ CASTAGNOLI_REVERSED_POLYNOMIAL
      : checksum >>> 1;
  }
  table[index] = checksum >>> 0;
}

export type KMSInteger =
  | number
  | string
  | bigint
  | { toString(): string };

export type KMSInt64Value = {
  value?: KMSInteger | null;
};

export function crc32c(value: Uint8Array): number {
  let checksum = 0xffffffff;
  for (const byte of value) {
    checksum = table[(checksum ^ byte) & 0xff]! ^ (checksum >>> 8);
  }
  return (checksum ^ 0xffffffff) >>> 0;
}

export function validCRC32C(
  expected: KMSInt64Value | null | undefined,
  value: Uint8Array,
): boolean {
  if (expected?.value === undefined || expected.value === null) return false;
  const encoded = expected.value.toString();
  if (!/^[0-9]+$/.test(encoded)) return false;
  const parsed = Number(encoded);
  return Number.isSafeInteger(parsed) &&
    parsed >= 0 &&
    parsed <= 0xffffffff &&
    parsed === crc32c(value);
}
