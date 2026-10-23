// Treat zero as a real choice, not an absent value. Reject fractions, scientific
// notation, infinities and a stale draft exceeding the current server balance.
export function parseBid(draft: string, available: number): number | undefined {
  if (!/^\d+$/.test(draft)) return undefined;
  const value = Number(draft);
  return Number.isSafeInteger(value) && value >= 0 && value <= available
    ? value
    : undefined;
}
