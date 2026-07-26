package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gadget"
	"github.com/techthos/gadget/gosdk"
)

// The scenario is one small application, Acme Dispatch, wired to real state:
// customers and the orders waiting to leave the warehouse. Every widget it
// registers reads and writes the same store, so an action taken in a table
// shows up in the card list, the form and the confirmation.
//
// It is the half of this server that answers "what does gadget look like in
// an app"; the gallery half answers "what can gadget render".

// --- widgets ---

func scenarioMenu(withGallery bool) *gadget.Menu {
	items := []gadget.MenuItem{
		{Tool: "list_customers", Label: "Customers", IconSVG: iconTable,
			Prompt:      "Show me the customer directory",
			Description: "Sortable, filterable directory with row and bulk actions.",
			Badge:       "read", BadgeVariant: gadget.BadgeInfo},
		{Tool: "browse_customers", Label: "Customer cards", IconSVG: iconCards,
			Prompt:      "Browse the customers as cards",
			Description: "The same accounts as a swipeable card strip.",
			Badge:       "read", BadgeVariant: gadget.BadgeInfo},
		{Tool: "list_orders", Label: "Orders", IconSVG: iconBox,
			Prompt:      "Show me the orders waiting on a dispatch decision",
			Description: "Parcels waiting on a dispatch decision.",
			Badge:       "read", BadgeVariant: gadget.BadgeInfo},
		{Tool: "show_customer", Label: "Account detail", IconSVG: iconCard,
			Prompt:      "Show me Ada Lovelace's account in detail",
			Description: "One record as a card, with the full detail list.",
			Badge:       "read", BadgeVariant: gadget.BadgeInfo},
		{Tool: "new_customer", Label: "New customer", IconSVG: iconPlus,
			Prompt:      "Start a new customer record",
			Description: "A form with every field type and server-side validation.",
			Badge:       "write", BadgeVariant: gadget.BadgeWarning},
		{Tool: "edit_customer", Label: "Edit Ada", IconSVG: iconPencil,
			Prompt:      "Open the edit form for Ada Lovelace",
			Description: "Open the edit form prefilled from the store.",
			Badge:       "write", BadgeVariant: gadget.BadgeWarning},
		{Tool: "choose_shipping", Label: "Ship ORD-4471", IconSVG: iconTruck,
			Prompt:      "How should order ORD-4471 ship?",
			Description: "Pick a carrier from options priced at call time.",
			Badge:       "write", BadgeVariant: gadget.BadgeWarning},
		{Tool: "confirm_delete_customer", Label: "Delete Alan", IconSVG: iconTrash,
			Prompt:      "Delete Alan Turing",
			Description: "A confirmation that spells out what the deletion costs.",
			Badge:       "danger", BadgeVariant: gadget.BadgeDanger},
		{Tool: "reset_demo", Label: "Reset data", IconSVG: iconList,
			Prompt:      "Reset the demo data",
			Description: "Restore the seed customers and orders."},
	}
	if withGallery {
		items = append(items, gadget.MenuItem{
			Tool: "preview_index", Label: "Widget gallery", IconSVG: iconPalette,
			Prompt:      "Show me the widget gallery",
			Description: "Every widget variant this library renders, one tool each.",
			Badge:       "preview", BadgeVariant: gadget.BadgeNeutral,
		})
	}
	return &gadget.Menu{
		URI:   "ui://preview/menu",
		Title: "Acme Dispatch",
		Intro: "Pick where to start. Each tile posts its request to the chat, and the model opens the widget that answers it.",
		Items: items,
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

// customerRowActions is the per-row menu: a link out, two tools that mutate,
// and one that opens another widget.
func customerRowActions() gadget.Column {
	return gadget.ActionsColumn(
		gadget.Action{Label: "Open console", Kind: gadget.ActionLink, HrefKey: "website"},
		gadget.Action{Label: "Edit", Tool: "edit_customer",
			Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
		gadget.Action{Label: "Send invite", Tool: "invite_customer",
			Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
		gadget.Action{Label: "Delete", Tool: "delete_customer", Variant: gadget.VariantDanger,
			Confirm: "Delete this customer?",
			Args:    map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
	)
}

func customerBulk() *gadget.SelectionConfig {
	return &gadget.SelectionConfig{Bulk: []gadget.Action{
		{Label: "Archive", Tool: "archive_customers",
			Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")}},
		{Label: "Reactivate", Tool: "activate_customers",
			Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")}},
		{Label: "Delete", Tool: "delete_customers", Variant: gadget.VariantDanger,
			Confirm: "Delete the selected customers?",
			Args:    map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")}},
	}}
}

// customersTable hydrates itself through LoadTool: a widget the host reopens
// from cache asks the server for current rows rather than showing the
// snapshot frozen when the document was rendered.
func customersTable() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://preview/customers",
		Title: "Customers",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Text("company", "Company"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Number("seats", "Seats", "int"),
			gadget.Number("utilization", "Usage", "percent"),
			gadget.Date("createdAt", "Customer since", "date"),
			customerStatusBadge(),
			customerRowActions(),
		},
		PageSize:    5,
		PageSizes:   []int{5, 10, 25},
		DefaultSort: &gadget.SortSpec{Key: "name"},
		Filterable:  true,
		Selection:   customerBulk(),
		Empty:       gadget.EmptyState{Title: "No customers", Body: "Create one from the menu, or call reset_demo."},
		LoadTool:    "list_customers",
		Brand:       appBrand(),
		Theme:       appTheme(),
	}
}

func customerCardTemplate() gadget.CardTemplate {
	return gadget.CardTemplate{
		Header: gadget.CardHeader{
			TitleKey:       "name",
			DescriptionKey: "company",
			Badge:          customerStatusBadge(),
		},
		Content: gadget.CardContent{
			Items: gadget.Descriptions{Items: []gadget.DescriptionItem{
				{Label: "Balance", Key: "balance", Type: gadget.ColNumber, Format: "currency:EUR"},
				{Label: "Seats", Key: "seats", Type: gadget.ColNumber, Format: "int"},
				{Label: "Renews", Key: "renewsAt", Type: gadget.ColDate, Format: "relative"},
				{Label: "Console", Type: gadget.ColLink, Link: &gadget.LinkSpec{HrefKey: "website", Text: "Open console"}},
			}},
		},
		Footer: gadget.CardFooter{Actions: []gadget.Action{
			{Label: "Edit", Tool: "edit_customer",
				Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
			{Label: "Delete", Tool: "delete_customer", Variant: gadget.VariantDanger,
				Confirm: "Delete this customer?",
				Args:    map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
		}},
	}
}

func customersCards() *gadget.CardList {
	return &gadget.CardList{
		URI:         "ui://preview/customer-cards",
		Title:       "Customers",
		Template:    customerCardTemplate(),
		PageSize:    3,
		PageSizes:   []int{3, 6, 12},
		DefaultSort: &gadget.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   customerBulk(),
		Empty:       gadget.EmptyState{Title: "No customers", Body: "Create one from the menu, or call reset_demo."},
		LoadTool:    "list_customers",
		Brand:       appBrand(),
		Theme:       appTheme(),
	}
}

// customerCard is the account view: the header carries prose and a link
// button instead of a badge, and the body is the full typed detail list.
func customerCard() *gadget.Card {
	return &gadget.Card{
		URI:   "ui://preview/customer",
		Title: "Account",
		Template: gadget.CardTemplate{
			Header: gadget.CardHeader{
				TitleKey:       "name",
				DescriptionKey: "company",
				Action: &gadget.Action{
					Label: "Open console", Kind: gadget.ActionLink, HrefKey: "website",
				},
			},
			Content: gadget.CardContent{
				TextKey: "notes",
				Items:   customerDetails(),
			},
			Footer: gadget.CardFooter{
				Text: "Usage figures update hourly.",
				Actions: []gadget.Action{
					{Label: "Edit", Tool: "edit_customer",
						Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
					{Label: "Delete", Tool: "confirm_delete_customer", Variant: gadget.VariantDanger,
						Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
				},
			},
		},
		Empty: gadget.EmptyState{Title: "No account", Body: "Call show_customer with an id."},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

func customerForm() *gadget.Form {
	return &gadget.Form{
		URI:   "ui://preview/customer-form",
		Title: "Edit customer",
		Fields: []gadget.Field{
			{Name: "id", Type: gadget.FHidden},
			{Name: "name", Label: "Name", Required: true, Description: "Shown everywhere the account appears."},
			{Name: "email", Label: "Email", Required: true,
				Validation: &gadget.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "company", Label: "Company"},
			{Name: "plan", Label: "Plan", Type: gadget.FSelect, Required: true,
				Options: []gadget.Option{gadget.Opt("starter"), gadget.Opt("team"), gadget.Opt("enterprise")}},
			{Name: "status", Label: "Status", Type: gadget.FSelect, Required: true,
				Options: []gadget.Option{gadget.Opt("active"), gadget.Opt("invited"), gadget.Opt("archived")}},
			{Name: "seats", Label: "Seats", Type: gadget.FNumber,
				Validation: &gadget.Validation{Min: ptr(1.0), Max: ptr(500.0), Step: ptr(1.0)}},
			{Name: "renewsAt", Label: "Renews on", Type: gadget.FDate},
			{Name: "notes", Label: "Notes", Type: gadget.FTextarea, Rows: 3},
			{Name: "notify", Label: "Send product notifications", Type: gadget.FCheckbox},
		},
		Submit: gadget.SubmitSpec{Tool: "save_customer", Label: "Save", SuccessMessage: "Customer saved."},
		Cancel: &gadget.CancelSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// newCustomerForm exercises every field type the library renders, including
// the ones the edit form has no use for.
func newCustomerForm() *gadget.Form {
	return &gadget.Form{
		URI:   "ui://preview/customer-new",
		Title: "New customer",
		Fields: []gadget.Field{
			{Name: "account", Label: "Workspace", Type: gadget.FReadonly, Default: "acme-eu"},
			{Name: "name", Label: "Name", Required: true, Placeholder: "Ada Lovelace"},
			{Name: "email", Label: "Email", Required: true, Placeholder: "ada@example.com",
				Validation: &gadget.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "company", Label: "Company", Placeholder: "Analytical Engines"},
			{Name: "website", Label: "Console URL", Placeholder: "https://example.com/ada"},
			{Name: "plan", Label: "Plan", Type: gadget.FSelect, Default: "team",
				Options: []gadget.Option{gadget.Opt("starter"), gadget.Opt("team"), gadget.Opt("enterprise")}},
			{Name: "scopes", Label: "Scopes", Type: gadget.FMultiSelect, Default: []string{"read"},
				Options: []gadget.Option{gadget.Opt("read"), gadget.Opt("write"), gadget.Opt("billing")}},
			{Name: "seats", Label: "Seats", Type: gadget.FNumber, Default: "3",
				Validation: &gadget.Validation{Min: ptr(1.0), Max: ptr(500.0), Step: ptr(1.0)}},
			{Name: "renewsAt", Label: "Renews on", Type: gadget.FDate, Default: "2027-08-01"},
			{Name: "digestAt", Label: "Daily digest", Type: gadget.FTime, Default: "09:00"},
			{Name: "notes", Label: "Notes", Type: gadget.FTextarea, Rows: 3,
				Placeholder: "Anything the team should know"},
			{Name: "notify", Label: "Send product notifications", Type: gadget.FCheckbox, Default: true},
		},
		Submit: gadget.SubmitSpec{Tool: "create_customer", Label: "Create customer",
			StaticArgs: map[string]any{"source": "preview-menu"}, SuccessMessage: "Customer created."},
		Cancel: &gadget.CancelSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// deleteConfirm authors the question; the record and what removing it costs
// arrive from confirm_delete_customer, computed from the store at call time.
func deleteConfirm() *gadget.Confirm {
	return &gadget.Confirm{
		URI:         "ui://preview/confirm-delete",
		Title:       "Delete customer",
		Prompt:      "Delete this customer?",
		Body:        "The account and everything attached to it is removed for good.",
		Severity:    gadget.BadgeDanger,
		Details:     customerDetails(),
		Acknowledge: "I understand this cannot be undone.",
		Accept: gadget.AcceptSpec{
			Tool:           "apply_delete_customer",
			Label:          "Delete customer",
			Args:           map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
			SuccessMessage: "Customer deleted.",
		},
		Reject: &gadget.RejectSpec{
			Label:   "Keep customer",
			Tool:    "keep_customer",
			Args:    map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
			Message: "Nothing was deleted.",
		},
		Brand: appBrand(),
		Theme: appTheme(),
	}
}

func ordersTable() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://preview/orders",
		Title: "Orders",
		Columns: []gadget.Column{
			gadget.Text("reference", "Reference"),
			gadget.Text("customer", "Customer"),
			gadget.Number("items", "Items", "int"),
			gadget.Number("weightKg", "Weight (kg)", "decimal:1"),
			gadget.Number("total", "Total", "currency:EUR"),
			gadget.Date("placedAt", "Placed", "relative"),
			orderStatusBadge(),
			gadget.Link("tracking", "Tracking"),
			gadget.ActionsColumn(
				gadget.Action{Label: "Choose shipping", Tool: "choose_shipping", Variant: gadget.VariantPrimary,
					Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
				gadget.Action{Label: "Add extras", Tool: "choose_extras",
					Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
				gadget.Action{Label: "Set delivery date", Tool: "schedule_delivery",
					Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
			),
		},
		PageSize:    5,
		DefaultSort: &gadget.SortSpec{Key: "placedAt", Desc: true},
		Filterable:  true,
		Empty:       gadget.EmptyState{Title: "No orders", Body: "Call reset_demo to restore the seed data."},
		LoadTool:    "list_orders",
		Brand:       appBrand(),
		Theme:       appTheme(),
	}
}

// shippingChoice authors the question and nothing else: what is on offer, and
// what it costs, comes from choose_shipping.
func shippingChoice() *gadget.Choice {
	return &gadget.Choice{
		URI:    "ui://preview/shipping",
		Title:  "Shipping",
		Prompt: "How should this order ship?",
		Body:   "The parcel is packed and leaves the warehouse today either way.",
		Details: gadget.Descriptions{Items: []gadget.DescriptionItem{
			{Label: "Order", Key: "reference"},
			{Label: "Customer", Key: "customer"},
			{Label: "Weight", Key: "weightKg", Type: gadget.ColNumber, Format: "decimal:1"},
			{Label: "Value", Key: "total", Type: gadget.ColNumber, Format: "currency:EUR"},
			{Label: "Destination", Text: "Berlin, DE"},
		}},
		Submit: gadget.ChoiceSubmit{
			Tool:           "ship_order",
			Label:          "Ship it",
			ValueArg:       "method",
			Args:           map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
			SuccessMessage: "On its way.",
		},
		Cancel: &gadget.RejectSpec{Label: "Decide later", Message: "Nothing was shipped."},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// deliveryPicker authors the question; the window it may be answered in comes
// from schedule_delivery, computed against the day the tool runs.
func deliveryPicker() *gadget.DatePicker {
	return &gadget.DatePicker{
		URI:    "ui://preview/delivery",
		Title:  "Delivery",
		Prompt: "When should this order arrive?",
		Body:   "The depot needs one working day's notice, and dispatches nothing on stocktaking days.",
		Calendar: &gadget.Calendar{
			Presets: []gadget.DatePreset{
				{Label: "Tomorrow", Span: gadget.SpanTomorrow},
				{Label: "In a week", Span: gadget.SpanNext7Days},
			},
		},
		Details: gadget.Descriptions{Items: []gadget.DescriptionItem{
			{Label: "Order", Key: "reference"},
			{Label: "Customer", Key: "customer"},
			{Label: "Items", Key: "items", Type: gadget.ColNumber, Format: "int"},
		}},
		Submit: gadget.DateSubmit{
			Tool:           "set_delivery_date",
			Label:          "Book it",
			Args:           map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
			SuccessMessage: "Delivery booked.",
		},
		Cancel: &gadget.RejectSpec{Label: "Decide later", Message: "Nothing was booked."},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// extrasChoice is the authored counterpart: a fixed catalog, several picks,
// bounded above and below.
func extrasChoice() *gadget.Choice {
	return &gadget.Choice{
		URI:      "ui://preview/extras",
		Title:    "Add-ons",
		Prompt:   "Which extras should this shipment carry?",
		Body:     "Choose one to three; they are billed with the shipping cost.",
		Layout:   gadget.ChoiceSplit,
		Multiple: true,
		Min:      1,
		Max:      3,
		Details: gadget.Descriptions{Items: []gadget.DescriptionItem{
			{Label: "Order", Key: "reference"},
			{Label: "Customer", Key: "customer"},
		}},
		Options: []gadget.ChoiceOption{
			{Value: "insurance", Label: "Extra insurance", Summary: "up to EUR 5,000",
				Body:    "Covers the declared value against loss and damage in transit.",
				Bullets: []string{"Claims within 30 days", "Proof of value required"},
				Default: true},
			{Value: "signature", Label: "Signature on delivery", Summary: "hand to the recipient",
				Body: "The courier hands the parcel over in person and records the signature."},
			{Value: "saturday", Label: "Saturday delivery", Summary: "weekend slot",
				Body:         "Delivered on Saturday morning instead of the next business day.",
				Badge:        "surcharge",
				BadgeVariant: gadget.BadgeWarning},
			{Value: "carbon", Label: "Carbon offset", Summary: "adds EUR 0.40",
				Body: "Buys certified offsets for the leg between the depot and the door."},
		},
		Submit: gadget.ChoiceSubmit{
			Tool:           "add_order_extras",
			Label:          "Add extras",
			ValueArg:       "extras",
			Args:           map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
			SuccessMessage: "Extras added.",
		},
		Cancel: &gadget.RejectSpec{},
		Brand:  appBrand(),
		Theme:  appTheme(),
	}
}

// --- tools ---

// registerScenario wires the app: model-visible tools that open a widget, and
// app-only tools the widgets call back into. Anything a widget fires and the
// model has no business calling directly is marked AppOnly.
func registerScenario(s *mcp.Server, data *store, withGallery bool) {
	menu := scenarioMenu(withGallery)
	table := customersTable()
	cards := customersCards()
	card := customerCard()
	form := customerForm()
	create := newCustomerForm()
	confirm := deleteConfirm()
	orders := ordersTable()
	shipping := shippingChoice()
	extras := extrasChoice()
	delivery := deliveryPicker()

	must(gosdk.AddWidgetToolFor(s, menu,
		&mcp.Tool{Name: "main_menu", Description: "Show the Acme Dispatch app menu."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, noOut, error) {
			return textResult("Showing the app menu."), noOut{}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, table,
		&mcp.Tool{Name: "list_customers", Description: "List all customers in an interactive table."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, rowsOut, error) {
			return nil, rowsOut{Rows: data.customerRows()}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, cards,
		&mcp.Tool{Name: "browse_customers", Description: "Browse customers as a strip of cards."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, rowsOut, error) {
			return nil, rowsOut{Rows: data.customerRows()}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, card,
		&mcp.Tool{Name: "show_customer", Description: "Show one customer account in detail."},
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, rowsOut, error) {
			row, ok := data.customerRow(in.ID)
			if !ok {
				return textResult(fmt.Sprintf("Customer %d not found.", in.ID)), rowsOut{Rows: []map[string]any{}}, nil
			}
			return nil, rowsOut{Rows: []map[string]any{row}}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, orders,
		&mcp.Tool{Name: "list_orders", Description: "List orders waiting on a dispatch decision."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, rowsOut, error) {
			return nil, rowsOut{Rows: data.orderRows()}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, form,
		&mcp.Tool{Name: "edit_customer", Description: "Open an edit form for the given customer id."},
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, valuesOut, error) {
			row, ok := data.customerRow(in.ID)
			if !ok {
				return textResult(fmt.Sprintf("Customer %d not found.", in.ID)), valuesOut{}, nil
			}
			return nil, valuesOut{Values: editValues(row)}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, create,
		&mcp.Tool{Name: "new_customer", Description: "Open the create-customer form."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, valuesOut, error) {
			return nil, valuesOut{Values: map[string]any{}}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, confirm,
		&mcp.Tool{Name: "confirm_delete_customer",
			Description: "Ask the user to confirm deleting a customer, listing what the deletion costs."},
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, confirmOut, error) {
			row, ok := data.customerRow(in.ID)
			if !ok {
				return textResult(fmt.Sprintf("Customer %d not found.", in.ID)), confirmOut{}, nil
			}
			return nil, confirmOut{Rows: []map[string]any{row}, Effects: deleteEffects(data, in.ID, row)}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, shipping,
		&mcp.Tool{Name: "choose_shipping", Description: "Offer the shipping methods available for an order."},
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, choiceOut, error) {
			row, options, ok := data.shippingFor(in.ID)
			if !ok {
				return textResult(fmt.Sprintf("Order %d not found.", in.ID)), choiceOut{}, nil
			}
			return nil, choiceOut{Rows: []map[string]any{row}, Options: options}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, extras,
		&mcp.Tool{Name: "choose_extras", Description: "Offer the add-ons an order can carry."},
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, rowsOut, error) {
			row, ok := data.orderRow(in.ID)
			if !ok {
				return textResult(fmt.Sprintf("Order %d not found.", in.ID)), rowsOut{}, nil
			}
			return nil, rowsOut{Rows: []map[string]any{row}}, nil
		}))

	must(gosdk.AddWidgetToolFor(s, delivery,
		&mcp.Tool{Name: "schedule_delivery", Description: "Ask the user which day an order should arrive on."},
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, dateOut, error) {
			row, window, ok := data.deliveryFor(in.ID, time.Now())
			if !ok {
				return textResult(fmt.Sprintf("Order %d not found.", in.ID)), dateOut{}, nil
			}
			return nil, dateOut{Rows: []map[string]any{row}, Value: window}, nil
		}))

	// The one tool with no widget of its own: it answers with text, the way
	// any ordinary MCP tool does.
	mcp.AddTool(s, &mcp.Tool{Name: "reset_demo", Description: "Restore the seed customers and orders."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, noOut, error) {
			data.reset()
			return textResult("Preview data restored."), noOut{}, nil
		})

	registerScenarioActions(s, data, table, cards, form, create, confirm, shipping, extras, delivery)
}

// registerScenarioActions installs the app-only half: the tools widgets call
// and the model does not.
func registerScenarioActions(s *mcp.Server, data *store,
	table *gadget.Table, cards *gadget.CardList, form, create *gadget.Form,
	confirm *gadget.Confirm, shipping, extras *gadget.Choice, delivery *gadget.DatePicker,
) {
	// Row action: delete, answering with the rows that remain so the table
	// repaints from the server's truth rather than guessing locally.
	deleteTool := &mcp.Tool{Name: "delete_customer", Description: "Delete a customer by id."}
	gosdk.AppOnly(deleteTool, table)
	must(gosdk.AddWidgetToolFor(s, table, deleteTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, rowsOut, error) {
			if !data.deleteCustomer(in.ID) {
				return textResult("No such customer."), rowsOut{Rows: data.customerRows()}, nil
			}
			return textResult("Customer deleted."), rowsOut{Rows: data.customerRows()}, nil
		}))

	inviteTool := &mcp.Tool{Name: "invite_customer", Description: "Re-send the invitation for a customer."}
	gosdk.AppOnly(inviteTool, table)
	must(gosdk.AddWidgetToolFor(s, table, inviteTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, rowsOut, error) {
			data.setStatus([]int{in.ID}, "invited")
			return textResult("Invitation sent."), rowsOut{Rows: data.customerRows()}, nil
		}))

	// Bulk actions: the same rows contract, over the selection.
	bulk := []struct {
		name, desc, status, done string
	}{
		{"archive_customers", "Archive the given customers.", "archived", "Archived %d customers."},
		{"activate_customers", "Reactivate the given customers.", "active", "Reactivated %d customers."},
	}
	for _, b := range bulk {
		t := &mcp.Tool{Name: b.name, Description: b.desc}
		gosdk.AppOnly(t, table)
		status, done := b.status, b.done
		must(gosdk.AddWidgetToolFor(s, table, t,
			func(_ context.Context, _ *mcp.CallToolRequest, in idsArg) (*mcp.CallToolResult, rowsOut, error) {
				n := data.setStatus(in.IDs, status)
				return textResult(fmt.Sprintf(done, n)), rowsOut{Rows: data.customerRows()}, nil
			}))
	}

	deleteManyTool := &mcp.Tool{Name: "delete_customers", Description: "Delete the given customers."}
	gosdk.AppOnly(deleteManyTool, table)
	must(gosdk.AddWidgetToolFor(s, table, deleteManyTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in idsArg) (*mcp.CallToolResult, rowsOut, error) {
			n := 0
			for _, id := range in.IDs {
				if data.deleteCustomer(id) {
					n++
				}
			}
			return textResult(fmt.Sprintf("Deleted %d customers.", n)), rowsOut{Rows: data.customerRows()}, nil
		}))

	// The card list fires the same tools; registering its resource keeps the
	// widget available to the host even though no tool of its own is bound to
	// it beyond browse_customers.
	must(gosdk.AddWidget(s, cards))

	// Form submit: server-side validation answers with field errors, which the
	// form renders under the offending controls.
	saveTool := &mcp.Tool{Name: "save_customer", Description: "Save an existing customer."}
	gosdk.AppOnly(saveTool, form)
	must(gosdk.AddWidgetToolFor(s, form, saveTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in saveInput) (*mcp.CallToolResult, errorsOut, error) {
			id, _ := strconv.Atoi(in.ID)
			errs, c := data.saveCustomer(in.normalize(id))
			if errs != nil {
				return nil, errorsOut{Errors: errs}, nil
			}
			return textResult(fmt.Sprintf("Saved %s.", c.Name)), errorsOut{}, nil
		}))

	createTool := &mcp.Tool{Name: "create_customer", Description: "Create a customer."}
	gosdk.AppOnly(createTool, create)
	must(gosdk.AddWidgetToolFor(s, create, createTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in createInput) (*mcp.CallToolResult, errorsOut, error) {
			errs, c := data.saveCustomer(in.normalize())
			if errs != nil {
				return nil, errorsOut{Errors: errs}, nil
			}
			return textResult(fmt.Sprintf("Created %s (id %d).", c.Name, c.ID)), errorsOut{}, nil
		}))

	// Confirmation: accept deletes, reject is told about so the server can log
	// the decision rather than only the widget knowing it.
	applyTool := &mcp.Tool{Name: "apply_delete_customer", Description: "Delete a customer after confirmation."}
	gosdk.AppOnly(applyTool, confirm)
	must(gosdk.AddWidgetToolFor(s, confirm, applyTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, noOut, error) {
			if !data.deleteCustomer(in.ID) {
				return textResult("No such customer."), noOut{}, nil
			}
			return textResult("Customer deleted."), noOut{}, nil
		}))

	keepTool := &mcp.Tool{Name: "keep_customer", Description: "Record that a deletion was declined."}
	gosdk.AppOnly(keepTool, confirm)
	must(gosdk.AddWidgetToolFor(s, confirm, keepTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in idArg) (*mcp.CallToolResult, noOut, error) {
			return textResult(fmt.Sprintf("Kept customer %d.", in.ID)), noOut{}, nil
		}))

	// Choice submit: the chosen value arrives under the submit's ValueArg.
	shipTool := &mcp.Tool{Name: "ship_order", Description: "Ship an order with the chosen method."}
	gosdk.AppOnly(shipTool, shipping)
	must(gosdk.AddWidgetToolFor(s, shipping, shipTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in shipInput) (*mcp.CallToolResult, noOut, error) {
			o, ok := data.shipOrder(in.ID, in.Method)
			if !ok {
				return textResult("No such order."), noOut{}, nil
			}
			return textResult(fmt.Sprintf("%s shipped by %s.", o.Reference, in.Method)), noOut{}, nil
		}))

	dateTool := &mcp.Tool{Name: "set_delivery_date", Description: "Record the day an order should arrive on."}
	gosdk.AppOnly(dateTool, delivery)
	must(gosdk.AddWidgetToolFor(s, delivery, dateTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in dateInput) (*mcp.CallToolResult, noOut, error) {
			o, ok := data.setDeliveryDate(in.ID, in.Date)
			if !ok {
				return textResult("No such order."), noOut{}, nil
			}
			return textResult(fmt.Sprintf("%s will arrive on %s.", o.Reference, in.Date)), noOut{}, nil
		}))

	extrasTool := &mcp.Tool{Name: "add_order_extras", Description: "Attach add-ons to an order."}
	gosdk.AppOnly(extrasTool, extras)
	must(gosdk.AddWidgetToolFor(s, extras, extrasTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in extrasInput) (*mcp.CallToolResult, noOut, error) {
			o, ok := data.addExtras(in.ID, in.Extras)
			if !ok {
				return textResult("No such order."), noOut{}, nil
			}
			return textResult(fmt.Sprintf("%s now carries: %s.", o.Reference, strings.Join(in.Extras, ", "))), noOut{}, nil
		}))
}

// --- tool input shapes ---

// saveInput is what the edit form submits. A hidden field arrives as a
// string, and an empty number field is omitted rather than sent as "", so
// every field is omitempty: the SDK marks anything else required, and a
// half-filled form would be rejected before the handler could answer with
// field errors of its own.
type saveInput struct {
	ID       string  `json:"id,omitempty"`
	Name     string  `json:"name,omitempty"`
	Email    string  `json:"email,omitempty"`
	Company  string  `json:"company,omitempty"`
	Plan     string  `json:"plan,omitempty"`
	Status   string  `json:"status,omitempty"`
	Seats    float64 `json:"seats,omitempty"`
	RenewsAt string  `json:"renewsAt,omitempty"`
	Notes    string  `json:"notes,omitempty"`
	Notify   bool    `json:"notify,omitempty"`
}

func (in saveInput) normalize(id int) customerInput {
	return customerInput{
		ID: id, Name: in.Name, Email: in.Email, Company: in.Company,
		Plan: in.Plan, Status: in.Status, Seats: int(in.Seats),
		RenewsAt: in.RenewsAt, Notes: in.Notes, Notify: in.Notify,
	}
}

// createInput is what the create form submits, including the static argument
// the submit spec merges in.
type createInput struct {
	Source   string   `json:"source,omitempty"`
	Account  string   `json:"account,omitempty"`
	Name     string   `json:"name,omitempty"`
	Email    string   `json:"email,omitempty"`
	Company  string   `json:"company,omitempty"`
	Website  string   `json:"website,omitempty"`
	Plan     string   `json:"plan,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
	Seats    float64  `json:"seats,omitempty"`
	RenewsAt string   `json:"renewsAt,omitempty"`
	DigestAt string   `json:"digestAt,omitempty"`
	Notes    string   `json:"notes,omitempty"`
	Notify   bool     `json:"notify,omitempty"`
}

func (in createInput) normalize() customerInput {
	return customerInput{
		Name: in.Name, Email: in.Email, Company: in.Company, Website: in.Website,
		Plan: in.Plan, Status: "invited", Seats: int(in.Seats), RenewsAt: in.RenewsAt,
		Notes: in.Notes, Scopes: in.Scopes, Notify: in.Notify,
	}
}

type shipInput struct {
	ID     int    `json:"id"`
	Method string `json:"method" jsonschema:"the chosen shipping method"`
}

type extrasInput struct {
	ID     int      `json:"id"`
	Extras []string `json:"extras" jsonschema:"the chosen add-ons"`
}

type dateInput struct {
	ID   int    `json:"id"`
	Date string `json:"date" jsonschema:"the delivery day, YYYY-MM-DD"`
}

// --- per-call data the widgets read ---

// editValues turns a stored record into form prefill: a date input wants
// YYYY-MM-DD, not the RFC 3339 timestamp the record carries.
func editValues(row map[string]any) map[string]any {
	values := map[string]any{}
	for k, v := range row {
		values[k] = v
	}
	if ts, ok := row["renewsAt"].(string); ok && len(ts) >= 10 {
		values["renewsAt"] = ts[:10]
	}
	return values
}

// deleteEffects is why the confirmation is worth showing: the consequences are
// counted from current state, not authored once at registration time.
func deleteEffects(data *store, id int, row map[string]any) []map[string]any {
	effects := []map[string]any{
		{"text": "Removes the account", "detail": "Sign-in stops working immediately.", "severity": "danger"},
	}
	if n := data.orderCount(id); n > 0 {
		effects = append(effects, map[string]any{
			"text": "Cancels open orders", "detail": "Parcels already packed are returned to stock.",
			"value": fmt.Sprintf("%d orders", n), "severity": "danger",
		})
	}
	if balance, ok := row["balance"].(float64); ok && balance > 0 {
		effects = append(effects, map[string]any{
			"text": "Writes off the balance", "value": fmt.Sprintf("%.2f EUR", balance), "severity": "warning",
		})
	}
	if seats, ok := row["seats"].(float64); ok && seats > 0 {
		effects = append(effects, map[string]any{
			"text": "Frees the seats", "value": fmt.Sprintf("%.0f seats", seats), "severity": "success",
		})
	}
	if row["status"] == "active" {
		effects = append(effects, map[string]any{
			"text": "Ends an active session", "severity": "warning",
		})
	}
	return effects
}
