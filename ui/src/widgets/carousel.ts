// Pure scroll math for the CardList carousel. Kept free of DOM access so the
// behavior can be unit-tested without layout (jsdom reports zero for every
// box measurement).

export interface CarouselMetrics {
	scrollLeft: number;
	scrollWidth: number;
	clientWidth: number;
}

export interface CarouselState {
	overflowing: boolean;
	atStart: boolean;
	atEnd: boolean;
}

// Sub-pixel slack: browsers report fractional scroll offsets, so an exact
// comparison never registers the end of the strip.
const EPSILON = 1;

// Fraction of the visible width one prev/next click travels. Less than a full
// width so the card at the edge stays partly visible as a continuity cue.
const STEP_RATIO = 0.9;

export function carouselState(m: CarouselMetrics): CarouselState {
	const overflowing = m.scrollWidth > m.clientWidth + EPSILON;
	// In RTL the scroll origin is the right edge and scrollLeft runs negative
	// (or, on legacy engines, positive-decreasing); the distance from either
	// end is what matters, so compare on magnitudes.
	const offset = Math.abs(m.scrollLeft);
	const max = Math.max(0, m.scrollWidth - m.clientWidth);
	return {
		overflowing,
		atStart: !overflowing || offset <= EPSILON,
		atEnd: !overflowing || offset >= max - EPSILON,
	};
}

// Signed pixel delta for one step in `dir`, ready to hand to scrollBy().
export function stepFor(m: CarouselMetrics, dir: "prev" | "next", rtl = false): number {
	const step = Math.round(m.clientWidth * STEP_RATIO);
	const forward = dir === "next";
	return (forward === rtl ? -step : step);
}
