import { describe, expect, it } from "vitest";
import { errorText, isJSONPayload, textOf } from "../src/status";

describe("isJSONPayload", () => {
  it("recognises serialized objects and arrays", () => {
    expect(isJSONPayload('{"rows":[{"id":1,"name":"Ada"}]}')).toBe(true);
    expect(isJSONPayload("  [1,2,3]\n")).toBe(true);
    expect(isJSONPayload("{}")).toBe(true);
    expect(isJSONPayload("[]")).toBe(true);
  });

  it("leaves prose alone", () => {
    expect(isJSONPayload("Deleted.")).toBe(false);
    expect(isJSONPayload("")).toBe(false);
    expect(isJSONPayload("Use {braces} in a sentence.")).toBe(false);
    expect(isJSONPayload("{not json")).toBe(false);
    expect(isJSONPayload("42")).toBe(false);
    expect(isJSONPayload('"quoted"')).toBe(false);
  });
});

describe("textOf", () => {
  it("returns the first prose text block", () => {
    expect(textOf({ content: [{ type: "text", text: "Deleted." }] })).toBe("Deleted.");
  });

  it("skips a serialized structuredContent mirror", () => {
    // Servers echo structuredContent into a text block when the handler leaves
    // the content field empty; that JSON must never become a status message.
    expect(
      textOf({
        content: [{ type: "text", text: '{"rows":[{"id":1}]}' }],
        structuredContent: { rows: [{ id: 1 }] },
      }),
    ).toBeUndefined();
  });

  it("finds the prose block behind a JSON one", () => {
    expect(
      textOf({
        content: [
          { type: "text", text: '{"rows":[]}' },
          { type: "text", text: "Nothing left." },
        ],
      }),
    ).toBe("Nothing left.");
  });

  it("skips blank and non-text blocks", () => {
    expect(
      textOf({
        content: [
          { type: "image", data: "…", mimeType: "image/png" },
          { type: "text", text: "   " },
          { type: "text", text: "Saved." },
        ],
      } as never),
    ).toBe("Saved.");
    expect(textOf({})).toBeUndefined();
  });
});

describe("errorText", () => {
  it("uses the error message when it reads as a message", () => {
    expect(errorText(new Error("timed out"), "fallback")).toBe("timed out");
    expect(errorText("plain string", "fallback")).toBe("plain string");
  });

  it("falls back for empty and JSON messages", () => {
    expect(errorText(new Error(""), "The action failed.")).toBe("The action failed.");
    expect(errorText(new Error('{"code":-32000}'), "The action failed.")).toBe(
      "The action failed.",
    );
  });
});
