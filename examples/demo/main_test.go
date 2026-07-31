package main

import (
	"strings"
	"testing"

	"github.com/techthos/gomukit"
)

// TestWidgetsRender renders every widget the demo registers. newServer would
// log.Fatal on an invalid one, which is a poor way to find out; this fails
// with the reason instead.
func TestWidgetsRender(t *testing.T) {
	widgets := map[string]gomukit.Widget{
		"table":   usersTable(),
		"cards":   usersCards(),
		"form":    userForm(),
		"profile": profileForm(),
		"confirm": deleteConfirm(),
		"picker":  followUpPicker(),
		"menu":    demoMenu(),
	}
	for name, w := range widgets {
		if _, err := w.Document(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestProfileFormLayout pins the grouped form's structure: the layout is the
// point of that widget, and a field silently leaving its group would still
// render a perfectly valid form.
func TestProfileFormLayout(t *testing.T) {
	doc, err := profileForm().Document()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<form class="gomu-form gomu-form--cols-2"`,
		`<h3 class="gomu-fieldset-title" id="gomu-fs-1">Contact</h3>`,
		`<h3 class="gomu-fieldset-title" id="gomu-fs-2">Subscription</h3>`,
		`gomu-fieldset gomu-fieldset--boxed`,
		`<h3 class="gomu-fieldset-title" id="gomu-fs-3">Internal</h3>`,
		// The contract period takes the whole row of its group.
		`gomu-field--daterange gomu-span-2`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("profile form missing %q", want)
		}
	}
}

// TestProfileFormFieldsMatchSaveArgs keeps the form and its submit tool
// honest: every field name (plus the range's end) is an argument saveProfileIn
// declares, so nothing the reader fills in is dropped on the way to the store.
func TestProfileFormFieldsMatchSaveArgs(t *testing.T) {
	// The struct is local to newServer; its json tags are these.
	args := map[string]bool{}
	for _, n := range []string{
		"id", "company", "name", "phone", "email", "plan", "seats",
		"status", "balance", "startsOn", "endsOn", "notes", "announce",
	} {
		args[n] = true
	}
	f := profileForm()
	var names []string
	for _, fd := range f.Fields {
		names = append(names, fd.Name)
	}
	for _, fs := range f.FieldSets {
		for _, fd := range fs.Fields {
			names = append(names, fd.Name)
			if fd.Type == gomukit.FDateRange {
				names = append(names, fd.EndName)
			}
		}
	}
	for _, n := range names {
		if !args[n] {
			t.Errorf("field %q has no argument in save_profile", n)
		}
		delete(args, n)
	}
	for n := range args {
		t.Errorf("save_profile argument %q has no field", n)
	}
}
