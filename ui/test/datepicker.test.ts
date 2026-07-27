import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Bridge } from "../src/bridge";
import { setLocale } from "../src/format";
import { M } from "../src/protocol";
import { mountDatePicker } from "../src/widgets/datepicker";
import { FakeHost, flush } from "./fake-host";

const TODAY = "2026-07-16";

// Mirrors the markup datepicker_render.go emits, minus the chrome the behavior
// never touches.
function pickerShell({ cancel = true, details = true } = {}): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "datepicker");
	root.innerHTML = `
    <div class="gomu-datepicker-prompt"><h3 id="gomu-datepicker-question">Which nights?</h3></div>
    ${details ? `<dl class="gomu-descriptions" data-gomu-descriptions="" hidden></dl>` : ""}
    <div class="gomu-datepicker-body">
      <div class="gomu-cal" data-gomu-calendar=""></div>
    </div>
    <p class="gomu-datepicker-summary" data-gomu-summary="" hidden></p>
    <div class="gomu-datepicker-actions" data-gomu-decision="">
      ${cancel ? `<button type="button" data-gomu-cancel="">Cancel</button>` : ""}
      <button type="button" data-gomu-submit="" disabled>Continue</button>
    </div>
    <p class="gomu-datepicker-outcome" data-gomu-outcome="" hidden></p>
    <div class="gomu-statusbar"><div class="gomu-status" data-gomu-status="" hidden></div></div>`;
	document.body.append(root);
	return root;
}

function config(over: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		widget: "datepicker",
		valueKey: "value",
		rowsKey: "rows",
		rowId: "id",
		calendar: { mode: "range", months: 1, startOn: TODAY },
		details: [{ key: "reference", label: "Booking", type: "text" }],
		submit: {
			tool: "hold_room",
			valueArg: "from",
			endArg: "until",
			args: { booking: { row: "id" } },
			successMessage: "Held.",
		},
		cancel: { message: "Cancelled." },
		...over,
	};
}

const DATA = { rows: [{ id: 7, reference: "BKG-7" }] };

const el = <T extends HTMLElement>(root: HTMLElement, sel: string): T =>
	root.querySelector<T>(sel)!;
const submit = (root: HTMLElement) => el<HTMLButtonElement>(root, "[data-gomu-submit]");
const cancel = (root: HTMLElement) => el<HTMLButtonElement>(root, "[data-gomu-cancel]");
const summary = (root: HTMLElement) => el(root, "[data-gomu-summary]");
const outcome = (root: HTMLElement) => el(root, "[data-gomu-outcome]");
const decision = (root: HTMLElement) => el(root, "[data-gomu-decision]");
const status = (root: HTMLElement) => el(root, "[data-gomu-status]");

function day(root: HTMLElement, iso: string): HTMLButtonElement {
	const found = root.querySelector<HTMLButtonElement>(`[data-gomu-cal-day="${iso}"]`);
	if (!found) throw new Error(`no cell for ${iso}`);
	return found;
}

