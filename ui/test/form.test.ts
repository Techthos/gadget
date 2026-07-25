import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountForm } from "../src/widgets/form";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

function shell(): HTMLElement {
  document.body.innerHTML = "";
  const root = document.createElement("div");
  root.className = "gadget-root";
  root.setAttribute("data-gadget-widget", "form");
  root.innerHTML = `
    <div data-gadget-status="" hidden></div>
    <form data-gadget-form="">
      <div class="gadget-field">
        <label for="f-name">Name</label>
        <input id="f-name" name="name" type="text" required>
        <p data-gadget-error-for="name" hidden></p>
      </div>
      <div class="gadget-field">
        <label for="f-age">Age</label>
        <input id="f-age" name="age" type="number">
        <p data-gadget-error-for="age" hidden></p>
      </div>
      <div class="gadget-field">
        <input id="f-active" name="active" type="checkbox">
        <label for="f-active">Active</label>
        <p data-gadget-error-for="active" hidden></p>
      </div>
      <div class="gadget-field">
        <label for="f-tags">Tags</label>
        <select id="f-tags" name="tags" multiple>
          <option value="a">A</option>
          <option value="b">B</option>
        </select>
        <p data-gadget-error-for="tags" hidden></p>
      </div>
      <div class="gadget-form-actions">
        <button type="button" data-gadget-cancel="">Cancel</button>
        <button type="button" data-gadget-submit="">Save</button>
      </div>
    </form>`;
  document.body.append(root);
  return root;
}

function config(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    widget: "form",
    prefillKey: "values",
    errorsKey: "errors",
    submit: { tool: "save_user", successMessage: "User saved." },
    fields: [
      { name: "name", type: "text", message: "Name is required." },
      { name: "age", type: "number" },
      { name: "active", type: "checkbox" },
      { name: "tags", type: "multiselect" },
    ],
    ...over,
  };
}

// Submission runs off the button click: hosts sandbox the widget without
// allow-forms, so a native form submit never fires.
function submit(root: HTMLElement): void {
  root.querySelector<HTMLElement>("[data-gadget-submit]")!.click();
}

