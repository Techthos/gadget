import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountConfirm } from "../src/widgets/confirm";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

// Mirrors the markup confirm_render.go emits, minus the chrome the behavior
// never touches.
function confirmShell({
	ack = false,
	phrase = false,
	reject = true,
	details = true,
} = {}): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "confirm");
	const guarded = ack || phrase;
	root.innerHTML = `
    <div class="gomu-confirm-prompt"><h3 class="gomu-confirm-question">Delete Ada?</h3></div>
    ${details ? `<dl class="gomu-descriptions" data-gomu-descriptions="" hidden></dl>` : ""}
    <ul class="gomu-effects" data-gomu-effects="" hidden></ul>
    ${
			guarded
				? `<div class="gomu-confirm-guards">
             ${ack ? `<label><input type="checkbox" data-gomu-ack=""><span>I understand.</span></label>` : ""}
             ${phrase ? `<div><label for="p">Type</label><input id="p" type="text" data-gomu-phrase=""></div>` : ""}
           </div>`
				: ""
		}
    <div class="gomu-confirm-actions" data-gomu-decision="">
      ${reject ? `<button type="button" data-gomu-reject="">Cancel</button>` : ""}
      <button type="button" data-gomu-accept="" ${guarded ? "disabled" : ""}>Confirm</button>
    </div>
    <p class="gomu-confirm-outcome" data-gomu-outcome="" hidden></p>
    <div class="gomu-statusbar"><div class="gomu-status" data-gomu-status="" hidden></div></div>`;
	document.body.append(root);
	return root;
}

function config(over: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		widget: "confirm",
		rowsKey: "rows",
		effectsKey: "effects",
		rowId: "id",
		accept: { tool: "delete_user", args: { id: { row: "id" } }, successMessage: "User deleted." },
		reject: { message: "Cancelled." },
		details: [
			{ key: "name", label: "User", type: "text" },
			{ key: "", label: "Region", type: "text", text: "eu-central-1" },
		],
		effects: [{ text: "Removes the account", severity: "danger" }],
		...over,
	};
}

const DATA = { rows: [{ id: 7, name: "Ada" }] };

const el = <T extends HTMLElement>(root: HTMLElement, sel: string): T =>
	root.querySelector<T>(sel)!;
const accept = (root: HTMLElement) => el<HTMLButtonElement>(root, "[data-gomu-accept]");
const reject = (root: HTMLElement) => el<HTMLButtonElement>(root, "[data-gomu-reject]");
const outcome = (root: HTMLElement) => el(root, "[data-gomu-outcome]");
const decision = (root: HTMLElement) => el(root, "[data-gomu-decision]");
const status = (root: HTMLElement) => el(root, "[data-gomu-status]");
const effects = (root: HTMLElement) => el(root, "[data-gomu-effects]");

