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

const SVG_NS = "http://www.w3.org/2000/svg";

// The two marks a checkbox can show, in a 16x16 viewBox. Mirrored in
// check_render.go, which renders the same node server-side.
const CHECK_TICK = "M4.25 8.5 6.75 11l5-6";
const CHECK_DASH = "M4.5 8h7";

/**
 * A checkbox as a token-styled box with an inline <svg> mark — the same
 * markup check_render.go emits, see ui/css/check.css. attrs go on the input,
 * which keeps every native behaviour; wrapClass goes on the wrapper, which is
 * the box and therefore what layout rules should target.
 *
 * The mark is real markup rather than a background image: a data: URI would
 * depend on the host allowing img-src data:, which the CSP does not promise.
 */
export function checkbox(
  attrs: Record<string, string> = {},
  wrapClass = "",
): { wrap: HTMLElement; input: HTMLInputElement } {
  const input = h("input", { type: "checkbox", ...attrs }) as HTMLInputElement;
  const wrap = h("span", { class: wrapClass ? `gadget-check ${wrapClass}` : "gadget-check" });
  wrap.append(input, checkIcon());
  return { wrap, input };
}

function checkIcon(): SVGElement {
  const svg = document.createElementNS(SVG_NS, "svg");
  for (const [k, v] of Object.entries({
    class: "gadget-check-icon",
    viewBox: "0 0 16 16",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": "2",
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
    "aria-hidden": "true",
  })) {
    svg.setAttribute(k, v);
  }
  const marks: Array<[string, string]> = [
    ["gadget-check-tick", CHECK_TICK],
    ["gadget-check-dash", CHECK_DASH],
  ];
  for (const [cls, d] of marks) {
    const p = document.createElementNS(SVG_NS, "path");
    p.setAttribute("class", cls);
    p.setAttribute("d", d);
    svg.append(p);
  }
  return svg;
}

/**
 * An inline stroked icon in a 16x16 viewBox, one <path> per `d`. Icons are
 * markup rather than URLs because the documents are self-contained: nothing
 * may be fetched, and a data: URI would depend on an img-src allowance the
 * MCP Apps CSP does not promise.
 */
export function icon(cls: string, ...ds: string[]): SVGElement {
  const svg = document.createElementNS(SVG_NS, "svg");
  for (const [k, v] of Object.entries({
    class: cls,
    viewBox: "0 0 16 16",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": "1.75",
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
    "aria-hidden": "true",
  })) {
    svg.setAttribute(k, v);
  }
  for (const d of ds) {
    const p = document.createElementNS(SVG_NS, "path");
    p.setAttribute("d", d);
    svg.append(p);
  }
  return svg;
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
