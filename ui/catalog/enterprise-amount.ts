// enterprise-amount.ts holds the enterprise support amount rule: a power of
// two from 8 to 4096 USD a month, halved with the "<<1" button and doubled
// with ">>1". The server checks the same rule (billing.EnterpriseAmount).

export const ENTERPRISE_MIN = 8;
export const ENTERPRISE_MAX = 4096;
export const ENTERPRISE_DEFAULT = 256;

export function isEnterpriseAmount(usd: number): boolean {
  return (
    Number.isInteger(usd) &&
    usd >= ENTERPRISE_MIN &&
    usd <= ENTERPRISE_MAX &&
    (usd & (usd - 1)) === 0
  );
}

export type Shift = "half" | "double";

// shift moves the amount one step; at either end it stays where it is.
export function shift(usd: number, step: Shift): number {
  const next = step === "double" ? usd * 2 : usd / 2;
  return isEnterpriseAmount(next) ? next : usd;
}
