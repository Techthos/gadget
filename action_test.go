package gomukit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActionConfigPrompt(t *testing.T) {
	a := Action{
		Label:  "Edit",
		Tool:   "edit_user",
		Prompt: "Open the edit form for this user",
		Args:   map[string]ArgSource{"id": FromRow("id")},
	}
	b, err := json.Marshal(a.config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)

	if !strings.Contains(cfg, `"prompt":"Open the edit form for this user"`) {
		t.Errorf("config missing prompt\nfull: %s", cfg)
	}
	// Tool stays: it documents what the action opens even though the model
	// makes the call.
	if !strings.Contains(cfg, `"tool":"edit_user"`) {
		t.Errorf("config should keep the tool name\nfull: %s", cfg)
	}
	// A prompt action never calls the tool from the view, so its args are dead
	// weight and must not reach the island.
	if strings.Contains(cfg, `"args"`) {
		t.Errorf("config should drop args of a prompt action\nfull: %s", cfg)
	}
}

func TestActionConfigWithoutPromptKeepsArgs(t *testing.T) {
	a := Action{Label: "Delete", Tool: "delete_user", Args: map[string]ArgSource{"id": FromRow("id")}}
	b, err := json.Marshal(a.config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	if !strings.Contains(cfg, `"args":{"id":{"row":"id"}}`) {
		t.Errorf("config lost args\nfull: %s", cfg)
	}
	if strings.Contains(cfg, `"prompt"`) {
		t.Errorf("config should carry no prompt key\nfull: %s", cfg)
	}
}

func TestActionConfigVisibleWhen(t *testing.T) {
	cases := []struct {
		name string
		pred RowPredicate
		want string
	}{
		{"equality", RowIs("paused", true), `"visibleWhen":{"equals":true,"key":"paused"}`},
		{"set", RowIn("state", "paused", "failed"), `"visibleWhen":{"in":["paused","failed"],"key":"state"}`},
		// One value is one value however it was written, so RowIn with a single
		// member emits the same equality test as RowIs.
		{"single-member set", RowIn("state", "paused"), `"visibleWhen":{"equals":"paused","key":"state"}`},
		{"complement", RowNot(RowIs("state", "running")), `"visibleWhen":{"equals":"running","key":"state","not":true}`},
		{"double complement", RowNot(RowNot(RowIs("state", "running"))), `"visibleWhen":{"equals":"running","key":"state"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := Action{Label: "Activate", Tool: "schedule_manage", VisibleWhen: tc.pred}
			b, err := json.Marshal(a.config())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), tc.want) {
				t.Errorf("config = %s, want it to contain %s", b, tc.want)
			}
		})
	}
}

// An action without a predicate must serialize exactly as it did before the
// field existed: an absent predicate keeps every existing widget's output
// byte identical.
func TestActionConfigWithoutVisibleWhen(t *testing.T) {
	b, err := json.Marshal(Action{Label: "Edit", Tool: "edit_user"}.config())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "visibleWhen") {
		t.Errorf("config = %s, want no visibleWhen key", b)
	}
	// RowNot has nothing to negate here, so the predicate stays absent.
	b, err = json.Marshal(Action{Label: "Edit", Tool: "edit_user", VisibleWhen: RowNot(RowPredicate{})}.config())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "visibleWhen") {
		t.Errorf("negated zero predicate: config = %s, want no visibleWhen key", b)
	}
}

func TestActionValidateVisibleWhen(t *testing.T) {
	t.Run("a predicate needs a field", func(t *testing.T) {
		a := Action{Label: "Activate", Tool: "run", VisibleWhen: RowIs("", "paused")}
		err := a.validate("ctx")
		if err == nil || !strings.Contains(err.Error(), "a row field is required") {
			t.Errorf("validate = %v, want a missing-field complaint", err)
		}
	})

	t.Run("a set predicate needs a member", func(t *testing.T) {
		a := Action{Label: "Activate", Tool: "run", VisibleWhen: RowIn("state")}
		err := a.validate("ctx")
		if err == nil || !strings.Contains(err.Error(), "at least one value") {
			t.Errorf("validate = %v, want an empty-set complaint", err)
		}
	})

	t.Run("bulk actions reject a predicate", func(t *testing.T) {
		// A bulk action stands over a selection, not on one record, so there is
		// nothing for the predicate to read.
		a := Action{Label: "Archive", Tool: "archive", VisibleWhen: RowIs("state", "paused")}
		err := validateBulkAction("ctx", a)
		if err == nil || !strings.Contains(err.Error(), "only valid on per-record actions") {
			t.Errorf("validateBulkAction = %v, want a bulk complaint", err)
		}
		if err := validateBulkAction("ctx", Action{Label: "Archive", Tool: "archive"}); err != nil {
			t.Errorf("validateBulkAction = %v, want nil for a predicate-free action", err)
		}
	})
}

func TestActionValidatePrompt(t *testing.T) {
	t.Run("tool action accepts a prompt", func(t *testing.T) {
		a := Action{Label: "Edit", Tool: "edit_user", Prompt: "Open the edit form"}
		if err := a.validate("ctx"); err != nil {
			t.Errorf("validate = %v, want nil", err)
		}
	})

	t.Run("link action rejects a prompt", func(t *testing.T) {
		// A link already navigates on its own, so a chat turn would be a second,
		// contradictory way to leave the view.
		a := Action{Label: "Open", Kind: ActionLink, HrefKey: "website", Prompt: "Open the site"}
		err := a.validate("ctx")
		if err == nil {
			t.Fatal("validate = nil, want an error")
		}
		if !strings.Contains(err.Error(), "does not apply to link actions") {
			t.Errorf("validate = %v, want a link/prompt complaint", err)
		}
	})

	t.Run("tool is still required with a prompt", func(t *testing.T) {
		a := Action{Label: "Edit", Prompt: "Open the edit form"}
		if err := a.validate("ctx"); err == nil {
			t.Error("validate = nil, want a missing-tool error")
		}
	})
}
