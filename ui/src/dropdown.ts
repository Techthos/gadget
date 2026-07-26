// Custom dropdown: upgrades a native <select> into a trigger button plus a
// popup listbox, so every widget gets the same control instead of the host
// browser's own select chrome (which is unstyleable and looks foreign inside
// a chat pane).
//
// The <select> stays in the DOM as the single source of truth: it keeps the
// value, the name, native constraint validation and multiple-selection, and
// widget behaviors go on reading and writing it exactly as before. This layer
// only mirrors it — writes go back through the select and re-emit its change
// event, so nothing downstream can tell the difference.
//
// Data reaches the DOM through textContent only (see dom.ts); option labels
// are no exception.
import { clear, h, icon } from "./dom";
import { openPopup, popupHost, positionPanel, releasePopup, type Popup } from "./popup";

export const CHEVRON_PATH = "M4 6.5 8 10.5 12 6.5";
const CHECK_PATH = "M3.5 8.5 6.5 11.5 12.5 5";
// A typed run this far apart starts a new typeahead search.
const TYPEAHEAD_RESET_MS = 600;

interface Dropdown extends Popup {
  sync(): void;
}

const registry = new WeakMap<HTMLSelectElement, Dropdown>();
let seq = 0;

/** Upgrades every <select> under root that is not already a dropdown. */
export function enhanceSelects(root: ParentNode): void {
  for (const select of root.querySelectorAll<HTMLSelectElement>("select")) {
    enhanceSelect(select);
  }
}

/**
 * Re-reads a select whose value was changed in code (a programmatic write
 * fires no event, so the dropdown cannot notice it on its own). A no-op for a
 * select that was never enhanced, so behaviors can call it unconditionally.
 */
export function refreshDropdown(select: HTMLSelectElement | null | undefined): void {
  if (select) registry.get(select)?.sync();
}

/** refreshDropdown for every select under root. */
export function refreshDropdowns(root: ParentNode): void {
  for (const select of root.querySelectorAll<HTMLSelectElement>("select")) {
    refreshDropdown(select);
  }
}