describe("confirm behavior", () => {
	let host: FakeHost;
	let bridge: Bridge;

	beforeEach(async () => {
		host = new FakeHost();
		bridge = new Bridge({ timeoutMs: 500 });
		await bridge.initialize();
		host.requests.length = 0;
	});

	afterEach(() => {
		bridge.dispose();
		host.dispose();
		document.body.innerHTML = "";
	});

	it("renders the details from the record and the authored effects", () => {
		const root = confirmShell();
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		const dl = el(root, "[data-gomu-descriptions]");
		expect(dl.hidden).toBe(false);
		expect([...dl.querySelectorAll("dd")].map((d) => d.textContent)).toEqual([
			"Ada",
			"eu-central-1",
		]);
		expect(effects(root).hidden).toBe(false);
		expect(effects(root).querySelector(".gomu-effect")?.className).toContain(
			"gomu-effect--danger",
		);
	});

	it("accepts: calls the tool with resolved args and settles", async () => {
		const root = confirmShell();
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		accept(root).click();
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "delete_user",
			arguments: { id: 7 },
		});
		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).hidden).toBe(false);
		expect(outcome(root).textContent).toBe("User deleted.");
		expect(outcome(root).className).toContain("gomu-confirm-outcome--accepted");
		expect(status(root).hidden).toBe(true);
	});

	it("accepts through the chat when the accept carries a chatPrompt", async () => {
		const root = confirmShell();
		mountConfirm({
			root,
			config: config({
				accept: { tool: "delete_user", chatPrompt: "Delete the account for Ada" },
			}),
			initialData: DATA,
			bridge,
		});

		accept(root).click();
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(host.received(M.message)[0]!.params).toMatchObject({
			role: "user",
			content: [{ type: "text", text: "Delete the account for Ada" }],
		});
		// The decision is still made: the widget settles rather than staying armed.
		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("Sent.");
		expect(outcome(root).className).toContain("gomu-confirm-outcome--accepted");
	});

	it("re-arms when the host refuses the chat turn", async () => {
		const root = confirmShell();
		host.mute.add(M.message);
		mountConfirm({
			root,
			config: config({
				accept: { tool: "delete_user", chatPrompt: "Delete the account for Ada" },
			}),
			initialData: DATA,
			bridge,
		});

		accept(root).click();
		await new Promise((r) => setTimeout(r, 600)); // past the 500ms bridge timeout

		expect(decision(root).hidden).toBe(false);
		expect(status(root).textContent).toContain("timed out");
	});

	it("falls back to the result's text when no success message is configured", async () => {
		const root = confirmShell();
		host.onToolCall = () => ({ content: [{ type: "text", text: "Gone." }] });
		mountConfirm({
			root,
			config: config({ accept: { tool: "delete_user" } }),
			initialData: DATA,
			bridge,
		});

		accept(root).click();
		await flush();
		expect(outcome(root).textContent).toBe("Gone.");
	});

	it("cannot be accepted twice", async () => {
		const root = confirmShell();
		host.onToolCall = () => ({ structuredContent: {} });
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		accept(root).click();
		await flush();
		accept(root).dispatchEvent(new MouseEvent("click", { bubbles: true }));
		await flush();

		expect(host.received(M.toolsCall)).toHaveLength(1);
		expect(accept(root).disabled).toBe(true);
	});

	it("stays inert while the accept call is in flight", async () => {
		const root = confirmShell();
		let release: (() => void) | undefined;
		host.onToolCall = () =>
			new Promise((resolve) => {
				release = () => resolve({ structuredContent: {} });
			});
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		accept(root).click();
		await flush();
		expect(accept(root).disabled).toBe(true);
		expect(reject(root).disabled).toBe(true);
		expect(status(root).textContent).toBe("Working…");
		expect(status(root).className).toContain("gomu-status--loading");

		release!();
		await flush();
		expect(decision(root).hidden).toBe(true);
	});

	it("re-arms after a failed accept so it can be retried", async () => {
		const root = confirmShell();
		host.onToolCall = () => ({ isError: true, content: [{ type: "text", text: "Locked." }] });
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		accept(root).click();
		await flush();
		expect(status(root).textContent).toBe("Locked.");
		expect(status(root).className).toContain("gomu-status--error");
		expect(decision(root).hidden).toBe(false);
		expect(accept(root).disabled).toBe(false);

		host.onToolCall = () => ({ structuredContent: {} });
		accept(root).click();
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(2);
		expect(decision(root).hidden).toBe(true);
	});

	it("rejects without a tool: no call, terminal message", async () => {
		const root = confirmShell();
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		reject(root).click();
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(outcome(root).textContent).toBe("Cancelled.");
		expect(outcome(root).className).toContain("gomu-confirm-outcome--declined");
		expect(decision(root).hidden).toBe(true);
	});

	it("rejects with a tool: calls it before settling", async () => {
		const root = confirmShell();
		mountConfirm({
			root,
			config: config({
				reject: { tool: "cancel_deletion", args: { id: { row: "id" } }, message: "Kept." },
			}),
			initialData: DATA,
			bridge,
		});

		reject(root).click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "cancel_deletion",
			arguments: { id: 7 },
		});
		expect(outcome(root).textContent).toBe("Kept.");
	});

	it("keeps accept disabled until the acknowledgement is ticked", async () => {
		const root = confirmShell({ ack: true });
		mountConfirm({ root, config: config({ acknowledge: true }), initialData: DATA, bridge });

		expect(accept(root).disabled).toBe(true);
		const box = el<HTMLInputElement>(root, "[data-gomu-ack]");
		box.checked = true;
		box.dispatchEvent(new Event("change", { bubbles: true }));
		expect(accept(root).disabled).toBe(false);

		box.checked = false;
		box.dispatchEvent(new Event("change", { bubbles: true }));
		expect(accept(root).disabled).toBe(true);
	});

	it("keeps accept disabled until the phrase matches exactly", () => {
		const root = confirmShell({ phrase: true });
		mountConfirm({
			root,
			config: config({ typeToConfirm: "ada@example.com" }),
			initialData: DATA,
			bridge,
		});

		const input = el<HTMLInputElement>(root, "[data-gomu-phrase]");
		input.value = "ada@example.co";
		input.dispatchEvent(new Event("input", { bubbles: true }));
		expect(accept(root).disabled).toBe(true);

		input.value = "  ada@example.com  ";
		input.dispatchEvent(new Event("input", { bubbles: true }));
		expect(accept(root).disabled).toBe(false);
	});

	it("requires every configured guard", () => {
		const root = confirmShell({ ack: true, phrase: true });
		mountConfirm({
			root,
			config: config({ acknowledge: true, typeToConfirm: "yes" }),
			initialData: DATA,
			bridge,
		});

		const box = el<HTMLInputElement>(root, "[data-gomu-ack]");
		box.checked = true;
		box.dispatchEvent(new Event("change", { bubbles: true }));
		expect(accept(root).disabled).toBe(true);

		const input = el<HTMLInputElement>(root, "[data-gomu-phrase]");
		input.value = "yes";
		input.dispatchEvent(new Event("input", { bubbles: true }));
		expect(accept(root).disabled).toBe(false);
	});

	it("does not fire while a guard is unsatisfied", async () => {
		const root = confirmShell({ phrase: true });
		mountConfirm({ root, config: config({ typeToConfirm: "yes" }), initialData: DATA, bridge });

		accept(root).dispatchEvent(new MouseEvent("click", { bubbles: true }));
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
	});

	it("replaces the record and the effects from a pushed tool result", async () => {
		const root = confirmShell();
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		host.pushToolResult({
			structuredContent: {
				rows: [{ id: 9, name: "Grace" }],
				effects: [
					{ text: "Deletes audit records", detail: "Not recoverable.", value: "12", severity: "warning" },
				],
			},
		});
		await flush();

		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("Grace");
		const rows = [...effects(root).querySelectorAll(".gomu-effect")];
		expect(rows).toHaveLength(1);
		expect(rows[0]!.className).toContain("gomu-effect--warning");
		expect(rows[0]!.querySelector(".gomu-effect-detail")?.textContent).toBe("Not recoverable.");
		expect(rows[0]!.querySelector(".gomu-effect-value")?.textContent).toBe("12");
	});

	it("keeps a decided widget decided when the host pushes a later result", async () => {
		const root = confirmShell();
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		reject(root).click();
		await flush();
		host.pushToolResult({ structuredContent: { rows: [{ id: 9, name: "Grace" }] } });
		await flush();

		expect(decision(root).hidden).toBe(true);
		expect(outcome(root).textContent).toBe("Cancelled.");
		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("Grace");
	});

	it("ignores an effect severity that is not a known variant", async () => {
		const root = confirmShell();
		mountConfirm({ root, config: config(), initialData: DATA, bridge });

		host.pushToolResult({
			structuredContent: { effects: [{ text: "Odd", severity: "danger; injected" }] },
		});
		await flush();
		expect(effects(root).querySelector(".gomu-effect")?.className).toBe("gomu-effect");
	});

	it("hydrates from LoadTool once a host is connected", async () => {
		const root = confirmShell();
		host.onToolCall = () => ({
			structuredContent: { rows: [{ id: 3, name: "Grace" }], effects: [{ text: "Fresh" }] },
		});
		mountConfirm({
			root,
			config: config({ loadTool: "get_user", loadArgs: { id: 3 } }),
			initialData: DATA,
			bridge,
			ready: Promise.resolve(true),
		});
		await flush();

		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "get_user",
			arguments: { id: 3 },
		});
		expect(el(root, "[data-gomu-descriptions] dd").textContent).toBe("Grace");
		expect(effects(root).querySelector(".gomu-effect-label")?.textContent).toBe("Fresh");
		expect(status(root).hidden).toBe(true);
	});

	it("does not hydrate without a host", async () => {
		const root = confirmShell();
		mountConfirm({
			root,
			config: config({ loadTool: "get_user" }),
			initialData: DATA,
			bridge,
			ready: Promise.resolve(false),
		});
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
	});

	it("opens a detail link through the host", async () => {
		const root = confirmShell();
		mountConfirm({
			root,
			config: config({
				details: [{ key: "", label: "Website", type: "link", link: { hrefKey: "website" } }],
			}),
			initialData: { rows: [{ id: 7, website: "https://example.com" }] },
			bridge,
		});

		el<HTMLElement>(root, "[data-gomu-link]").click();
		await flush();
		expect(host.received(M.openLink)[0]!.params).toMatchObject({ url: "https://example.com" });
	});
});
