// Status-bar text derivation, shared by every widget behavior.
//
// A tool result's first text block is very often not prose: the spec suggests
// mirroring structuredContent into a text block when the handler leaves the
// content field empty, and SDKs (the Go one included) do it automatically. That
// payload must never reach the status bar, so the helpers here skip it and let
// the widget fall back to its own wording.
import { CallToolResult } from "./protocol";

/** True when text is a serialized JSON object or array rather than a message. */
export function isJSONPayload(text: string): boolean {
  const t = text.trim();
  const open = t[0];
  const close = t[t.length - 1];
  if (!((open === "{" && close === "}") || (open === "[" && close === "]"))) return false;
  try {
    JSON.parse(t);
    return true;
  } catch {
    return false;
  }
}

/** First human-readable text block of a tool result. Blank blocks and JSON
 * payloads are skipped, so undefined means "the server sent no message". */
export function textOf(res: CallToolResult): string | undefined {
  for (const block of res.content ?? []) {
    if (block.type !== "text" || typeof block.text !== "string") continue;
    const text = block.text.trim();
    if (text === "" || isJSONPayload(text)) continue;
    return text;
  }
  return undefined;
}

/** Status message for a rejected call. Hosts can reject with a JSON-RPC error
 * whose message is itself a JSON payload, which the fallback replaces. */
export function errorText(e: unknown, fallback: string): string {
  const msg = (e instanceof Error ? e.message : String(e)).trim();
  return msg === "" || isJSONPayload(msg) ? fallback : msg;
}
