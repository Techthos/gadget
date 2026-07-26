// Confirm widget behavior: one decision, two outcomes. Go rendered the
// question, the guards and the buttons; this fills in what depends on the
// particular call — the record's detail values and the list of side effects —
// enforces the guards, and drives the accept/reject tool calls.
//
// The decision is terminal: once it is made the buttons go away and the
// outcome stays on screen. The single exception is a failed accept, which
// re-arms the widget so a transient error can be retried.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { clear, delegate, h } from "../dom";
import { CallToolResult, M } from "../protocol";
import { ActionCfg, resolveArgs, textOf } from "./card-common";
import { DescriptionItemCfg, fillDescriptions } from "./descriptions";

interface EffectCfg {
	text: string;
	detail?: string;
	value?: string;
	severity?: string;
}

interface ConfirmCfg {
	widget: string;
	rowsKey: string;
	effectsKey: string;
	rowId: string;
	accept: { tool: string; args?: ActionCfg["args"]; successMessage?: string };
	reject?: { tool?: string; args?: ActionCfg["args"]; message: string };
	details?: DescriptionItemCfg[];
	effects?: EffectCfg[];
	typeToConfirm?: string;
	acknowledge?: boolean;
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

// Effect severities that may become a class name. The list arrives as tool
// data, not as author config, so it is matched against this set rather than
// interpolated — the same reason data never reaches the DOM as markup.
const SEVERITIES = new Set(["neutral", "info", "success", "warning", "danger"]);

export function mountConfirm(ctx: MountContext): void {
	const cfg = ctx.config as unknown as ConfirmCfg;
	const { root, bridge } = ctx;

	const detailsEl = root.querySelector<HTMLElement>("[data-gadget-descriptions]");
	const effectsEl = root.querySelector<HTMLElement>("[data-gadget-effects]");
	const decisionEl = root.querySelector<HTMLElement>("[data-gadget-decision]");
	const outcomeEl = root.querySelector<HTMLElement>("[data-gadget-outcome]");
	const statusEl = root.querySelector<HTMLElement>("[data-gadget-status]");
	const acceptEl = root.querySelector<HTMLButtonElement>("[data-gadget-accept]");
	const rejectEl = root.querySelector<HTMLButtonElement>("[data-gadget-reject]");
	const ackEl = root.querySelector<HTMLInputElement>("[data-gadget-ack]");
	const phraseEl = root.querySelector<HTMLInputElement>("[data-gadget-phrase]");

	const items = Array.isArray(cfg.details) ? cfg.details : [];
	let row: Row | null = rowsFrom(ctx.initialData, cfg.rowsKey)[0] ?? null;
	let effects: EffectCfg[] = effectsFrom(ctx.initialData, cfg.effectsKey) ?? cfg.effects ?? [];
	let phase: "deciding" | "working" | "settled" = "deciding";

	function showStatus(kind: "loading" | "error" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gadget-status" + (kind ? ` gadget-status--${kind}` : "");
	}

	// Every guard the author configured must be satisfied, and a decision
	// already in flight or already made closes the door for good.
	function guardsOk(): boolean {
		if (cfg.acknowledge && ackEl && !ackEl.checked) return false;
		if (cfg.typeToConfirm && (phraseEl?.value ?? "").trim() !== cfg.typeToConfirm) return false;
		return true;
	}

	function syncButtons(): void {
		const locked = phase !== "deciding";
		if (acceptEl) acceptEl.disabled = locked || !guardsOk();
		if (rejectEl) rejectEl.disabled = locked;
		if (ackEl) ackEl.disabled = locked;
		if (phraseEl) phraseEl.disabled = locked;
	}

	function renderDetails(): void {
		if (detailsEl) fillDescriptions(detailsEl, items, row);
	}

	function renderEffects(): void {
		if (!effectsEl) return;
		clear(effectsEl);
		effectsEl.hidden = effects.length === 0;
		for (const effect of effects) {
			const severity = effect.severity && SEVERITIES.has(effect.severity) ? effect.severity : "";
			const li = h("li", {
				class: "gadget-effect" + (severity ? ` gadget-effect--${severity}` : ""),
			});
			li.append(h("span", { class: "gadget-effect-mark", "aria-hidden": "true" }));

			const text = h("span", { class: "gadget-effect-text" });
			text.append(h("span", { class: "gadget-effect-label" }, effect.text));
			if (effect.detail) {
				text.append(h("span", { class: "gadget-effect-detail" }, effect.detail));
			}
			li.append(text);

			if (effect.value) {
				li.append(h("span", { class: "gadget-effect-value" }, effect.value));
			}
			effectsEl.append(li);
		}
	}

	function render(): void {
		renderDetails();
		renderEffects();
		syncButtons();
	}

	// The end of the widget's life: the buttons go, the outcome stays.
	function settle(message: string, kind: "accepted" | "declined"): void {
		phase = "settled";
		syncButtons();
		if (decisionEl) decisionEl.hidden = true;
		if (outcomeEl) {
			outcomeEl.hidden = false;
			outcomeEl.textContent = message;
			outcomeEl.className = `gadget-confirm-outcome gadget-confirm-outcome--${kind}`;
		}
		showStatus("", "");
	}

	function fail(message: string): void {
		phase = "deciding";
		syncButtons();
		showStatus("error", message);
	}

	// Runs one side of the decision. Returns the result, or null when the
	// call failed and the widget has already re-armed for a retry.
	async function call(
		tool: string,
		args: ActionCfg["args"],
		fallback: string,
	): Promise<CallToolResult | null> {
		phase = "working";
		syncButtons();
		showStatus("loading", "Working…");
		try {
			const action: ActionCfg = { label: "", kind: "tool", tool, args };
			const res = await bridge.callTool(tool, resolveArgs(action, row, []));
			if (res.isError) {
				fail(textOf(res) ?? fallback);
				return null;
			}
			return res;
		} catch (e) {
			fail(e instanceof Error ? e.message : String(e));
			return null;
		}
	}

	async function accept(): Promise<void> {
		if (phase !== "deciding" || !guardsOk() || !cfg.accept?.tool) return;
		const res = await call(cfg.accept.tool, cfg.accept.args, "The action failed.");
		if (!res) return;
		applyData(res);
		settle(cfg.accept.successMessage || textOf(res) || "Done.", "accepted");
	}

	async function reject(): Promise<void> {
		if (phase !== "deciding" || !cfg.reject) return;
		if (cfg.reject.tool) {
			const res = await call(cfg.reject.tool, cfg.reject.args, "Could not cancel.");
			if (!res) return;
		}
		settle(cfg.reject.message || "Cancelled.", "declined");
	}

	// Refreshes whichever parts of the widget a payload carries. Never
	// changes the phase: a host may push results long after the decision.
	function applyData(data: CallToolResult | { structuredContent?: unknown }): void {
		const sc = (data as CallToolResult).structuredContent;
		if (!sc || typeof sc !== "object") return;
		const content = sc as Record<string, unknown>;
		if (cfg.rowsKey in content) {
			row = rowsFrom(content, cfg.rowsKey)[0] ?? null;
		}
		const pushed = effectsFrom(content, cfg.effectsKey);
		if (pushed) effects = pushed;
		render();
	}

	acceptEl?.addEventListener("click", () => void accept());
	rejectEl?.addEventListener("click", () => void reject());
	ackEl?.addEventListener("change", syncButtons);
	phraseEl?.addEventListener("input", syncButtons);
	// Typing the phrase and pressing Enter is the same decision as clicking.
	phraseEl?.addEventListener("keydown", (ev) => {
		if (ev.key === "Enter") {
			ev.preventDefault();
			void accept();
		}
	});

	// Link values in the detail list go to the host: navigation is blocked
	// inside the sandboxed iframe.
	delegate(root, "click", "link", (_el, href) => {
		if (href !== "") void bridge.openLink(href);
	});

	bridge.on(M.toolInput, () => {
		if (phase === "deciding") showStatus("loading", "Loading…");
	});
	bridge.on(M.toolResult, (params) => {
		applyData((params ?? {}) as CallToolResult);
		if (phase === "deciding") showStatus("", "");
	});
	bridge.on(M.toolCancelled, () => {
		if (phase === "working") phase = "deciding";
		syncButtons();
		showStatus("", "");
	});
	document.addEventListener(HOST_CONTEXT_EVENT, renderDetails);

	render();

	// Load-time hydration: with a host connected, decide on current facts
	// rather than on the snapshot frozen when the document was registered.
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (!ok) return;
			showStatus("loading", "Loading…");
			bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {}).then(
				(res) => {
					applyData(res);
					if (phase === "deciding") showStatus("", "");
				},
				() => {
					if (phase === "deciding") showStatus("", "");
				},
			);
		});
	}
}

/** Reads the effects array from a structuredContent-shaped object, or null
 * when the payload does not mention them at all. */
function effectsFrom(
	data: Record<string, unknown> | null | undefined,
	key: string,
): EffectCfg[] | null {
	const v = data?.[key];
	if (!Array.isArray(v)) return null;
	return v
		.filter((e): e is Record<string, unknown> => e !== null && typeof e === "object")
		.map((e) => ({
			text: String(e.text ?? ""),
			detail: e.detail === undefined ? undefined : String(e.detail),
			value: e.value === undefined ? undefined : String(e.value),
			severity: e.severity === undefined ? undefined : String(e.severity),
		}))
		.filter((e) => e.text !== "");
}