describe("form behavior", () => {
  let host: FakeHost;
  let bridge: Bridge;

  beforeEach(async () => {
    host = new FakeHost();
    bridge = new Bridge({ timeoutMs: 500 });
    // Mirrors boot(): initialize runs before user clicks.
    await bridge.initialize();
    host.requests.length = 0;
  });

  afterEach(() => {
    bridge.dispose();
    host.dispose();
    document.body.innerHTML = "";
  });

  it("blocks submit on invalid fields and shows the custom message", async () => {
    const root = shell();
    mountForm({ root, config: config(), initialData: null, bridge });
    submit(root); // name is required and empty
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(0);
    const slot = root.querySelector<HTMLElement>('[data-gadget-error-for="name"]')!;
    expect(slot.hidden).toBe(false);
    expect(slot.textContent).toBe("Name is required.");
    expect(root.querySelector("#f-name")?.getAttribute("aria-invalid")).toBe("true");
  });

  it("submits coerced values merged over static args", async () => {
    const root = shell();
    mountForm({
      root,
      config: config({
        submit: { tool: "save_user", staticArgs: { org: "acme" }, successMessage: "Saved!" },
      }),
      initialData: null,
      bridge,
    });
    (root.querySelector("#f-name") as HTMLInputElement).value = "Ada";
    (root.querySelector("#f-age") as HTMLInputElement).value = "36";
    (root.querySelector("#f-active") as HTMLInputElement).checked = true;
    const tags = root.querySelector("#f-tags") as HTMLSelectElement;
    tags.options[1]!.selected = true;

    submit(root);
    await flush();

    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({
      name: "save_user",
      arguments: { org: "acme", name: "Ada", age: 36, active: true, tags: ["b"] },
    });
    const status = root.querySelector<HTMLElement>("[data-gadget-status]")!;
    expect(status.textContent).toBe("Saved!");
    expect(status.className).toContain("gadget-status--success");
  });

  it("omits empty number fields from the payload", async () => {
    const root = shell();
    mountForm({ root, config: config(), initialData: null, bridge });
    (root.querySelector("#f-name") as HTMLInputElement).value = "Ada";
    submit(root);
    await flush();
    const args = (host.received(M.toolsCall)[0]!.params as { arguments: Record<string, unknown> }).arguments;
    expect("age" in args).toBe(false);
  });

  it("maps server-side field errors inline", async () => {
    const root = shell();
    host.onToolCall = () => ({
      structuredContent: { errors: { name: "Already taken." } },
    });
    mountForm({ root, config: config(), initialData: null, bridge });
    (root.querySelector("#f-name") as HTMLInputElement).value = "Ada";
    submit(root);
    await flush();
    const slot = root.querySelector<HTMLElement>('[data-gadget-error-for="name"]')!;
    expect(slot.hidden).toBe(false);
    expect(slot.textContent).toBe("Already taken.");
    const status = root.querySelector<HTMLElement>("[data-gadget-status]")!;
    expect(status.className).toContain("gadget-status--error");
  });

  it("prefills from the baked initial data and from tool results", async () => {
    const root = shell();
    mountForm({
      root,
      config: config(),
      initialData: { values: { name: "Ada", age: 36, active: true, tags: ["a", "b"] } },
      bridge,
    });
    expect((root.querySelector("#f-name") as HTMLInputElement).value).toBe("Ada");
    expect((root.querySelector("#f-active") as HTMLInputElement).checked).toBe(true);
    const tags = root.querySelector("#f-tags") as HTMLSelectElement;
    expect([...tags.selectedOptions].map((o) => o.value)).toEqual(["a", "b"]);

    host.pushToolResult({ structuredContent: { values: { name: "Grace", active: false } } });
    await flush();
    expect((root.querySelector("#f-name") as HTMLInputElement).value).toBe("Grace");
    expect((root.querySelector("#f-active") as HTMLInputElement).checked).toBe(false);
  });

  it("cancel resets values and errors", async () => {
    const root = shell();
    mountForm({ root, config: config(), initialData: null, bridge });
    const name = root.querySelector("#f-name") as HTMLInputElement;
    submit(root); // empty required name -> validation error
    await flush();
    expect(root.querySelector<HTMLElement>('[data-gadget-error-for="name"]')!.hidden).toBe(false);

    name.value = "Ada";
    root.querySelector<HTMLElement>("[data-gadget-cancel]")!.click();
    expect(root.querySelector<HTMLElement>('[data-gadget-error-for="name"]')!.hidden).toBe(true);
    expect(root.querySelector<HTMLElement>("[data-gadget-status]")!.hidden).toBe(true);
    expect(name.value).toBe(""); // form.reset()
  });

  it("disables controls while submitting", async () => {
    const root = shell();
    let release: (v: { structuredContent: Record<string, unknown> }) => void = () => {};
    host.onToolCall = () =>
      new Promise((resolve) => {
        release = resolve;
      });
    mountForm({ root, config: config(), initialData: null, bridge });
    (root.querySelector("#f-name") as HTMLInputElement).value = "Ada";
    submit(root);
    await flush();
    expect((root.querySelector("#f-name") as HTMLInputElement).disabled).toBe(true);
    expect(root.querySelector<HTMLElement>("[data-gadget-status]")!.className).toContain("loading");

    release({ structuredContent: {} });
    await flush();
    expect((root.querySelector("#f-name") as HTMLInputElement).disabled).toBe(false);
  });

  it("hydrates prefill from loadTool on mount", async () => {
    const root = shell();
    host.onToolCall = (name) =>
      name === "get_user"
        ? { structuredContent: { values: { name: "Grace", age: 42 } } }
        : { structuredContent: {} };
    mountForm({
      root,
      config: config({ loadTool: "get_user" }),
      initialData: null,
      bridge,
      ready: Promise.resolve(true),
    });
    await flush();

    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({ name: "get_user", arguments: {} });
    expect((root.querySelector("#f-name") as HTMLInputElement).value).toBe("Grace");
    expect((root.querySelector("#f-age") as HTMLInputElement).value).toBe("42");
  });

  it("does not hydrate when no host answered the handshake (ready=false)", async () => {
    const root = shell();
    mountForm({
      root,
      config: config({ loadTool: "get_user" }),
      initialData: null,
      bridge,
      ready: Promise.resolve(false),
    });
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(0);
  });
});
