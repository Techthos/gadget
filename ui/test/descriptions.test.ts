import { afterEach, describe, expect, it } from "vitest";
import {
	clearInputErrors,
	collectInputs,
	fillDescriptions,
	hasInputs,
	showInputErrors,
	validateInputs,
	watchInputs,
} from "../src/widgets/descriptions";

function host(): HTMLElement {
	document.body.innerHTML = "";
	const dl = document.createElement("dl");
	dl.className = "gomu-descriptions";
	dl.setAttribute("data-gomu-descriptions", "");
	dl.hidden = true;
	document.body.append(dl);
	return dl;
}

function labels(dl: HTMLElement): string[] {
	return [...dl.querySelectorAll("dt")].map((el) => el.textContent ?? "");
}

function values(dl: HTMLElement): string[] {
	return [...dl.querySelectorAll("dd")].map((el) => el.textContent ?? "");
}

const ROW = {
	id: 1,
	name: "Ada",
	balance: 12.5,
	status: "active",
	website: "https://example.com",
};

describe("descriptions", () => {
	afterEach(() => {
		document.body.innerHTML = "";
	});

	it("renders a label and value per item, in order", () => {
		const dl = host();
		fillDescriptions(
			dl,
			[
				{ key: "name", label: "User", type: "text" },
				{ key: "balance", label: "Balance", type: "number", format: "currency:EUR" },
			],
			ROW,
		);
		expect(dl.hidden).toBe(false);
		expect(labels(dl)).toEqual(["User", "Balance"]);
		expect(values(dl)[0]).toBe("Ada");
		// Intl output varies by environment; the currency is what matters.
		expect(values(dl)[1]).toMatch(/12[.,]50/);
	});

	it("renders an authored text value without touching the record", () => {
		const dl = host();
		fillDescriptions(dl, [{ key: "", label: "Region", type: "text", text: "eu-central-1" }], null);
		expect(values(dl)).toEqual(["eu-central-1"]);
	});

	it("shows an em dash for a value the record does not carry", () => {
		const dl = host();
		fillDescriptions(dl, [{ key: "missing", label: "Nope", type: "text" }], ROW);
		expect(values(dl)).toEqual(["—"]);
		expect(dl.querySelector("dd")?.className).toContain("gomu-desc-value--missing");
	});

	it("renders a badge value as a variant-mapped pill", () => {
		const dl = host();
		fillDescriptions(
			dl,
			[{ key: "status", label: "Status", type: "badge", badge: { active: "success" } }],
			ROW,
		);
		const badge = dl.querySelector(".gomu-badge");
		expect(badge?.textContent).toBe("active");
		expect(badge?.className).toContain("gomu-badge--success");
	});

	it("renders a link value as a button carrying the href", () => {
		const dl = host();
		fillDescriptions(
			dl,
			[{ key: "", label: "Website", type: "link", link: { hrefKey: "website" } }],
			ROW,
		);
		const link = dl.querySelector<HTMLElement>("[data-gomu-link]");
		expect(link?.tagName).toBe("BUTTON");
		expect(link?.getAttribute("data-gomu-link")).toBe("https://example.com");
	});

	it("replaces previous content and hides itself when there is nothing to show", () => {
		const dl = host();
		fillDescriptions(dl, [{ key: "name", label: "User", type: "text" }], ROW);
		expect(labels(dl)).toHaveLength(1);

		fillDescriptions(dl, [], ROW);
		expect(labels(dl)).toHaveLength(0);
		expect(dl.hidden).toBe(true);
	});

	it("renders a control for an item that asks instead of states", () => {
		const dl = host();
		fillDescriptions(
			dl,
			[
				{ key: "", label: "Guests", type: "text", input: { name: "guests", type: "number" } },
				{ key: "", label: "Bed", type: "text", input: { name: "bed", type: "select", options: [{ value: "double", label: "Double" }] } },
				{ key: "", label: "Late", type: "text", input: { name: "late", type: "checkbox" } },
				{ key: "", label: "Notes", type: "text", input: { name: "notes", type: "text" } },
			],
			null,
		);
		expect(hasInputs([{ key: "name", label: "User", type: "text" }])).toBe(false);
		const number = dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!;
		expect(number.type).toBe("number");
		expect(dl.querySelector('select[data-gomu-input="bed"]')).not.toBeNull();
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="late"]')!.type).toBe("checkbox");
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="notes"]')!.type).toBe("text");
		// The label addresses the control, the way a form field's does.
		const label = dl.querySelector("dt label")!;
		expect(label.getAttribute("for")).toBe(number.id);
	});

	it("opens a control on the answer, then the record, then the default", () => {
		const dl = host();
		const items = [
			{ key: "guests", label: "Guests", type: "text", input: { name: "guests", type: "number" as const, default: 2 } },
			{ key: "", label: "Notes", type: "text", input: { name: "notes", type: "text" as const, default: "none" } },
		];

		// No record and no answer: the authored default.
		fillDescriptions(dl, items, null);
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!.value).toBe("2");
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="notes"]')!.value).toBe("none");

		// The record prefills over the default.
		fillDescriptions(dl, items, { guests: 5 });
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!.value).toBe("5");

		// What the reader already answered wins over both.
		fillDescriptions(dl, items, { guests: 5 }, { values: { guests: 3 } });
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!.value).toBe("3");
	});

	it("collects typed values and omits an empty number", () => {
		const dl = host();
		fillDescriptions(
			dl,
			[
				{ key: "", label: "Guests", type: "text", input: { name: "guests", type: "number" } },
				{ key: "", label: "Rooms", type: "text", input: { name: "rooms", type: "number" } },
				{ key: "", label: "Late", type: "text", input: { name: "late", type: "checkbox" } },
				{ key: "", label: "Notes", type: "text", input: { name: "notes", type: "text" } },
			],
			null,
		);
		dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!.value = "3";
		dl.querySelector<HTMLInputElement>('[data-gomu-input="late"]')!.checked = true;
		dl.querySelector<HTMLInputElement>('[data-gomu-input="notes"]')!.value = "arriving late";

		expect(collectInputs(dl)).toEqual({ guests: 3, late: true, notes: "arriving late" });
	});

	it("reports what the reader changes and keeps disabled controls out of reach", () => {
		const dl = host();
		const items = [
			{ key: "", label: "Guests", type: "text", input: { name: "guests", type: "number" as const } },
		];
		fillDescriptions(dl, items, null);
		const seen: Array<[string, unknown]> = [];
		watchInputs(dl, (name, value) => seen.push([name, value]));

		const input = dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!;
		input.value = "4";
		input.dispatchEvent(new Event("input", { bubbles: true }));
		expect(seen).toEqual([["guests", 4]]);

		fillDescriptions(dl, items, null, { disabled: true });
		expect(dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!.disabled).toBe(true);
	});

	it("shows why a control is invalid, and clears it once it is not", () => {
		const dl = host();
		fillDescriptions(
			dl,
			[
				{
					key: "",
					label: "Guests",
					type: "text",
					input: { name: "guests", type: "number", required: true, min: 1, message: "Between 1 and 8." },
				},
			],
			null,
		);
		const input = dl.querySelector<HTMLInputElement>('[data-gomu-input="guests"]')!;

		expect(validateInputs(dl)).toBe(false);
		expect(input.getAttribute("aria-invalid")).toBe("true");
		expect(dl.querySelector("[data-gomu-input-error]")!.textContent).toBe("Between 1 and 8.");

		input.value = "3";
		expect(validateInputs(dl)).toBe(true);
		const slot = dl.querySelector<HTMLElement>("[data-gomu-input-error]")!;
		expect(slot.hidden).toBe(true);
		expect(input.hasAttribute("aria-invalid")).toBe(false);

		// A server can name the same control in its own errors.
		expect(showInputErrors(dl, { guests: "Fully booked." })).toBe(1);
		expect(slot.textContent).toBe("Fully booked.");
		clearInputErrors(dl);
		expect(slot.hidden).toBe(true);
	});

	it("renders values as text, never as markup", () => {
		const dl = host();
		fillDescriptions(dl, [{ key: "name", label: "User", type: "text" }], {
			name: "<img src=x onerror=alert(1)>",
		});
		expect(dl.querySelector("img")).toBeNull();
		expect(values(dl)[0]).toBe("<img src=x onerror=alert(1)>");
	});
});
