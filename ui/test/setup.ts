// jsdom does not implement CSSOM's CSS.escape, which the runtime uses to build
// selectors when it restores focus after a re-render (see cardlist.ts). Provide
// the standard algorithm so tests exercise the same path a browser does.
// Source: the CSSOM "serialize an identifier" steps (mathiasbynens/CSS.escape).
function cssEscape(value: string): string {
	const str = String(value);
	const length = str.length;
	const first = str.charCodeAt(0);
	let result = "";
	for (let i = 0; i < length; i++) {
		const c = str.charCodeAt(i);
		if (c === 0x0000) {
			result += "�";
		} else if (
			(c >= 0x0001 && c <= 0x001f) ||
			c === 0x007f ||
			(i === 0 && c >= 0x0030 && c <= 0x0039) ||
			(i === 1 && c >= 0x0030 && c <= 0x0039 && first === 0x002d)
		) {
			result += "\\" + c.toString(16) + " ";
		} else if (i === 0 && length === 1 && c === 0x002d) {
			result += "\\" + str.charAt(i);
		} else if (
			c >= 0x0080 ||
			c === 0x002d ||
			c === 0x005f ||
			(c >= 0x0030 && c <= 0x0039) ||
			(c >= 0x0041 && c <= 0x005a) ||
			(c >= 0x0061 && c <= 0x007a)
		) {
			result += str.charAt(i);
		} else {
			result += "\\" + str.charAt(i);
		}
	}
	return result;
}

const g = globalThis as unknown as { CSS?: { escape?: (v: string) => string } };
if (!g.CSS) g.CSS = {};
if (typeof g.CSS.escape !== "function") g.CSS.escape = cssEscape;
