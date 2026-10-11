type Failure = { code: string; message: string; text?: string };
const error = (code: string, message: string, text?: string): Failure => ({
  code,
  message,
  ...(text ? { text } : {}),
});
const bytes = (s: string) => new TextEncoder().encode(s).length;
const plain = (v: unknown): v is Record<string, unknown> =>
  v !== null &&
  typeof v === "object" &&
  (Object.getPrototypeOf(v) === Object.prototype ||
    Object.getPrototypeOf(v) === null);
export function normalizeInput(
  input: unknown,
): string | { role: "user" | "assistant"; content: string }[] {
  if (typeof input === "string") {
    if (!input.trim()) throw error("invalid_request", "Prompt is empty");
    if (bytes(input) > 262144)
      throw error("prompt_too_large", "Prompt exceeds 256 KiB");
    return input;
  }
  if (!Array.isArray(input) || !input.length)
    throw error("invalid_request", "Input must be text or turns");
  const turns: { role: "user" | "assistant"; content: string }[] = [];
  for (const t of input) {
    if (
      !plain(t) ||
      (t.role !== "user" && t.role !== "assistant") ||
      typeof t.content !== "string"
    )
      throw error("invalid_request", "Invalid turn");
    const last = turns.at(-1);
    if (last?.role === t.role) last.content += "\n\n" + t.content;
    else turns.push({ role: t.role, content: t.content });
  }
  if (turns[0].role !== "user" || turns.at(-1)!.role !== "user")
    throw error("invalid_request", "Turns must start and end with user");
  if (bytes(turns.map((t) => t.content).join("")) > 262144)
    throw error("prompt_too_large", "Prompt exceeds 256 KiB");
  return turns;
}
export function parseJSON(text: string, truncated = false): unknown {
  if (truncated)
    throw error("invalid_json", "JSON response was cut short", text);
  const fence = text.match(/```(?:json)?\s*\n?([\s\S]*?)```/i);
  const first = text.search(/[\[{]/);
  const last = Math.max(text.lastIndexOf("}"), text.lastIndexOf("]"));
  for (const candidate of [
    text,
    fence?.[1],
    first >= 0 && last >= first ? text.slice(first, last + 1) : undefined,
  ]) {
    if (candidate !== undefined) {
      try {
        return JSON.parse(candidate);
      } catch {}
    }
  }
  throw error("invalid_json", "Response does not contain complete JSON", text);
}
