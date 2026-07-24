// Applies hostContext to the document: theme variables, fonts, theme
// attribute, locale — and reports content size back to the host.
import { Bridge } from "./bridge";
import { HostContext } from "./protocol";
import { setLocale } from "./format";

const FONT_STYLE_ID = "gadget-host-fonts";

/** Document event fired after a hostContext has been applied. Behaviors
 * re-render on it because Intl formatting depends on host locale/timeZone. */
export const HOST_CONTEXT_EVENT = "gadget:hostcontext";

export function emitHostContextApplied(): void {
	document.dispatchEvent(new CustomEvent(HOST_CONTEXT_EVENT));
}

/**
 * Applies a hostContext (from ui/initialize or host-context-changed) to the
 * document. Only custom properties (keys starting with "--") are accepted
 * from styles.variables.
 */
export function applyHostContext(
  ctx: HostContext | null | undefined,
  root: HTMLElement = document.documentElement,
): void {
  if (!ctx) return;

  const vars = ctx.styles?.variables;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      if (k.startsWith("--") && typeof v === "string") {
        root.style.setProperty(k, v);
      }
    }
  }

  const fonts = ctx.styles?.css?.fonts;
  if (typeof fonts === "string" && fonts !== "") {
    let el = document.getElementById(FONT_STYLE_ID);
    if (!el) {
      el = document.createElement("style");
      el.id = FONT_STYLE_ID;
      document.head.appendChild(el);
    }
    el.textContent = fonts;
  }

  if (ctx.theme === "light" || ctx.theme === "dark") {
    root.setAttribute("data-gadget-theme", ctx.theme);
  }

  if (ctx.locale !== undefined || ctx.timeZone !== undefined) {
    setLocale(ctx.locale, ctx.timeZone);
  }
}

/**
 * Watches the document body and reports content size to the host via
 * ui/notifications/size-changed (view -> host per spec). Reports once
 * immediately; returns a stop function.
 */
export function watchSize(bridge: Bridge, el?: HTMLElement): () => void {
  const target = el ?? document.body;
  let raf = 0;

  const report = (): void => {
    // Width must echo the host-given viewport, not the content: wide tables live
    // in an overflow-x scroll wrap whose CSS min-width is 0, so reporting content
    // width lets the host shrink the frame, the wrap collapses, and the loop runs
    // to zero. Height legitimately grows with content, so keep scrollHeight.
    bridge.sizeChanged(document.documentElement.clientWidth, target.scrollHeight);
  };
  const schedule = (): void => {
    if (raf) return;
    raf = requestAnimationFrame(() => {
      raf = 0;
      report();
    });
  };

  report();
  if (typeof ResizeObserver === "undefined") {
    return () => {};
  }
  const ro = new ResizeObserver(schedule);
  ro.observe(target);
  return () => {
    ro.disconnect();
    if (raf) cancelAnimationFrame(raf);
  };
}
