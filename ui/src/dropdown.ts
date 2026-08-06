// Custom dropdown: upgrades a native <select> into a trigger button plus a
// modal listbox, so every widget gets the same control instead of the host
// browser's own select chrome (which is unstyleable and looks foreign inside
// a chat pane), and the list opens as a centered overlay rather than a panel
// with nowhere to drop in a short iframe (see popup.ts).
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
import { mountOverlay, openPopup, popupHost, releasePopup, type Popup } from "./popup";

export const CHEVRON_PATH = "M4 6.5 8 10.5 12 6.5";
const CHECK_PATH = "M3.5 8.5 6.5 11.5 12.5 5";
const CLOSE_PATHS = ["M4 4 12 12", "M12 4 4 12"];
// A typed run this far apart starts a new typeahead search.
const TYPEAHEAD_RESET_MS = 600;
// Above this many options the panel gains a search field; below it, the list
// is short enough to scan and typeahead covers it.
const SEARCH_MIN = 8;

interface Dropdown extends Popup {
  overlay: HTMLElement;
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

/**
 * Undoes the upgrade of a select that is about to be thrown away. The overlay
 * is a child of the widget root rather than of the select (see popup.ts), so
 * dropping the select alone would leave the overlay behind — a list rebuilt on
 * every data change would pile them up.
 */
export function releaseDropdown(select: HTMLSelectElement | null | undefined): void {
  if (!select) return;
  const dd = registry.get(select);
  if (!dd) return;
  dd.close();
  dd.overlay.remove();
  registry.delete(select);
}

/** releaseDropdown for every select under root. */
export function releaseDropdowns(root: ParentNode): void {
  for (const select of root.querySelectorAll<HTMLSelectElement>("select")) {
    releaseDropdown(select);
  }
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

  const id = `gomu-dd-${++seq}`;
  const multiple = select.multiple;

  const wrap = h("div", { class: "gomu-dd", "data-gomu-dd": "" });
  parent.insertBefore(wrap, select);
  wrap.append(select);
  select.classList.add("gomu-dd-native");
  select.tabIndex = -1;

  // Author classes (e.g. gomu-sort-select) style what the user sees, which
  // is now the trigger; the select itself is only a value holder.
  const extra = [...select.classList].filter(
    (c) => c !== "gomu-input" && c !== "gomu-dd-native",
  );
  const trigger = h("button", {
    type: "button",
    class: ["gomu-input", "gomu-dd-trigger", ...extra].join(" "),
    // Select-only combobox: the trigger points at the active option with
    // aria-activedescendant while the list is closed; when the panel carries a
    // search field, the field takes that role once open.
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

  const valueEl = h("span", { class: "gomu-dd-value" });
  trigger.append(valueEl, icon("gomu-dd-chevron", CHEVRON_PATH));
  wrap.append(trigger);

  const host = popupHost(select);
  const titleText = ariaLabel ?? labelText(host, trigger.id) ?? "Select";

  const closeBtn = h(
    "button",
    { type: "button", class: "gomu-pop-close", "aria-label": "Close" },
    icon("gomu-pop-close-icon", ...CLOSE_PATHS),
  ) as HTMLButtonElement;
  const header = h("div", { class: "gomu-pop-header" }, h("span", { class: "gomu-pop-title" }, titleText), closeBtn);

  const searchInput =
    select.options.length > SEARCH_MIN
      ? (h("input", {
          type: "text",
          class: "gomu-input gomu-dd-search",
          role: "combobox",
          "aria-expanded": "true",
          "aria-controls": id,
          "aria-autocomplete": "list",
          "aria-label": ariaLabel !== null ? `Search ${ariaLabel}` : "Search options",
          placeholder: "Search…",
        }) as HTMLInputElement)
      : null;
  // Whichever element holds focus while the list is open owns the active
  // descendant: the search field when there is one, else the trigger.
  const owner: HTMLElement = searchInput ?? trigger;

  const list = h("div", { class: "gomu-dd-list", id, role: "listbox" });
  if (multiple) list.setAttribute("aria-multiselectable", "true");

  const panel = h("div", { class: "gomu-pop-panel gomu-dd-panel" });
  panel.append(header);
  if (searchInput) panel.append(searchInput);
  panel.append(list);

  const overlay = mountOverlay(host, panel);

  let optionEls: HTMLElement[] = [];
  let active = -1;
  let typed = "";
  let typedAt = 0;

  function buildOptions(): void {
    clear(list);
    optionEls = [...select.options].map((opt, i) => {
      const el = h("div", {
        class: "gomu-dd-option",
        role: "option",
        id: `${id}-o${i}`,
        "data-gomu-dd-index": String(i),
        "aria-disabled": opt.disabled ? "true" : null,
      });
      el.append(
        icon("gomu-dd-check", CHECK_PATH),
        h("span", { class: "gomu-dd-option-label" }, opt.text),
      );
      list.append(el);
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
    valueEl.classList.toggle("gomu-dd-value--placeholder", isPlaceholder);

    // Widget behaviors disable and flag the select (busy forms, field errors);
    // the trigger is what the user sees, so it carries both.
    trigger.disabled = select.disabled;
    const invalid = select.getAttribute("aria-invalid");
    if (invalid === null) trigger.removeAttribute("aria-invalid");
    else trigger.setAttribute("aria-invalid", invalid);
    if (select.disabled) close();
  }

  function isOpen(): boolean {
    return !overlay.hidden;
  }

  function open(): void {
    if (isOpen() || select.disabled) return;
    overlay.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
    openPopup(dd);
    if (searchInput) {
      searchInput.value = "";
      filter("");
    }
    const start = select.selectedIndex;
    setActive(start >= 0 && selectableAt(start) ? start : step(-1, 1));
    owner.focus();
  }

  function close(focus = false): void {
    releasePopup(dd);
    if (!isOpen()) return;
    overlay.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
    setActive(-1);
    if (focus) trigger.focus();
  }

  function setActive(i: number): void {
    optionEls[active]?.classList.remove("gomu-dd-option--active");
    active = i;
    const el = optionEls[i];
    if (!el) {
      owner.removeAttribute("aria-activedescendant");
      return;
    }
    el.classList.add("gomu-dd-option--active");
    owner.setAttribute("aria-activedescendant", el.id);
    // Absent in some non-browser DOM implementations.
    if (typeof el.scrollIntoView === "function") el.scrollIntoView({ block: "nearest" });
  }

  /** A pickable option: neither disabled nor filtered out by the search. */
  function selectableAt(i: number): boolean {
    const opt = select.options[i];
    return !!opt && !opt.disabled && !optionEls[i]?.hidden;
  }

  /** Index of the next selectable option from `from` in direction `dir`. */
  function step(from: number, dir: 1 | -1): number {
    const n = select.options.length;
    for (let i = from + dir; i >= 0 && i < n; i += dir) {
      if (selectableAt(i)) return i;
    }
    return selectableAt(from) ? from : -1;
  }

  /** Hides options whose label does not contain the query, and keeps the
   * active option on something visible. */
  function filter(query: string): void {
    const needle = query.trim().toLowerCase();
    optionEls.forEach((el, i) => {
      const opt = select.options[i];
      el.hidden = needle !== "" && !(opt?.text.toLowerCase().includes(needle) ?? false);
    });
    if (active < 0 || optionEls[active]?.hidden) setActive(step(-1, 1));
    else setActive(active);
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
      if (opt && selectableAt(i) && opt.text.toLowerCase().startsWith(typed)) {
        setActive(i);
        return;
      }
    }
  }

  /** Keyboard for the control, shared by the trigger and the search field —
   * whichever holds focus. */
  function onKey(ev: KeyboardEvent): void {
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
      if (!isOpen() || searchInput) return;
      ev.preventDefault();
      setActive(key === "Home" ? step(-1, 1) : step(select.options.length, -1));
      return;
    }
    if (key === "Enter") {
      ev.preventDefault();
      if (!isOpen()) open();
      else if (active >= 0) choose(active);
      return;
    }
    if (key === " " || key === "Spacebar") {
      // With a search field, space types into it; without one it selects.
      if (searchInput) return;
      ev.preventDefault();
      if (!isOpen()) open();
      else if (active >= 0) choose(active);
      return;
    }
    if (key.length === 1 && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
      if (searchInput) {
        // The field filters; only open the list so the first keystroke lands.
        if (!isOpen()) {
          open();
          ev.preventDefault();
        }
        return;
      }
      if (!isOpen()) open();
      typeahead(key.toLowerCase());
      ev.preventDefault();
    }
  }

  trigger.addEventListener("click", () => {
    if (isOpen()) close(true);
    else open();
  });
  trigger.addEventListener("keydown", onKey);
  searchInput?.addEventListener("keydown", onKey);
  searchInput?.addEventListener("input", () => filter(searchInput.value));

  closeBtn.addEventListener("click", () => close(true));

  // A press on the option list keeps focus on the owner (trigger or search
  // field) so keyboard control survives a click; the header controls stay
  // clickable because they sit outside the list.
  list.addEventListener("pointerdown", (ev) => ev.preventDefault());

  list.addEventListener("click", (ev) => {
    const target = ev.target;
    if (!(target instanceof Element)) return;
    const el = target.closest<HTMLElement>("[data-gomu-dd-index]");
    if (el) choose(Number(el.getAttribute("data-gomu-dd-index")));
  });

  list.addEventListener("mousemove", (ev) => {
    const target = ev.target;
    if (!(target instanceof Element)) return;
    const el = target.closest<HTMLElement>("[data-gomu-dd-index]");
    if (el) setActive(Number(el.getAttribute("data-gomu-dd-index")));
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

  const dd: Dropdown = { anchor: wrap, panel, overlay, sync, close };
  registry.set(select, dd);
  buildOptions();
  sync();
}

/** The text of the <label> that points at id, if any — the field name to
 * title the overlay with when the select carries no aria-label. */
function labelText(host: ParentNode, id: string): string | null {
  if (!id) return null;
  const label = host.querySelector(`label[for="${id}"]`);
  const text = label?.textContent?.trim();
  return text ? text : null;
}
