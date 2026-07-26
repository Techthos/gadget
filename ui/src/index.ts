// gomukit runtime entry point. Mounts the widget behavior found on the page,
// performs the MCP Apps handshake, applies host styling, and reports size.
//
// Boot order matters: the behavior mounts (and paints any baked initial
// data) BEFORE ui/initialize resolves, so widgets render even without a
// host; host context is applied when (and if) the handshake completes.
import { Bridge } from "./bridge";
import { CONFIG_ISLAND_ID, DATA_ISLAND_ID, readIsland } from "./data";
import { delegate } from "./dom";
import { enhanceSelects } from "./dropdown";
import { applyHostContext, emitHostContextApplied, watchSize } from "./host";
import { HostContext, M } from "./protocol";

export interface MountContext {
  root: HTMLElement;
  config: Record<string, unknown>;
  initialData: Record<string, unknown> | null;
  bridge: Bridge;
  // ready resolves after the ui/initialize handshake: true when a host
  // answered, false otherwise (standalone preview / no host). Behaviors gate
  // load-time hydration on it so tool calls only fire against a live host.
  ready?: Promise<boolean>;
}

export type Behavior = (ctx: MountContext) => void;

const behaviors = new Map<string, Behavior>();

export function registerBehavior(kind: string, behavior: Behavior): void {
  behaviors.set(kind, behavior);
}

export async function boot(): Promise<void> {
  const root = document.querySelector<HTMLElement>("[data-gomu-widget]");
  if (!root) return;
  const kind = root.getAttribute("data-gomu-widget") ?? "";
  const config = readIsland<Record<string, unknown>>(CONFIG_ISLAND_ID) ?? {};
  const initialData = readIsland<Record<string, unknown>>(DATA_ISLAND_ID);

  const bridge = new Bridge();
  bridge.on(M.hostContextChanged, (params) => {
    applyHostContext(params as HostContext);
    emitHostContextApplied();
  });

  // Resolve true once the handshake completes (false if no host answers).
  // Behaviors await this before hydrating so load-time tool calls only fire
  // against a live host. Rows may have painted before the handshake; applying
  // host context and re-emitting lets behaviors re-render with the host's
  // locale/timeZone.
  const ready = bridge
    .initialize()
    .then((hostCtx) => {
      applyHostContext(hostCtx);
      emitHostContextApplied();
      return true;
    })
    .catch(() => false);

  // Brand chrome is server-rendered for every widget kind, so its link is
  // wired here rather than in each behavior. It is a button, not an anchor:
  // navigation inside the host's sandboxed iframe is blocked, so the URL goes
  // to the host through ui/openLink.
  delegate(root, "click", "brand", (_el, url) => {
    if (url !== "") void bridge.openLink(url);
  });

  // Every server-rendered <select> becomes a gomukit dropdown before the
  // behavior mounts. The select survives the upgrade, so behaviors keep
  // finding and driving the same element.
  enhanceSelects(root);

  behaviors.get(kind)?.({ root, config, initialData, bridge, ready });

  // Report size from first paint on — NOT gated on the handshake, so the
  // host can size the frame while ui/initialize is still in flight.
  watchSize(bridge);

  await ready;
}

// Widget behaviors register here as they are implemented.
import { mountForm } from "./widgets/form";
import { mountTable } from "./widgets/table";
import { mountCard } from "./widgets/card";
import { mountCardList } from "./widgets/cardlist";
import { mountMenu } from "./widgets/menu";
import { mountConfirm } from "./widgets/confirm";
import { mountChoice } from "./widgets/choice";
import { mountDatePicker } from "./widgets/datepicker";

registerBehavior("table", mountTable);
registerBehavior("form", mountForm);
registerBehavior("card", mountCard);
registerBehavior("cardlist", mountCardList);
registerBehavior("menu", mountMenu);
registerBehavior("confirm", mountConfirm);
registerBehavior("choice", mountChoice);
registerBehavior("datepicker", mountDatePicker);

if (typeof document !== "undefined" && "addEventListener" in document) {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => void boot());
  } else {
    void boot();
  }
}
