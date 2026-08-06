// Action confirmation as a modal over the frame: the question and a
// cancel/confirm pair, centered and dimmed like the dropdown and action menu
// (see popup.ts). It replaces the older two-phase button — arm on the first
// press, fire on a second, with the label swapped to the question in place —
// which was cramped and easy to miss. Native confirm() is silently disabled in
// sandboxed MCP Apps iframes, so the dialog is built from the same DOM as
// everything else.
//
// Data reaches the DOM through textContent only (see dom.ts); the question and
// the button labels are no exception.
import { h } from "./dom";
import { mountOverlay, openPopup, popupHost, releasePopup, type Popup } from "./popup";

export interface ConfirmRequest {
  /** The question, e.g. "Really delete?" — what the two-phase button used to
   * swap its label to. */
  message: string;
  /** The confirm button's label — the action's own name. */
  confirmLabel: string;
  /** Styles the confirm button to match the action (e.g. "danger"). */
  variant?: string;
}

let seq = 0;

/**
 * Opens a confirmation over the frame. onConfirm fires only if the reader
 * confirms; a cancel, an outside press or Escape dismisses it without firing.
 * Focus returns to anchor — the control the action was invoked from — either
 * way, unless a re-render has since removed it.
 */
export function confirmAction(
  anchor: HTMLElement,
  req: ConfirmRequest,
  onConfirm: () => void,
): void {
  const host = popupHost(anchor);
  const id = `gomu-ask-${++seq}`;

  const cancelBtn = h(
    "button",
    { type: "button", class: "gomu-btn gomu-ask-cancel" },
    "Cancel",
  ) as HTMLButtonElement;
  const confirmBtn = h(
    "button",
    {
      type: "button",
      class: "gomu-btn gomu-ask-confirm" + (req.variant ? ` gomu-btn--${req.variant}` : ""),
    },
    req.confirmLabel,
  ) as HTMLButtonElement;

  const panel = h(
    "div",
    {
      class: "gomu-pop-panel gomu-ask-panel",
      role: "alertdialog",
      "aria-modal": "true",
      "aria-labelledby": id,
    },
    h("p", { class: "gomu-ask-message", id }, req.message),
    h("div", { class: "gomu-ask-actions" }, cancelBtn, confirmBtn),
  );

  const overlay = mountOverlay(host, panel);
  const popup: Popup = { anchor: panel, panel, close };

  function close(focus = false): void {
    releasePopup(popup);
    if (overlay.hidden) return;
    overlay.hidden = true;
    overlay.remove();
    if (focus && anchor.isConnected) anchor.focus();
  }

  overlay.hidden = false;
  openPopup(popup);

  cancelBtn.addEventListener("click", () => close(true));
  confirmBtn.addEventListener("click", () => {
    close(true);
    onConfirm();
  });
  panel.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape") {
      ev.preventDefault();
      close(true);
    }
  });

  // A destructive action lands focus on Cancel, so an accidental Enter dismisses
  // rather than fires; anything else lands on the confirm.
  (req.variant === "danger" ? cancelBtn : confirmBtn).focus();
}
