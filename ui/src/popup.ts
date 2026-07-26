// Anchored popups: the plumbing shared by the dropdown (a listbox over a
// <select>) and the action menu (a row's or a selection's actions).
//
// Two things about a popup are properties of the set, not of any one of them,
// so they live here rather than in either module: only one may be open at a
// time, and a press anywhere else closes whichever is. The third is placement,
// which both solve identically — a panel is a child of the widget root, not of
// its trigger, because the card chrome and the table's scroll container clip
// their overflow and a nested panel would be cut off at their edge. Its
// coordinates are therefore written relative to that root.
//
// "One at a time" is a stack rather than a single slot, because a popup can be
// opened from inside another: the calendar's month and year dropdowns live in
// the calendar's own panel. Opening one closes every popup it is not nested
// inside, and an outside press peels the stack from the top — so pressing the
// calendar closes the dropdown over it without taking the calendar with it.

/** Gap between a trigger and its panel, and the room a panel needs below the
 * trigger before it flips above it. */
export const PANEL_GAP_PX = 4;
/** Room a panel keeps from the viewport edge when it has to slide inward. */
const VIEWPORT_MARGIN_PX = 8;

export interface Popup {
  /** The trigger, or whatever else counts as "inside" for an outside press. */
  readonly anchor: HTMLElement;
  readonly panel: HTMLElement;
  close(focus?: boolean): void;
  position(): void;
}

// Open popups, outermost first. A popup is nested inside the one below it when
// that one's panel holds its anchor.
let stack: Popup[] = [];
let globalsBound = false;

/** The element a popup's panel is appended to and positioned against. */
export function popupHost(el: Element): HTMLElement {
  return el.closest<HTMLElement>(".gadget-root") ?? document.body;
}

/**
 * Records p as open, closing every open popup p is not nested inside. A nested
 * panel hangs off the widget root like any other (see popupHost), so ancestry
 * is read from where the trigger sits, not from where the panel was appended.
 */
export function openPopup(p: Popup): void {
  for (let i = stack.length - 1; i >= 0; i--) {
    const q = stack[i] as Popup;
    if (q === p || q.panel.contains(p.anchor)) break;
    stack.pop();
    q.close();
  }
  if (!stack.includes(p)) stack.push(p);
  bindGlobals();
}

/** Drops p, and anything opened from inside it, from the open stack. Safe to
 * call for a popup that is not in it. */
export function releasePopup(p: Popup): void {
  const i = stack.indexOf(p);
  if (i < 0) return;
  // Splice first: each close() calls back in here, and by then its entry is
  // already gone, so the recursion stops at one level.
  const dropped = stack.splice(i);
  for (let k = dropped.length - 1; k >= 1; k--) (dropped[k] as Popup).close();
}

export interface PlaceOptions {
  /** Which edge of the panel lines up with the trigger's. Defaults to start. */
  align?: "start" | "end";
  /** Widens the panel to at least the trigger's width. */
  matchWidth?: boolean;
}

/**
 * Places panel against trigger in host's coordinate space: below it unless the
 * viewport has no room there and more above, and slid inward when the aligned
 * edge would take it off-screen.
 */
export function positionPanel(
  trigger: HTMLElement,
  panel: HTMLElement,
  host: HTMLElement,
  opts: PlaceOptions = {},
): void {
  const t = trigger.getBoundingClientRect();
  const h0 = host.getBoundingClientRect();
  if (opts.matchWidth) panel.style.minWidth = `${t.width}px`;

  const width = panel.offsetWidth;
  const height = panel.offsetHeight;

  let left = opts.align === "end" ? t.right - width : t.left;
  const rightLimit = Math.max(VIEWPORT_MARGIN_PX, window.innerWidth - width - VIEWPORT_MARGIN_PX);
  left = Math.min(Math.max(left, VIEWPORT_MARGIN_PX), rightLimit);
  panel.style.left = `${left - h0.left}px`;

  const above =
    window.innerHeight - t.bottom < height + PANEL_GAP_PX && t.top > height + PANEL_GAP_PX;
  panel.classList.toggle("gadget-pop-panel--above", above);
  panel.style.top = above
    ? `${t.top - h0.top - height - PANEL_GAP_PX}px`
    : `${t.bottom - h0.top + PANEL_GAP_PX}px`;
}

function bindGlobals(): void {
  if (globalsBound || typeof document === "undefined") return;
  globalsBound = true;

  document.addEventListener(
    "pointerdown",
    (ev) => {
      const target = ev.target;
      if (!(target instanceof Node)) return;
      // Peel from the top: a press inside the calendar is outside the dropdown
      // over it, and closes only that one.
      for (let i = stack.length - 1; i >= 0; i--) {
        const p = stack[i] as Popup;
        if (p.anchor.contains(target) || p.panel.contains(target)) break;
        p.close();
      }
    },
    true,
  );

  const reposition = (): void => {
    for (const p of [...stack]) p.position();
  };
  window.addEventListener("resize", reposition);
  window.addEventListener("scroll", reposition, true);
}
