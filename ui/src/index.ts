// gadget runtime entry point. Mounts the widget behavior found on the page,
// performs the MCP Apps handshake, applies host styling, and reports size.
//
// Boot order matters: the behavior mounts (and paints any baked initial
// data) BEFORE ui/initialize resolves, so widgets render even without a
// host; host context is applied when (and if) the handshake completes.
import { Bridge } from "./bridge";
import { CONFIG_ISLAND_ID, DATA_ISLAND_ID, readIsland } from "./data";
import { applyHostContext, emitHostContextApplied, watchSize } from "./host";
import { HostContext, M } from "./protocol";

export interface MountContext {
  root: HTMLElement;
  config: Record<string, unknown>;
  initialData: Record<string, unknown> | null;
  bridge: Bridge;
}

export type Behavior = (ctx: MountContext) => void;

const behaviors = new Map<string, Behavior>();

export function registerBehavior(kind: string, behavior: Behavior): void {
  behaviors.set(kind, behavior);
}

export async function boot(): Promise<void> {
  const root = document.querySelector<HTMLElement>("[data-gadget-widget]");
  if (!root) return;
  const kind = root.getAttribute("data-gadget-widget") ?? "";
  const config = readIsland<Record<string, unknown>>(CONFIG_ISLAND_ID) ?? {};
  const initialData = readIsland<Record<string, unknown>>(DATA_ISLAND_ID);

  const bridge = new Bridge();
  bridge.on(M.hostContextChanged, (params) => {
    applyHostContext(params as HostContext);
    emitHostContextApplied();
  });

  behaviors.get(kind)?.({ root, config, initialData, bridge });

  // Report size from first paint on — NOT gated on the handshake, so the
  // host can size the frame while ui/initialize is still in flight.
  watchSize(bridge);

  try {
    const hostCtx = await bridge.initialize();
    applyHostContext(hostCtx);
    // Rows may have painted before the handshake; let behaviors re-render
    // with the host's locale/timeZone applied.
    emitHostContextApplied();
  } catch {
    // No responding host (standalone preview, harness without init support):
    // the widget still renders with fallback tokens.
  }
}

// Widget behaviors register here as they are implemented.
import { mountForm } from "./widgets/form";
import { mountTable } from "./widgets/table";

registerBehavior("table", mountTable);
registerBehavior("form", mountForm);

if (typeof document !== "undefined" && "addEventListener" in document) {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => void boot());
  } else {
    void boot();
  }
}