describe("datepicker behavior", () => {
	let host: FakeHost;
	let bridge: Bridge;

	beforeEach(async () => {
		vi.useFakeTimers({ toFake: ["Date"] });
		vi.setSystemTime(new Date(`${TODAY}T09:00:00Z`));
		setLocale("en-GB", "UTC");
		host = new FakeHost();
		bridge = new Bridge({ timeoutMs: 500 });
		await bridge.initialize();
		host.requests.length = 0;
	});

	afterEach(() => {
		bridge.dispose();
		host.dispose();
		vi.useRealTimers();
		setLocale(undefined, undefined);
		document.body.innerHTML = "";
	});

	it("builds the grid and states what is still needed", () => {
		const root = pickerShell();
		mountDatePicker({ root, config: config(), initialData: DATA, bridge });

		expect(root.querySelectorAll("[data-gomu-cal-day]")).toHaveLength(31);
		expect(submit(root).disabled).toBe(true);
		expect(summary(root).textContent).toBe("Pick the first day.");
		// The record the question is about.
		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("BKG-7");
	});

	it("arms the submit only once the range is finished", () => {
		const root = pickerShell();
		mountDatePicker({ root, config: config(), initialData: DATA, bridge });

		day(root, "2026-07-20").click();
		expect(submit(root).disabled).toBe(true);
		expect(summary(root).textContent).toContain("Pick the last day.");

		day(root, "2026-07-24").click();
		expect(submit(root).disabled).toBe(false);
		expect(summary(root).textContent).toContain("5 days");
	});

	it("submits a range through the chat with both ends appended", async () => {
		const root = pickerShell();
		mountDatePicker({
			root,
			config: config({
				submit: { tool: "hold_room", valueArg: "from", endArg: "until", chatPrompt: "Book the room" },
			}),
			initialData: DATA,
			bridge,
		});

		day(root, "2026-07-20").click();
		day(root, "2026-07-24").click();
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(0);
		// ISO in the message, whatever the reader's locale rendered on the grid.
		expect(host.received(M.message)[0]!.params).toMatchObject({
			role: "user",
			content: [{ type: "text", text: "Book the room — chose: 2026-07-20 to 2026-07-24" }],
		});
		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("Sent.");
	});

	it("submits a single date through the chat without a span", async () => {
		const root = pickerShell();
		mountDatePicker({
			root,
			config: config({
				calendar: { mode: "single", months: 1, startOn: TODAY },
				submit: { tool: "hold_room", valueArg: "on", chatPrompt: "Book the visit" },
			}),
			initialData: DATA,
			bridge,
		});

		day(root, "2026-07-20").click();
		submit(root).click();
		await flush();

		expect(host.received(M.message)[0]!.params).toMatchObject({
			content: [{ type: "text", text: "Book the visit — chose: 2026-07-20" }],
		});
	});

	it("submits both ends as flat arguments and settles", async () => {
		const root = pickerShell();
		mountDatePicker({ root, config: config(), initialData: DATA, bridge });

		day(root, "2026-07-20").click();
		day(root, "2026-07-24").click();
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "hold_room",
			arguments: { booking: 7, from: "2026-07-20", until: "2026-07-24" },
		});
		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("Held.");
		expect(outcome(root).className).toContain("gomu-datepicker-outcome--accepted");
		expect(status(root).hidden).toBe(true);
	});

	it("submits one date when there is only one to submit", async () => {
		const root = pickerShell();
		mountDatePicker({
			root,
			config: config({
				calendar: { mode: "single", months: 1, startOn: TODAY },
				submit: { tool: "schedule", valueArg: "date" },
			}),
			initialData: DATA,
			bridge,
		});

		day(root, "2026-07-20").click();
		expect(submit(root).disabled).toBe(false);
		expect(summary(root).textContent).toBe("20 Jul 2026");
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "schedule",
			arguments: { date: "2026-07-20" },
		});
	});

	it("sends what the detail controls collected along with the date", async () => {
		const root = pickerShell();
		const details = [
			{ key: "reference", label: "Booking", type: "text" },
			{
				key: "guests",
				label: "Guests",
				type: "text",
				input: { name: "guests", type: "number" as const, required: true, min: 1 },
			},
			{ key: "", label: "Late arrival", type: "text", input: { name: "late", type: "checkbox" as const } },
		];
		mountDatePicker({ root, config: config({ details }), initialData: DATA, bridge });

		// The record carries no guest count, so the control opens empty.
		const guests = el<HTMLInputElement>(root, '[data-gomu-input="guests"]');
		expect(guests.value).toBe("");
		guests.value = "3";
		guests.dispatchEvent(new Event("input", { bubbles: true }));
		const late = el<HTMLInputElement>(root, '[data-gomu-input="late"]');
		late.checked = true;
		late.dispatchEvent(new Event("change", { bubbles: true }));

		day(root, "2026-07-20").click();
		day(root, "2026-07-24").click();
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "hold_room",
			arguments: { booking: 7, from: "2026-07-20", until: "2026-07-24", guests: 3, late: true },
		});
		// The decision is over, so the controls are locked with it.
		expect(el<HTMLInputElement>(root, '[data-gomu-input="guests"]').disabled).toBe(true);
	});

	it("keeps a half-typed answer when a tool result re-renders the details", async () => {
		const root = pickerShell();
		const details = [
			{ key: "reference", label: "Booking", type: "text" },
			{ key: "", label: "Guests", type: "text", input: { name: "guests", type: "number" as const } },
		];
		mountDatePicker({ root, config: config({ details }), initialData: DATA, bridge });

		const guests = el<HTMLInputElement>(root, '[data-gomu-input="guests"]');
		guests.value = "4";
		guests.dispatchEvent(new Event("input", { bubbles: true }));

		host.notify(M.toolResult, { structuredContent: { rows: [{ id: 7, reference: "BKG-9" }] } });
		await flush();

		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("BKG-9");
		expect(el<HTMLInputElement>(root, '[data-gomu-input="guests"]').value).toBe("4");
	});

	it("refuses to submit while a required control is empty", async () => {
		const root = pickerShell();
		const details = [
			{
				key: "",
				label: "Guests",
				type: "text",
				input: { name: "guests", type: "number" as const, required: true, message: "How many?" },
			},
		];
		mountDatePicker({ root, config: config({ details }), initialData: DATA, bridge });

		day(root, "2026-07-20").click();
		day(root, "2026-07-24").click();
		submit(root).click();
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(status(root).textContent).toBe("Please fix the highlighted fields.");
		expect(el(root, "[data-gomu-input-error]").textContent).toBe("How many?");
		expect(submit(root).disabled).toBe(false);
	});

	it("spells the answers out after the date in a chat turn", async () => {
		const root = pickerShell();
		mountDatePicker({
			root,
			config: config({
				details: [
					{ key: "", label: "Guests", type: "text", input: { name: "guests", type: "number" as const, default: 2 } },
				],
				submit: { tool: "hold_room", valueArg: "from", endArg: "until", chatPrompt: "Book the room" },
			}),
			initialData: DATA,
			bridge,
		});

		day(root, "2026-07-20").click();
		day(root, "2026-07-24").click();
		submit(root).click();
		await flush();

		expect(host.received(M.message)[0]!.params).toMatchObject({
			content: [{ type: "text", text: "Book the room — chose: 2026-07-20 to 2026-07-24; Guests: 2" }],
		});
	});

	it("re-arms after a failed submit", async () => {
		const root = pickerShell();
		host.onToolCall = () => ({ isError: true, content: [{ type: "text", text: "Taken." }] });
		mountDatePicker({ root, config: config(), initialData: DATA, bridge });

		day(root, "2026-07-20").click();
		day(root, "2026-07-24").click();
		submit(root).click();
		await flush();

		expect(status(root).textContent).toBe("Taken.");
		expect(outcome(root).hidden).toBe(true);
		expect(submit(root).disabled).toBe(false);
	});

	it("cancels locally without calling anything", async () => {
		const root = pickerShell();
		mountDatePicker({ root, config: config(), initialData: DATA, bridge });

		cancel(root).click();
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(outcome(root).textContent).toBe("Cancelled.");
		expect(outcome(root).className).toContain("gomu-datepicker-outcome--declined");
		expect(submit(root).disabled).toBe(true);
	});

	it("starts on the authored default", () => {
		const root = pickerShell();
		mountDatePicker({
			root,
			config: config({ default: "2026-07-20", defaultEnd: "2026-07-24" }),
			initialData: DATA,
			bridge,
		});

		expect(submit(root).disabled).toBe(false);
		expect(day(root, "2026-07-20").classList.contains("gomu-cal-day--start")).toBe(true);
		expect(day(root, "2026-07-24").classList.contains("gomu-cal-day--end")).toBe(true);
	});

	it("takes the selection and the free days from a pushed result", async () => {
		const root = pickerShell();
		mountDatePicker({ root, config: config(), initialData: null, bridge });

		host.pushToolResult({
			structuredContent: {
				rows: [{ id: 9, reference: "BKG-9" }],
				value: {
					start: "2026-07-20",
					end: "2026-07-22",
					min: "2026-07-10",
					max: "2026-07-25",
					disabled: ["2026-07-18"],
				},
			},
		});
		await flush();

		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("BKG-9");
		expect(submit(root).disabled).toBe(false);
		expect(day(root, "2026-07-18").getAttribute("aria-disabled")).toBe("true");
		expect(day(root, "2026-07-09").getAttribute("aria-disabled")).toBe("true");
		expect(day(root, "2026-07-26").getAttribute("aria-disabled")).toBe("true");
	});

	it("accepts a bare date string as the pushed selection", async () => {
		const root = pickerShell();
		mountDatePicker({
			root,
			config: config({
				calendar: { mode: "single", months: 1, startOn: TODAY },
				submit: { tool: "schedule", valueArg: "date" },
			}),
			initialData: null,
			bridge,
		});

		host.pushToolResult({ structuredContent: { value: "2026-07-21" } });
		await flush();
		expect(summary(root).textContent).toBe("21 Jul 2026");
		expect(submit(root).disabled).toBe(false);
	});

	it("hydrates from a load tool once a host has answered", async () => {
		const root = pickerShell();
		host.onToolCall = () => ({ structuredContent: { value: { start: "2026-07-28", end: "2026-07-30" } } });
		mountDatePicker({
			root,
			config: config({ loadTool: "get_availability", loadArgs: { room: 4 } }),
			initialData: null,
			bridge,
			ready: Promise.resolve(true),
		});
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "get_availability",
			arguments: { room: 4 },
		});
		expect(submit(root).disabled).toBe(false);
	});

	it("leaves a settled decision settled when a late result arrives", async () => {
		const root = pickerShell();
		mountDatePicker({ root, config: config({ default: "2026-07-20", defaultEnd: "2026-07-24" }), initialData: DATA, bridge });

		submit(root).click();
		await flush();
		host.pushToolResult({ structuredContent: { value: { start: "2026-07-01", end: "2026-07-02" } } });
		await flush();

		expect(outcome(root).textContent).toBe("Held.");
		expect(decision(root).hidden).toBe(true);
		expect(day(root, "2026-07-20").classList.contains("gomu-cal-day--start")).toBe(true);
	});
});
