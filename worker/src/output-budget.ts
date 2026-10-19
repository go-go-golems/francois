// One server-selected generated-token budget shared by every provider round.
export class OutputBudget {
  remaining: number;
  private admitted = 0;
  constructor(limit = 8192) {
    if (!Number.isSafeInteger(limit) || limit < 1 || limit > 65536)
      throw new Error("Invalid generated-token budget");
    this.remaining = limit;
  }
  next(modelLimit: number) {
    this.admitted = Math.min(modelLimit, 8192, this.remaining);
    if (this.admitted < 1) throw new Error("Generated-token budget exhausted");
    return this.admitted;
  }
  consume(output: number | undefined) {
    if (!Number.isSafeInteger(output) || output! < 0 || output! > this.admitted)
      throw new Error(
        "Provider returned unknown or invalid generated-token usage",
      );
    this.remaining -= output!;
  }
}
