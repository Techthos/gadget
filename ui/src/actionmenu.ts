// Action menu: a "⋯" trigger and the popup of actions it opens, used for a
// table row's actions and for the bulk actions of a selection.
//
// Actions are a menu rather than a strip of buttons because their number is an
// authoring decision the layout cannot absorb: three buttons in every row set
// the width of a column that carries no data, and the row's own values are
// what should have that space. One trigger costs the same whatever is behind
// it, and the labels only exist while they are being read.
//
// One panel serves the whole widget: it is filled from the binding the trigger
// resolves to at the moment it is pressed, so nothing has to be built (or
// cleaned up) for rows that are re-rendered on every state change.
//
// Data reaches the DOM through textContent only (see dom.ts); action labels
// are no exception.
import { confirmAction } from "./confirm-modal";
import { clear, delegate, h, icon } from "./dom";
import { mountOverlay, openPopup, popupHost, releasePopup, type Popup } from "./popup";

// Three dots in a 16x16 viewBox: near-zero runs, painted as dots by the round
// line cap.
const DOTS_PATHS = ["M3.5 8h.01", "M8 8h.01", "M12.5 8h.01"];

/** The parts of an action config this menu renders. */
export interface MenuAction {
  label: string;
  variant?: string;
  confirm?: string;
}

/** What a trigger stands for: the actions to show and what a choice means. */
export interface MenuBinding {
  items: MenuAction[];
  onSelect(index: number): void;
}

export interface ActionMenu {
  /**
   * Wires every `data-gomu-<attr>` trigger under root, now and in the
   * future: resolve is asked what the pressed one stands for, and returning
   * null (or no actions) leaves the press unanswered.
   */
  bind(
    root: HTMLElement,
    attr: string,
    resolve: (el: HTMLElement, value: string) => MenuBinding | null,
  ): void;
  close(focus?: boolean): void;
  isOpen(): boolean;
}

let seq = 0;

/** The "⋯" button that opens an action menu. attrs go on the button. */
export function actionMenuTrigger(
  attrs: Record<string, string | boolean | null | undefined> = {},
  label = "Actions",
): HTMLButtonElement {
  const btn = h("button", {
    type: "button",
    class: "gomu-btn gomu-action-trigger",
    "aria-haspopup": "menu",
    "aria-expanded": "false",
    "aria-label": label,
    ...attrs,
  }) as HTMLButtonElement;
  btn.append(icon("gomu-action-dots", ...DOTS_PATHS));
  return btn;
}

