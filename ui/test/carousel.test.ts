import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { carouselState, stepFor } from "../src/widgets/carousel";
import { boot } from "../src/index";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

describe("carousel scroll math", () => {
	it("reports no overflow when the cards fit", () => {
		const s = carouselState({ scrollLeft: 0, scrollWidth: 400, clientWidth: 400 });
		expect(s.overflowing).toBe(false);
		// Both ends are "reached" so neither control is offered.
		expect(s.atStart).toBe(true);
		expect(s.atEnd).toBe(true);
	});

	it("tracks both ends of an overflowing strip", () => {
		const m = { scrollWidth: 1200, clientWidth: 400 };
		expect(carouselState({ ...m, scrollLeft: 0 })).toMatchObject({
			overflowing: true,
			atStart: true,
			atEnd: false,
		});
		expect(carouselState({ ...m, scrollLeft: 360 })).toMatchObject({
			atStart: false,
			atEnd: false,
		});
		expect(carouselState({ ...m, scrollLeft: 800 })).toMatchObject({
			atStart: false,
			atEnd: true,
		});
	});

	it("tolerates fractional scroll offsets at the end", () => {
		const s = carouselState({ scrollLeft: 799.4, scrollWidth: 1200, clientWidth: 400 });
		expect(s.atEnd).toBe(true);
	});

	it("treats negative RTL offsets by magnitude", () => {
		const m = { scrollWidth: 1200, clientWidth: 400 };
		expect(carouselState({ ...m, scrollLeft: -800 }).atEnd).toBe(true);
		expect(carouselState({ ...m, scrollLeft: -0.5 }).atStart).toBe(true);
	});

	it("steps just under one viewport, signed by direction", () => {
		const m = { scrollLeft: 0, scrollWidth: 1200, clientWidth: 400 };
		expect(stepFor(m, "next")).toBe(360);
		expect(stepFor(m, "prev")).toBe(-360);
		// In RTL the visual "next" is a leftward scroll.
		expect(stepFor(m, "next", true)).toBe(-360);
		expect(stepFor(m, "prev", true)).toBe(360);
	});
});

describe("brand link", () => {
	let host: FakeHost;

	beforeEach(() => {
		host = new FakeHost();
	});

	afterEach(() => {
		host.dispose();
		document.body.innerHTML = "";
	});

	it("opens the brand URL through the host", async () => {
		document.body.innerHTML = `
      <div class="gadget-root" data-gadget-widget="cardlist">
        <button type="button" class="gadget-brand" data-gadget-brand="https://acme.test">
          <span class="gadget-brand-name">Acme</span>
        </button>
      </div>`;
		await boot();
		document.querySelector<HTMLElement>("[data-gadget-brand]")?.click();
		await flush();

		const opened = host.received(M.openLink);
		expect(opened).toHaveLength(1);
		expect(opened[0]?.params).toMatchObject({ url: "https://acme.test" });
	});
});
