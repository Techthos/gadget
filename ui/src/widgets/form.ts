// Form widget behavior: native-validation gating, submit as an MCP tool
// call, server-side field errors mapped inline, prefill from tool results.
import type { MountContext } from "../index";
import { CallToolResult, M } from "../protocol";

interface FieldCfg {
	name: string;
	type: string;
	message?: string;
}

interface FormCfg {
	widget: string;
	prefillKey: string;
	errorsKey: string;
	submit: {
		tool: string;
		staticArgs?: Record<string, unknown>;
		successMessage?: string;
	};
	fields: FieldCfg[];
}

type FormControl = HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;

export function mountForm(ctx: MountContext): void {
	const cfg = ctx.config as unknown as FormCfg;
	const { root, bridge } = ctx;

	const formMaybe = root.querySelector<HTMLFormElement>("form[data-gadget-form]");
	if (!formMaybe || !Array.isArray(cfg.fields)) return;
	const form: HTMLFormElement = formMaybe;
	const statusEl = root.querySelector<HTMLElement>("[data-gadget-status]");

	function controlFor(name: string): FormControl | null {
		const el = form.elements.namedItem(name);
		return el instanceof HTMLInputElement ||
			el instanceof HTMLTextAreaElement ||
			el instanceof HTMLSelectElement
			? el
			: null;
	}

	function showStatus(kind: "loading" | "error" | "success" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gadget-status" + (kind ? ` gadget-status--${kind}` : "");
	}

	function setBusy(busy: boolean): void {
		for (const el of form.querySelectorAll<HTMLElement>("input, textarea, select, button")) {
			(el as FormControl | HTMLButtonElement).disabled = busy;
		}
	}

	function clearErrors(): void {
		for (const slot of root.querySelectorAll<HTMLElement>("[data-gadget-error-for]")) {
			slot.hidden = true;
			slot.textContent = "";
		}
		for (const f of cfg.fields) {
			controlFor(f.name)?.removeAttribute("aria-invalid");
		}
	}

	function showFieldError(name: string, message: string): void {
		for (const slot of root.querySelectorAll<HTMLElement>("[data-gadget-error-for]")) {
			if (slot.getAttribute("data-gadget-error-for") === name) {
				slot.hidden = false;
				slot.textContent = message;
			}
		}
		controlFor(name)?.setAttribute("aria-invalid", "true");
	}

	function showErrors(errors: Record<string, unknown>): number {
		let n = 0;
		for (const [name, msg] of Object.entries(errors)) {
			if (typeof msg === "string" && msg !== "") {
				showFieldError(name, msg);
				n++;
			}
		}
		return n;
	}

	function applyValues(values: Record<string, unknown>): void {
		for (const f of cfg.fields) {
			if (!(f.name in values)) continue;
			const control = controlFor(f.name);
			if (!control) continue;
			const v = values[f.name];
			if (f.type === "checkbox" && control instanceof HTMLInputElement) {
				control.checked = v === true;
			} else if (f.type === "multiselect" && control instanceof HTMLSelectElement) {
				const wanted = Array.isArray(v) ? v.map(String) : [];
				for (const opt of control.options) {
					opt.selected = wanted.includes(opt.value);
				}
			} else {
				control.value = v === null || v === undefined ? "" : String(v);
			}
		}
	}

	function collectValues(): Record<string, unknown> {
		const out: Record<string, unknown> = {};
		for (const f of cfg.fields) {
			const control = controlFor(f.name);
			if (!control) continue;
			if (f.type === "checkbox" && control instanceof HTMLInputElement) {
				out[f.name] = control.checked;
			} else if (f.type === "multiselect" && control instanceof HTMLSelectElement) {
				out[f.name] = [...control.selectedOptions].map((o) => o.value);
			} else if (f.type === "number") {
				if (control.value !== "") out[f.name] = Number(control.value);
			} else {
				out[f.name] = control.value;
			}
		}
		return out;
	}

	function validate(): boolean {
		if (form.checkValidity()) return true;
		for (const f of cfg.fields) {
			const control = controlFor(f.name);
			if (control && !control.validity.valid) {
				showFieldError(f.name, f.message ?? control.validationMessage);
			}
		}
		showStatus("error", "Please fix the highlighted fields.");
		return false;
	}

	function applyResult(res: CallToolResult, viaSubmit: boolean): void {
		const sc = res.structuredContent ?? {};
		const values = sc[cfg.prefillKey];
		if (values && typeof values === "object" && !Array.isArray(values)) {
			applyValues(values as Record<string, unknown>);
		}
		const errors = sc[cfg.errorsKey];
		const errCount =
			errors && typeof errors === "object" && !Array.isArray(errors)
				? showErrors(errors as Record<string, unknown>)
				: 0;

		if (errCount > 0) {
			showStatus("error", "Please fix the highlighted fields.");
		} else if (res.isError) {
			showStatus("error", textOf(res) ?? "The request failed.");
		} else if (viaSubmit) {
			showStatus("success", cfg.submit.successMessage ?? textOf(res) ?? "Saved.");
		} else {
			showStatus("", "");
		}
	}

	function textOf(res: CallToolResult): string | undefined {
		for (const block of res.content ?? []) {
			if (block.type === "text" && typeof block.text === "string") return block.text;
		}
		return undefined;
	}

	form.addEventListener("submit", (ev) => {
		ev.preventDefault();
		clearErrors();
		if (!validate()) return;
		const args = { ...(cfg.submit.staticArgs ?? {}), ...collectValues() };
		showStatus("loading", "Submitting…");
		setBusy(true);
		bridge.callTool(cfg.submit.tool, args).then(
			(res) => {
				setBusy(false);
				applyResult(res, true);
			},
			(e: unknown) => {
				setBusy(false);
				showStatus("error", e instanceof Error ? e.message : String(e));
			},
		);
	});

	root.querySelector<HTMLElement>("[data-gadget-cancel]")?.addEventListener("click", () => {
		form.reset();
		clearErrors();
		showStatus("", "");
	});

	// Host-pushed results (e.g. the model invoked the edit tool: prefill).
	bridge.on(M.toolInput, () => showStatus("loading", "Loading…"));
	bridge.on(M.toolResult, (params) => applyResult((params ?? {}) as CallToolResult, false));
	bridge.on(M.toolCancelled, () => showStatus("", ""));

	// Baked snapshot: prefill and errors, if present.
	if (ctx.initialData) {
		applyResult({ structuredContent: ctx.initialData }, false);
	}
}
