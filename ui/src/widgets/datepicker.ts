// DatePicker widget behavior: an inline calendar, a line stating what is
// picked, then submit. Go rendered the question and the buttons; the grid is
// runtime work (see calendar.ts) and so is everything the host can change
// about it — the selection, the bounds, and which days are already taken all
// arrive in structuredContent.
//
// The decision is terminal: once it is made the controls go away and the
// outcome stays. The single exception is a failed submit, which re-arms the
// widget so a transient error can be retried.
import type { MountContext } from "../index";
import {
	createCalendar,
	formatDate,
	formatDateRange,
	parseISO,
	type CalendarCfg,
	type CalendarView,
	type DateValue,
} from "../calendar";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { delegate } from "../dom";
import { CallToolResult, M } from "../protocol";
import { errorText, textOf } from "../status";
import { ActionCfg, resolveArgs } from "./card-common";
import {
	clearInputErrors,
	collectInputs,
	DescriptionItemCfg,
	fillDescriptions,
	hasInputs,
	type InputValues,
	validateInputs,
	watchInputs,
} from "./descriptions";

interface DatePickerCfg {
	widget: string;
	valueKey: string;
	rowsKey: string;
	rowId: string;
	calendar: CalendarCfg;
	default?: string;
	defaultEnd?: string;
	details?: DescriptionItemCfg[];
	submit: {
		tool: string;
		valueArg: string;
		endArg?: string;
		args?: ActionCfg["args"];
		/** Posts this plus the picked date as a user turn. See DateSubmit.ChatPrompt. */
		chatPrompt?: string;
		successMessage?: string;
	};
	cancel?: { tool?: string; args?: ActionCfg["args"]; message: string };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

export function mountDatePicker(ctx: MountContext): void {
	const cfg = ctx.config as unknown as DatePickerCfg;
	const { root, bridge } = ctx;

	const calEl = root.querySelector<HTMLElement>("[data-gomu-calendar]");
	const detailsEl = root.querySelector<HTMLElement>("[data-gomu-descriptions]");
	const summaryEl = root.querySelector<HTMLElement>("[data-gomu-summary]");
	const decisionEl = root.querySelector<HTMLElement>("[data-gomu-decision]");
	const outcomeEl = root.querySelector<HTMLElement>("[data-gomu-outcome]");
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");
	const submitEl = root.querySelector<HTMLButtonElement>("[data-gomu-submit]");
	const cancelEl = root.querySelector<HTMLButtonElement>("[data-gomu-cancel]");
	if (!calEl) return;

	const items = Array.isArray(cfg.details) ? cfg.details : [];
	const range = cfg.calendar?.mode === "range";
	let row: Row | null = rowsFrom(ctx.initialData, cfg.rowsKey)[0] ?? null;
	let phase: "deciding" | "working" | "settled" = "deciding";
	// What the reader has put into the detail controls. Held here rather than
	// read back off the DOM alone, because a tool result re-renders the list
	// and must not wipe a half-typed answer.
	const inputs: InputValues = {};

	const cal: CalendarView = createCalendar(cfg.calendar ?? {}, {
		host: calEl,
		onChange: () => {
			syncControls();
		},
	});

	function showStatus(kind: "loading" | "error" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gomu-status" + (kind ? ` gomu-status--${kind}` : "");
	}

	/** What is picked, spelled out in the host's locale: a range says so as one
	 * phrase, and a half-made one says what it is still waiting for. */
	function summaryText(): string {
		const v = cal.value();
		if (v.start === "") return range ? "Pick the first day." : "Pick a date.";
		if (!range) return formatDate(v.start);
		if (v.end === "") return `From ${formatDate(v.start)}. Pick the last day.`;
		const nights = Math.round(
			((parseISO(v.end) as number) - (parseISO(v.start) as number)) / 86400000,
		);
		const days = nights + 1;
		return `${formatDateRange(v.start, v.end)} (${days} ${days === 1 ? "day" : "days"})`;
	}

	function syncControls(): void {
		const locked = phase !== "deciding";
		if (submitEl) submitEl.disabled = locked || !cal.complete();
		if (cancelEl) cancelEl.disabled = locked;
		if (summaryEl) {
			summaryEl.hidden = locked;
			summaryEl.textContent = summaryText();
		}
		// The controls are part of the decision, so they lock with it.
		for (const el of detailsEl?.querySelectorAll<HTMLInputElement | HTMLSelectElement>(
			"[data-gomu-input]",
		) ?? []) {
			el.disabled = locked;
		}
		calEl?.classList.toggle("gomu-cal--locked", locked);
	}

	function renderDetails(): void {
		if (detailsEl) {
			fillDescriptions(detailsEl, items, row, { values: inputs, disabled: phase !== "deciding" });
		}
	}

	// The end of the widget's life: the controls go, the outcome stays.
	function settle(message: string, kind: "accepted" | "declined"): void {
		phase = "settled";
		syncControls();
		if (decisionEl) decisionEl.hidden = true;
		if (outcomeEl) {
			outcomeEl.hidden = false;
			outcomeEl.textContent = message;
			outcomeEl.className = `gomu-datepicker-outcome gomu-datepicker-outcome--${kind}`;
		}
		showStatus("", "");
	}

	function fail(message: string): void {
		phase = "deciding";
		syncControls();
		showStatus("error", message);
	}

	// Runs one side of the decision. Returns the result, or null when the call
	// failed and the widget has already re-armed for a retry.
	async function call(
		tool: string,
		args: ActionCfg["args"],
		extra: Record<string, unknown>,
		fallback: string,
	): Promise<CallToolResult | null> {
		phase = "working";
		syncControls();
		showStatus("loading", "Working…");
		try {
			const action: ActionCfg = { label: "", kind: "tool", tool, args };
			const res = await bridge.callTool(tool, { ...resolveArgs(action, row, []), ...extra });
			if (res.isError) {
				fail(textOf(res) ?? fallback);
				return null;
			}
			return res;
		} catch (e) {
			fail(errorText(e, fallback));
			return null;
		}
	}

	// Hands the request to the host's chat instead of calling the tool: the
	// model makes the call, so there is no result to apply — only the turn
	// being accepted. Returns false when the host refused and the widget has
	// re-armed for a retry.
	async function chat(text: string): Promise<boolean> {
		phase = "working";
		syncControls();
		showStatus("loading", "Working…");
		try {
			await bridge.sendMessage(text);
			return true;
		} catch (e) {
			fail(errorText(e, "The request failed."));
			return false;
		}
	}

	async function submit(): Promise<void> {
		if (phase !== "deciding" || !cfg.submit?.tool || !cal.complete()) return;
		// The details may ask for more than the date, and an unanswered required
		// question is as good a reason to stop as a missing one.
		if (detailsEl && !validateInputs(detailsEl)) {
			showStatus("error", "Please fix the highlighted fields.");
			return;
		}
		const v = cal.value();
		if (cfg.submit.chatPrompt) {
			// A picker's whole output is the date, and a chat turn has no argument
			// to carry it, so it goes in the text. ISO keeps it unambiguous for the
			// model, whatever the reader's locale showed. Whatever the details
			// collected is spelled out after it, labelled as it was on screen.
			const picked = range && v.end ? `${v.start} to ${v.end}` : v.start;
			if (!(await chat(`${cfg.submit.chatPrompt} — chose: ${picked}${answers()}`))) return;
			settle(cfg.submit.successMessage || "Sent.", "accepted");
			return;
		}
		const args: Record<string, unknown> = {
			...(detailsEl ? collectInputs(detailsEl) : {}),
			[cfg.submit.valueArg || "date"]: v.start,
		};
		if (range && cfg.submit.endArg) args[cfg.submit.endArg] = v.end;
		const res = await call(cfg.submit.tool, cfg.submit.args, args, "The action failed.");
		if (!res) return;
		applyData(res);
		settle(cfg.submit.successMessage || textOf(res) || "Done.", "accepted");
	}

	/** What the detail controls hold, as a phrase to append to a chat turn:
	 * "; Guests: 3, Notes: late arrival". Empty when nothing was asked or
	 * nothing was answered. */
	function answers(): string {
		if (!detailsEl) return "";
		const values = collectInputs(detailsEl);
		const parts = items
			.filter((i) => i.input && i.input.name in values)
			.map((i) => `${i.label || i.input?.name}: ${String(values[i.input?.name ?? ""])}`);
		return parts.length > 0 ? `; ${parts.join(", ")}` : "";
	}

	async function cancel(): Promise<void> {
		if (phase !== "deciding" || !cfg.cancel) return;
		if (cfg.cancel.tool) {
			const res = await call(cfg.cancel.tool, cfg.cancel.args, {}, "Could not cancel.");
			if (!res) return;
		}
		settle(cfg.cancel.message || "Cancelled.", "declined");
	}

	/**
	 * Refreshes whichever parts of the widget a payload carries. Never changes
	 * the phase: a host may push results long after the decision.
	 *
	 * The value key carries either a date, or an object holding the selection and
	 * the grid's own limits — which days are still free is exactly the kind of
	 * thing that changes between registration and the question.
	 */
	function applyData(data: CallToolResult | { structuredContent?: unknown }): void {
		const sc = (data as CallToolResult).structuredContent;
		if (!sc || typeof sc !== "object") return;
		const content = sc as Record<string, unknown>;
		if (cfg.rowsKey in content) {
			row = rowsFrom(content, cfg.rowsKey)[0] ?? null;
		}
		const v = content[cfg.valueKey];
		if (typeof v === "string") {
			if (phase === "deciding") setSelection({ start: v, end: "" });
		} else if (v && typeof v === "object" && !Array.isArray(v)) {
			const o = v as Record<string, unknown>;
			const patch: Partial<CalendarCfg> = {};
			if (typeof o.min === "string") patch.min = o.min;
			if (typeof o.max === "string") patch.max = o.max;
			if (Array.isArray(o.disabled)) {
				patch.disabled = o.disabled.filter((d): d is string => typeof d === "string");
			}
			if (Object.keys(patch).length > 0) cal.patch(patch);
			if (phase === "deciding" && ("start" in o || "end" in o)) {
				setSelection({
					start: typeof o.start === "string" ? o.start : "",
					end: typeof o.end === "string" ? o.end : "",
				});
			}
		}
		renderDetails();
		syncControls();
	}

	function setSelection(v: DateValue): void {
		cal.setValue(v);
		syncControls();
	}

	submitEl?.addEventListener("click", () => void submit());
	cancelEl?.addEventListener("click", () => void cancel());

	// The list is rebuilt whenever the record changes, so answers are kept here
	// and handed back to it; a control the reader fixes clears its own message.
	if (detailsEl && hasInputs(items)) {
		watchInputs(detailsEl, (name, value, el) => {
			inputs[name] = value;
			if (el.checkValidity()) clearInputErrors(el.closest(".gomu-desc-item") ?? detailsEl);
		});
	}

	// Link values in a detail list go to the host: navigation is blocked inside
	// the sandboxed iframe.
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
		syncControls();
		showStatus("", "");
	});
	// Month names, the week break and today all come from the host context, so a
	// context change rebuilds the grid rather than only restyling it.
	document.addEventListener(HOST_CONTEXT_EVENT, () => {
		cal.render();
		renderDetails();
		syncControls();
	});

	if (cfg.default) {
		setSelection({ start: cfg.default, end: cfg.defaultEnd ?? "" });
	}
	renderDetails();
	cal.render();
	syncControls();

	// Baked snapshot: a selection, and bounds, if present.
	if (ctx.initialData) {
		applyData({ structuredContent: ctx.initialData });
	}

	// Load-time hydration: with a host connected, offer the days that are free
	// now rather than the ones that were free when the document was registered.
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