/** Creates the widget's single action-menu popup. */
export function createActionMenu(root: HTMLElement): ActionMenu {
  const host = popupHost(root);
  const id = `gomu-menu-${++seq}`;
  // Visibility lives on the overlay (see popup.ts); the panel itself is never
  // hidden, or [hidden]'s display:none would blank it inside a shown overlay.
  const panel = h("div", {
    class: "gomu-pop-panel gomu-action-panel",
    id,
    role: "menu",
  });
  const overlay = mountOverlay(host, panel);

  let trigger: HTMLElement | null = null;
  let binding: MenuBinding | null = null;
  let itemEls: HTMLElement[] = [];
  let active = -1;

  const popup: Popup = {
    get anchor(): HTMLElement {
      return trigger ?? panel;
    },
    panel,
    close: (focus?: boolean) => close(focus),
  };

  function isOpen(): boolean {
    return !overlay.hidden;
  }

  function build(items: MenuAction[]): void {
    clear(panel);
    itemEls = items.map((action, i) => {
      let cls = "gomu-action-item";
      if (action.variant) cls += ` gomu-action-item--${action.variant}`;
      const el = h(
        "div",
        {
          class: cls,
          role: "menuitem",
          tabindex: "-1",
          id: `${id}-i${i}`,
          "data-gomu-action-index": String(i),
        },
        action.label,
      );
      panel.append(el);
      return el;
    });
  }

  function open(el: HTMLElement, b: MenuBinding): void {
    if (b.items.length === 0) return;
    trigger = el;
    binding = b;
    build(b.items);
    overlay.hidden = false;
    el.setAttribute("aria-expanded", "true");
    el.setAttribute("aria-controls", id);
    const name = el.getAttribute("aria-label") ?? el.textContent ?? "";
    if (name.trim() !== "") panel.setAttribute("aria-label", name.trim());
    openPopup(popup);
    setActive(0);
  }

  function close(focus = false): void {
    releasePopup(popup);
    if (!isOpen()) return;
    overlay.hidden = true;
    active = -1;
    const el = trigger;
    trigger = null;
    binding = null;
    if (el) {
      el.setAttribute("aria-expanded", "false");
      el.removeAttribute("aria-controls");
      // Focus goes back where it was opened from; a menu that closed onto the
      // document body would strand a keyboard reader mid-table.
      if (focus) el.focus();
    }
  }

  function setActive(i: number): void {
    const el = itemEls[i];
    if (!el) return;
    active = i;
    el.focus();
  }

  /** Index of the next item from `from` in direction `dir`, wrapping. */
  function step(from: number, dir: 1 | -1): number {
    const n = itemEls.length;
    if (n === 0) return -1;
    return (from + dir + n) % n;
  }

  // A confirmed action asks over the frame (see confirm-modal.ts): opening the
  // dialog closes the menu (its openPopup peels this popup off the stack), so
  // onSelect fires only on a deliberate confirm. Native confirm() is silently
  // disabled in sandboxed MCP Apps iframes, which is why it is built, not called.
  function choose(i: number): void {
    const b = binding;
    const action = b?.items[i];
    if (!b || !action) return;
    if (action.confirm) {
      const anchor = trigger ?? panel;
      confirmAction(
        anchor,
        { message: action.confirm, confirmLabel: action.label, variant: action.variant },
        () => b.onSelect(i),
      );
      return;
    }
    close(true);
    b.onSelect(i);
  }

  function indexOf(target: EventTarget | null): number {
    if (!(target instanceof Element)) return -1;
    const el = target.closest<HTMLElement>("[data-gomu-action-index]");
    return el ? Number(el.getAttribute("data-gomu-action-index")) : -1;
  }

  panel.addEventListener("click", (ev) => {
    const i = indexOf(ev.target);
    if (i >= 0) choose(i);
  });

  panel.addEventListener("mousemove", (ev) => {
    const i = indexOf(ev.target);
    if (i >= 0 && i !== active) setActive(i);
  });

  panel.addEventListener("keydown", (ev) => {
    switch (ev.key) {
      case "Escape":
        ev.preventDefault();
        close(true);
        return;
      case "ArrowDown":
        ev.preventDefault();
        setActive(step(active, 1));
        return;
      case "ArrowUp":
        ev.preventDefault();
        setActive(step(active, -1));
        return;
      case "Home":
        ev.preventDefault();
        setActive(0);
        return;
      case "End":
        ev.preventDefault();
        setActive(itemEls.length - 1);
        return;
      case "Enter":
      case " ":
      case "Spacebar":
        ev.preventDefault();
        if (active >= 0) choose(active);
        return;
      case "Tab":
        // Tab is a way out, not a way around: it leaves the menu closed and
        // lets focus continue from the trigger.
        close(true);
    }
  });

  // Anything that takes focus out of the panel has ended the interaction —
  // except the trigger, which close() itself focuses.
  panel.addEventListener("focusout", (ev) => {
    const next = (ev as FocusEvent).relatedTarget;
    if (!isOpen() || !(next instanceof Node)) return;
    if (!panel.contains(next) && next !== trigger) close();
  });

  function bind(
    target: HTMLElement,
    attr: string,
    resolve: (el: HTMLElement, value: string) => MenuBinding | null,
  ): void {
    delegate(target, "click", attr, (el, value) => {
      if (isOpen() && trigger === el) {
        close(true);
        return;
      }
      const b = resolve(el, value);
      if (b) open(el, b);
    });

    // Enter and Space on a button already produce a click, so only the arrow
    // keys need answering here — and each lands on the end of the menu it
    // opens, which is where it was aiming.
    delegate(target, "keydown", attr, (el, value, ev) => {
      const key = (ev as KeyboardEvent).key;
      if (key !== "ArrowDown" && key !== "ArrowUp") return;
      ev.preventDefault();
      if (isOpen() && trigger === el) {
        setActive(key === "ArrowDown" ? step(active, 1) : step(active, -1));
        return;
      }
      const b = resolve(el, value);
      if (!b) return;
      open(el, b);
      if (key === "ArrowUp") setActive(itemEls.length - 1);
    });
  }

  return { bind, close, isOpen };
}
