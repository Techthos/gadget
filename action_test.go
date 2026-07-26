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
