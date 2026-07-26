import { afterEach, describe, expect, it } from "vitest";
import { fillDescriptions } from "../src/widgets/descriptions";

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

	it("renders values as text, never as markup", () => {
		const dl = host();
		fillDescriptions(dl, [{ key: "name", label: "User", type: "text" }], {
			name: "<img src=x onerror=alert(1)>",
		});
		expect(dl.querySelector("img")).toBeNull();
		expect(values(dl)[0]).toBe("<img src=x onerror=alert(1)>");
	});
});