export function enhanceSelect(select: HTMLSelectElement): void {
  const parent = select.parentNode;
  if (!parent || registry.has(select)) return;

  const id = `gadget-dd-${++seq}`;
  const multiple = select.multiple;

  const wrap = h("div", { class: "gadget-dd", "data-gadget-dd": "" });
  parent.insertBefore(wrap, select);
  wrap.append(select);
  select.classList.add("gadget-dd-native");
  select.tabIndex = -1;

  // Author classes (e.g. gadget-sort-select) style what the user sees, which
  // is now the trigger; the select itself is only a value holder.
  const extra = [...select.classList].filter(
    (c) => c !== "gadget-input" && c !== "gadget-dd-native",
  );
  const trigger = h("button", {
    type: "button",
    class: ["gadget-input", "gadget-dd-trigger", ...extra].join(" "),
    // Select-only combobox: the trigger keeps focus while open and points at
    // the active option with aria-activedescendant, so the panel itself never
    // has to be focusable.
    role: "combobox",
    "aria-haspopup": "listbox",
    "aria-expanded": "false",
    "aria-controls": id,
  }) as HTMLButtonElement;
  // A field label addresses its control by id; the trigger is what clicking
  // that label should reach.
  if (select.id) {
    trigger.id = select.id;
    select.removeAttribute("id");
  }
  const ariaLabel = select.getAttribute("aria-label");
  if (ariaLabel !== null) trigger.setAttribute("aria-label", ariaLabel);

  const valueEl = h("span", { class: "gadget-dd-value" });
  trigger.append(valueEl, icon("gadget-dd-chevron", CHEVRON_PATH));
  wrap.append(trigger);

  const panel = h("div", {
    class: "gadget-pop-panel gadget-dd-panel",
    id,
    role: "listbox",
    hidden: true,
  });
  if (multiple) panel.setAttribute("aria-multiselectable", "true");
  // The panel hangs off the widget root rather than the field: the card chrome
  // clips its overflow, and a panel nested inside it would be cut off at the
  // card's edge (see popup.ts).
  const host = popupHost(select);
  host.append(panel);

  let optionEls: HTMLElement[] = [];
  let active = -1;
  let typed = "";
  let typedAt = 0;

  function buildOptions(): void {
    clear(panel);
    optionEls = [...select.options].map((opt, i) => {
      const el = h("div", {
        class: "gadget-dd-option",
        role: "option",
        id: `${id}-o${i}`,
        "data-gadget-dd-index": String(i),
        "aria-disabled": opt.disabled ? "true" : null,
      });
      el.append(
        icon("gadget-dd-check", CHECK_PATH),
        h("span", { class: "gadget-dd-option-label" }, opt.text),
      );
      panel.append(el);
      return el;
    });
  }

  function placeholder(): string {
    return select.getAttribute("placeholder") ?? "Select…";
  }

  function sync(): void {
    optionEls.forEach((el, i) => {
      el.setAttribute("aria-selected", select.options[i]?.selected ? "true" : "false");
    });

    // Filtered by hand rather than read from selectedOptions: that collection
    // goes stale after a selectedIndex write in some DOM implementations.
    const chosen = [...select.options].filter((o) => o.selected);
    let text: string;
    let isPlaceholder: boolean;
    if (multiple) {
      isPlaceholder = chosen.length === 0;
      text = isPlaceholder
        ? placeholder()
        : chosen.length > 2
          ? `${chosen.length} selected`
          : chosen.map((o) => o.text).join(", ");
    } else {
      const one = chosen[0];
      isPlaceholder = !one || one.value === "";
      text = one ? one.text : placeholder();
    }
    valueEl.textContent = text;
    valueEl.classList.toggle("gadget-dd-value--placeholder", isPlaceholder);

    // Widget behaviors disable and flag the select (busy forms, field errors);
    // the trigger is what the user sees, so it carries both.
    trigger.disabled = select.disabled;
    const invalid = select.getAttribute("aria-invalid");
    if (invalid === null) trigger.removeAttribute("aria-invalid");
    else trigger.setAttribute("aria-invalid", invalid);
    if (select.disabled) close();
  }

  function position(): void {
    positionPanel(trigger, panel, host, { matchWidth: true });
  }

  function isOpen(): boolean {
    return !panel.hidden;
  }

  function open(): void {
    if (isOpen() || select.disabled) return;
    panel.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
    openPopup(dd);
    position();
    setActive(select.selectedIndex >= 0 ? select.selectedIndex : step(-1, 1));
  }

  function close(focus = false): void {
    releasePopup(dd);
    if (!isOpen()) return;
    panel.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
    trigger.removeAttribute("aria-activedescendant");
    setActive(-1);
    if (focus) trigger.focus();
  }

  function setActive(i: number): void {
    optionEls[active]?.classList.remove("gadget-dd-option--active");
    active = i;
    const el = optionEls[i];
    if (!el) {
      trigger.removeAttribute("aria-activedescendant");
      return;
    }
    el.classList.add("gadget-dd-option--active");
    trigger.setAttribute("aria-activedescendant", el.id);
    // Absent in some non-browser DOM implementations.
    if (typeof el.scrollIntoView === "function") el.scrollIntoView({ block: "nearest" });
  }

  /** Index of the next selectable option from `from` in direction `dir`. */
  function step(from: number, dir: 1 | -1): number {
    const n = select.options.length;
    for (let i = from + dir; i >= 0 && i < n; i += dir) {
      if (!select.options[i]?.disabled) return i;
    }
    return from >= 0 && from < n && !select.options[from]?.disabled ? from : -1;
  }

  function choose(i: number): void {
    const opt = select.options[i];
    if (!opt || opt.disabled) return;
    if (multiple) opt.selected = !opt.selected;
    else select.selectedIndex = i;
    sync();
    select.dispatchEvent(new Event("input", { bubbles: true }));
    select.dispatchEvent(new Event("change", { bubbles: true }));
    // A single choice is the whole interaction; a multiple one is not.
    if (multiple) setActive(i);
    else close(true);
  }

  function typeahead(ch: string): void {
    const now = Date.now();
    typed = now - typedAt > TYPEAHEAD_RESET_MS ? ch : typed + ch;
    typedAt = now;
    const from = active >= 0 ? active : 0;
    const n = select.options.length;
    for (let k = 0; k < n; k++) {
      // Start one past the active option so repeating a letter cycles.
      const i = (from + (typed.length > 1 ? 0 : 1) + k) % n;
      const opt = select.options[i];
      if (opt && !opt.disabled && opt.text.toLowerCase().startsWith(typed)) {
        setActive(i);
        return;
      }
    }
  }

  trigger.addEventListener("click", () => {
    if (isOpen()) close(true);
    else open();
  });

  trigger.addEventListener("keydown", (ev) => {
    const key = ev.key;
    if (key === "Escape") {
      if (isOpen()) {
        ev.preventDefault();
        close(true);
      }
      return;
    }
    if (key === "Tab") {
      close();
      return;
    }
    if (key === "ArrowDown" || key === "ArrowUp") {
      ev.preventDefault();
      if (!isOpen()) open();
      else setActive(step(active, key === "ArrowDown" ? 1 : -1));
      return;
    }
    if (key === "Home" || key === "End") {
      if (!isOpen()) return;
      ev.preventDefault();
      setActive(key === "Home" ? step(-1, 1) : step(select.options.length, -1));
      return;
    }
    if (key === "Enter" || key === " " || key === "Spacebar") {
      ev.preventDefault();
      if (!isOpen()) open();
      else if (active >= 0) choose(active);
      return;
    }
    if (key.length === 1 && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
      if (!isOpen()) open();
      typeahead(key.toLowerCase());
      ev.preventDefault();
    }
  });

  // The panel is not focusable: pressing inside it must not pull focus off
  // the trigger, or the dropdown would close before the click lands.
  panel.addEventListener("pointerdown", (ev) => ev.preventDefault());

  panel.addEventListener("click", (ev) => {
    const target = ev.target;
    if (!(target instanceof Element)) return;
    const el = target.closest<HTMLElement>("[data-gadget-dd-index]");
    if (el) choose(Number(el.getAttribute("data-gadget-dd-index")));
  });

  panel.addEventListener("mousemove", (ev) => {
    const target = ev.target;
    if (!(target instanceof Element)) return;
    const el = target.closest<HTMLElement>("[data-gadget-dd-index]");
    if (el) setActive(Number(el.getAttribute("data-gadget-dd-index")));
  });

  // Anything that focuses the control itself (a label click, a behavior
  // calling focus()) lands on the hidden select; hand it to the trigger.
  select.addEventListener("focus", () => trigger.focus());

  // Behaviors set disabled and aria-invalid on the select directly, and both
  // reflect to attributes — so watching them keeps the trigger in step without
  // every call site having to remember this layer exists.
  if (typeof MutationObserver !== "undefined") {
    new MutationObserver(() => sync()).observe(select, {
      attributes: true,
      attributeFilter: ["disabled", "aria-invalid"],
    });
  }

  const dd: Dropdown = { anchor: wrap, panel, sync, close, position };
  registry.set(select, dd);
  buildOptions();
  sync();
}

