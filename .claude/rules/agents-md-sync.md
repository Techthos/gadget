---
paths:
  - "*.go"
  - "theme/**/*.go"
  - "uispec/**/*.go"
  - "gosdk/**/*.go"
  - "ui/src/**"
---

# Keep AGENTS.md in sync with the public API

`AGENTS.md` at the repo root is the complete LLM-facing reference for this library's public API. Any public API change MUST be reflected in `AGENTS.md` in the same change set — never defer it to a follow-up.

## What counts as a public API change

- Adding, removing, or renaming any exported identifier (type, interface, function, method, constant, variable) in the root package `gadget` (`widget.go`, `table.go`, `form.go`, `action.go`, `rows.go`) or in `theme/`, `uispec/`, `gosdk/`.
- Adding, removing, or changing exported struct fields, or changing their meaning or behavior.
- Changing a documented default value (e.g. `RowsKey` `"rows"`, `RowID` `"id"`, `PrefillKey` `"values"`, `ErrorsKey` `"errors"`).
- Changing the validation rules enforced by `Validate()` / `Document()`.
- Changing the runtime structuredContent data contract: the `"rows"` / `"values"` / `"errors"` keys, or the shapes and types of submitted form values. This contract surfaces through `ui/src`, so such changes count even though `ui/` itself is not public Go API.

Not covered: `internal/...` packages and `ui/` implementation details that do not alter the data contract. This rule is about `AGENTS.md` only; `CLAUDE.md` is contributor guidance with its own upkeep.

## What to do

When making such a change, in the same change set:

1. Update the corresponding `AGENTS.md` section(s) — locate by heading:
   - Root package `gadget` (Widget interface, shared types, Table, Column, Form, Field, Action, RowsOf, validation rules) → "Package `gadget` — full API reference"
   - structuredContent contract → "The runtime data contract (structuredContent keys)"
   - `gosdk` → "Package `gosdk`"; `theme` → "Package `theme`"; `uispec` → "Package `uispec`"
2. For removed APIs: delete their documentation everywhere it appears, including examples, cross-references, and constraint lists.
3. Keep documented defaults and validation rules exactly accurate to the code — if `Validate()` gains or loses a rule, the "Validation rules" section must match.
4. For new exported APIs: document signature, fields, defaults, and behavior at the same level of detail as the surrounding entries.

If a change touches these files but alters no exported surface, default, validation rule, or the data contract, `AGENTS.md` needs no update.
