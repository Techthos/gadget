// Small DOM helpers. Data flows into the DOM exclusively through
// createTextNode/textContent — never innerHTML — so widget data cannot
// inject markup by construction.

type Child = Node | string | null | undefined;

export function h(
  tag: string,
  attrs: Record<string, string | number | boolean | null | undefined> = {},
  ...children: Child[]
): HTMLElement {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === false || v === null || v === undefined) continue;
    el.setAttribute(k, v === true ? "" : String(v));
  }
  for (const c of children) {
    if (c === null || c === undefined) continue;
    el.append(typeof c === "string" ? document.createTextNode(c) : c);
  }
  return el;
}

export function clear(el: Element): void {
  while (el.firstChild) el.removeChild(el.firstChild);
}

/**
 * Event delegation on data-gadget-* attributes: one listener on the widget
 * root dispatches to the nearest ancestor carrying data-gadget-<attr>.
 */
export function delegate(
  root: HTMLElement,
  type: string,
  attr: string,
  handler: (el: HTMLElement, value: string, ev: Event) => void,
): void {
  const selector = `[data-gadget-${attr}]`;
  root.addEventListener(type, (ev) => {
    const target = ev.target;
    if (!(target instanceof Element)) return;
    const el = target.closest(selector);
    if (!(el instanceof HTMLElement) || !root.contains(el)) return;
    handler(el, el.getAttribute(`data-gadget-${attr}`) ?? "", ev);
  });
}
