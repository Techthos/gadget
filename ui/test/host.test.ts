import { afterEach, describe, expect, it } from "vitest";
import { applyHostContext } from "../src/host";
import { formatCell, setLocale } from "../src/format";

afterEach(() => {
  document.documentElement.removeAttribute("data-gadget-theme");
  document.documentElement.removeAttribute("style");
  document.getElementById("gadget-host-fonts")?.remove();
  setLocale(undefined, undefined);
});

describe("applyHostContext", () => {
  it("applies custom-property variables and rejects other keys", () => {
    applyHostContext({
      styles: {
        variables: {
          "--color-background-primary": "#111",
          "color": "red", // not a custom property — must be ignored
        },
      },
    });
    const root = document.documentElement;
    expect(root.style.getPropertyValue("--color-background-primary")).toBe("#111");
    expect(root.style.getPropertyValue("color")).toBe("");
  });

  it("sets the theme attribute for valid themes only", () => {
    applyHostContext({ theme: "dark" });
    expect(document.documentElement.getAttribute("data-gadget-theme")).toBe("dark");
    applyHostContext({ theme: "purple" as never });
    expect(document.documentElement.getAttribute("data-gadget-theme")).toBe("dark");
  });

  it("injects host fonts once and replaces on update", () => {
    applyHostContext({ styles: { css: { fonts: "@font-face{font-family:A}" } } });
    applyHostContext({ styles: { css: { fonts: "@font-face{font-family:B}" } } });
    const els = document.querySelectorAll("#gadget-host-fonts");
    expect(els).toHaveLength(1);
    expect(els[0]!.textContent).toContain("font-family:B");
  });

  it("tolerates null and empty contexts", () => {
    expect(() => applyHostContext(null)).not.toThrow();
    expect(() => applyHostContext({})).not.toThrow();
  });
});

describe("formatCell", () => {
  it("formats numbers by format spec", () => {
    setLocale("en-US", undefined);
    expect(formatCell(1234.567, "number", "int")).toBe("1,235");
    expect(formatCell(1234.5, "number", "decimal:2")).toBe("1,234.50");
    expect(formatCell(0.42, "number", "percent")).toBe("42%");
    expect(formatCell(9.5, "number", "currency:EUR")).toBe("€9.50");
  });

  it("respects the host locale", () => {
    setLocale("de-DE", undefined);
    expect(formatCell(1234.5, "number", "decimal:2")).toBe("1.234,50");
  });

  it("formats dates with the host time zone", () => {
    setLocale("en-US", "UTC");
    expect(formatCell("2026-07-22T10:30:00Z", "date", "datetime")).toContain("10:30");
    expect(formatCell("2026-07-22T10:30:00Z", "date", "date")).toContain("2026");
  });

  it("falls back to string form for malformed values", () => {
    expect(formatCell("not-a-date", "date", "date")).toBe("not-a-date");
    expect(formatCell("NaN?", "number", "int")).toBe("NaN?");
    expect(formatCell(null, "text")).toBe("");
  });
});
