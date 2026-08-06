// Modal popups: the plumbing shared by the dropdown (a listbox over a
// <select>), the action menu (a row's or a selection's actions) and the date
// field's calendar. Each opens as a centered overlay that dims the whole
// iframe, rather than a panel anchored to its trigger — a content-sized iframe
// often has no room to drop a panel below a control near its bottom edge, and
// the card chrome clips whatever hangs past it. A modal escapes both: it is
// fixed to the iframe viewport and scrolls inside itself.
//
// Two things about a popup are properties of the set, not of any one of them,
// so they live here rather than in either module: only one branch may be open
// at a time, and a press on the backdrop closes whichever is on top.
//
// "One at a time" is a stack rather than a single slot, because a popup can be
// opened from inside another: the calendar's month and year dropdowns live in
// the calendar's own panel. Opening one closes every popup it is not nested
// inside, and an outside press peels the stack from the top — so pressing the
// calendar's backdrop closes the dropdown over it without taking the calendar
// with it.

import { h } from "./dom";

export interface Popup {
  /** The trigger, or whatever else counts as "inside" for an outside press. */
  readonly anchor: HTMLElement;
  readonly panel: HTMLElement;
  close(focus?: boolean): void;
}

// Open popups, outermost first. A popup is nested inside the one below it when
// that one's panel holds its anchor.
let stack: Popup[] = [];
let globalsBound = false;

/** The element a popup's overlay is appended to. */
export function popupHost(el: Element): HTMLElement {
  return el.closest<HTMLElement>(".gomu-root") ?? document.body;
}

/**
 * Wraps panel in a centered, dimmed overlay and mounts it in host, hidden. The
 * overlay hangs off the widget root rather than the field, so the card chrome
 * cannot clip it. Consumers toggle the returned element's `hidden` to open and
 * close, and keep the panel to fill.
 */
export function mountOverlay(host: HTMLElement, panel: HTMLElement): HTMLElement {
  const overlay = h("div", { class: "gomu-pop-overlay", hidden: true });
  overlay.append(panel);
  host.append(overlay);
  return overlay;
}

/** Locks the document scroller while any overlay is open. A fixed overlay
 * already covers the frame; this stops the content behind it drifting. */
function syncScrollLock(): void {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("gomu-modal-open", stack.length > 0);
}

/**
 * Records p as open, closing every open popup p is not nested inside. A nested
 * overlay hangs off the widget root like any other (see popupHost), so
 * ancestry is read from where the trigger sits, not from where the panel was
 * appended.
 */
export function openPopup(p: Popup): void {
  for (let i = stack.length - 1; i >= 0; i--) {
    const q = stack[i] as Popup;
    if (q === p || q.panel.contains(p.anchor)) break;
    stack.pop();
    q.close();
  }
  if (!stack.includes(p)) stack.push(p);
  syncScrollLock();
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
  syncScrollLock();
}

function bindGlobals(): void {
  if (globalsBound || typeof document === "undefined") return;
  globalsBound = true;

  document.addEventListener(
    "pointerdown",
    (ev) => {
      const target = ev.target;
      if (!(target instanceof Node)) return;
      // Peel from the top: a press on the calendar's backdrop is outside the
      // dropdown over it, and closes only that one.
      for (let i = stack.length - 1; i >= 0; i--) {
        const p = stack[i] as Popup;
        if (p.anchor.contains(target) || p.panel.contains(target)) break;
        p.close();
      }
    },
    true,
  );
}
