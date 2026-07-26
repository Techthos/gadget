package main

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/techthos/gomukit"
)

// customer is one account in the scenario app. The json tags are the row
// field names every widget reads, so a record travels from here to a table
// cell, a card, a form prefill or a confirmation summary unchanged.
type customer struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Company     string   `json:"company"`
	Plan        string   `json:"plan"`
	Status      string   `json:"status"`
	Balance     float64  `json:"balance"`
	Seats       int      `json:"seats"`
	Utilization float64  `json:"utilization"`
	CreatedAt   string   `json:"createdAt"`
	RenewsAt    string   `json:"renewsAt"`
	Website     string   `json:"website"`
	Notes       string   `json:"notes"`
	Scopes      []string `json:"scopes"`
	Notify      bool     `json:"notify"`
}

// order is one shipment waiting on a dispatch decision.
type order struct {
	ID         int      `json:"id"`
	Reference  string   `json:"reference"`
	CustomerID int      `json:"customerId"`
	Customer   string   `json:"customer"`
	Status     string   `json:"status"`
	Items      int      `json:"items"`
	WeightKg   float64  `json:"weightKg"`
	Total      float64  `json:"total"`
	PlacedAt   string   `json:"placedAt"`
	Method     string   `json:"method"`
	Extras     []string `json:"extras"`
	Tracking   string   `json:"tracking"`
	// DeliverOn is the day the customer expects the parcel, "YYYY-MM-DD", set
	// by the date picker. Empty until someone picks one.
	DeliverOn string `json:"deliverOn"`
}

// store holds the scenario state. Everything the scenario tools read and
// write lives here, so the widgets show a server that actually changed rather
// than a fixture that pretends to.
type store struct {
	mu         sync.Mutex
	customers  map[int]*customer
	orders     map[int]*order
	nextCustID int
	nextOrdID  int
}

func newStore() *store {
	s := &store{}
	s.reset()
	return s
}

// reset restores the seed dataset. The reset_demo tool calls it so a preview
// session can start over without restarting the process.
func (s *store) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	seedCustomers := []*customer{
		{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com", Company: "Analytical Engines", Plan: "enterprise",
			Status: "active", Balance: 1200.50, Seats: 48, Utilization: 0.71,
			CreatedAt: "2024-11-02T09:00:00Z", RenewsAt: "2027-01-31T00:00:00Z",
			Website: "https://example.com/ada", Notes: "Wrote the first published algorithm; runs the engine team.",
			Scopes: []string{"read", "write"}, Notify: true},
		{ID: 2, Name: "Grace Hopper", Email: "grace@example.com", Company: "Compiler Works", Plan: "team",
			Status: "active", Balance: 815, Seats: 12, Utilization: 0.44,
			CreatedAt: "2025-02-03T10:30:00Z", RenewsAt: "2026-11-30T00:00:00Z",
			Website: "https://example.com/grace", Notes: "Keeps the nightly build honest.",
			Scopes: []string{"read", "write", "billing"}, Notify: true},
		{ID: 3, Name: "Alan Turing", Email: "alan@example.com", Company: "Bletchley Labs", Plan: "starter",
			Status: "invited", Balance: 0, Seats: 3, Utilization: 0,
			CreatedAt: "2026-03-19T14:00:00Z", RenewsAt: "2027-03-19T00:00:00Z",
			Website: "", Notes: "Invited last week; has not signed in yet.",
			Scopes: []string{"read"}, Notify: false},
		{ID: 4, Name: "Katherine Johnson", Email: "katherine@example.com", Company: "Orbital Math", Plan: "team",
			Status: "active", Balance: 233.10, Seats: 9, Utilization: 0.88,
			CreatedAt: "2025-04-01T08:15:00Z", RenewsAt: "2026-10-01T00:00:00Z",
			Website: "https://example.com/katherine", Notes: "Checks every trajectory by hand before it ships.",
			Scopes: []string{"read", "write"}, Notify: true},
		{ID: 5, Name: "Barbara Liskov", Email: "barbara@example.com", Company: "Substitution Ltd", Plan: "enterprise",
			Status: "archived", Balance: 4820.40, Seats: 60, Utilization: 0.12,
			CreatedAt: "2023-06-14T11:45:00Z", RenewsAt: "2026-08-14T00:00:00Z",
			Website: "https://example.com/barbara", Notes: "Contract paused for the summer.",
			Scopes: []string{"read"}, Notify: false},
	}
	s.customers = map[int]*customer{}
	for _, c := range seedCustomers {
		s.customers[c.ID] = c
	}
	s.nextCustID = 6

	seedOrders := []*order{
		{ID: 4471, Reference: "ORD-4471", CustomerID: 1, Customer: "Ada Lovelace", Status: "packed",
			Items: 3, WeightKg: 2.4, Total: 189.90, PlacedAt: "2026-07-24T08:10:00Z",
			Tracking: "https://example.com/track/ORD-4471"},
		{ID: 4472, Reference: "ORD-4472", CustomerID: 2, Customer: "Grace Hopper", Status: "packed",
			Items: 1, WeightKg: 0.6, Total: 42.00, PlacedAt: "2026-07-25T13:35:00Z",
			Tracking: "https://example.com/track/ORD-4472"},
		{ID: 4473, Reference: "ORD-4473", CustomerID: 4, Customer: "Katherine Johnson", Status: "shipped",
			Items: 7, WeightKg: 11.8, Total: 1204.15, PlacedAt: "2026-07-21T16:00:00Z",
			Method: "express", Extras: []string{"insurance"},
			Tracking: "https://example.com/track/ORD-4473"},
		{ID: 4474, Reference: "ORD-4474", CustomerID: 2, Customer: "Grace Hopper", Status: "held",
			Items: 2, WeightKg: 5.1, Total: 310.00, PlacedAt: "2026-07-26T07:05:00Z",
			Tracking: "https://example.com/track/ORD-4474"},
	}
	s.orders = map[int]*order{}
	for _, o := range seedOrders {
		s.orders[o.ID] = o
	}
	s.nextOrdID = 4475
}

// --- reads ---

func (s *store) customerRows() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]*customer, 0, len(s.customers))
	for _, c := range s.customers {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	rows, _ := gomukit.RowsOf(list)
	return rows
}

func (s *store) customerRow(id int) (map[string]any, bool) {
	s.mu.Lock()
	c := s.customers[id]
	s.mu.Unlock()
	if c == nil {
		return nil, false
	}
	return rowOf(c), true
}

func (s *store) orderRows() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]*order, 0, len(s.orders))
	for _, o := range s.orders {
		list = append(list, o)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	rows, _ := gomukit.RowsOf(list)
	return rows
}

func (s *store) orderRow(id int) (map[string]any, bool) {
	s.mu.Lock()
	o := s.orders[id]
	s.mu.Unlock()
	if o == nil {
		return nil, false
	}
	return rowOf(o), true
}

// orderCount reports how many orders reference a customer, which is what the
// delete confirmation weighs.
func (s *store) orderCount(id int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, o := range s.orders {
		if o.CustomerID == id {
			n++
		}
	}
	return n
}

// deliveryFor returns the order as a row plus the window it can be delivered
// in: from tomorrow to four weeks out, with the depot's closed days blocked.
// Both are computed at call time, which is the point — a window authored at
// registration time would be stale by the time anyone read it.
func (s *store) deliveryFor(id int, today time.Time) (map[string]any, map[string]any, bool) {
	s.mu.Lock()
	o := s.orders[id]
	s.mu.Unlock()
	if o == nil {
		return nil, nil, false
	}
	first := today.AddDate(0, 0, 1)
	value := map[string]any{
		"min": first.Format(time.DateOnly),
		"max": today.AddDate(0, 0, 28).Format(time.DateOnly),
		// Stocktaking: the depot dispatches nothing on the 1st and 2nd of the
		// month after next.
		"disabled": []string{
			firstOfMonth(today, 2).Format(time.DateOnly),
			firstOfMonth(today, 2).AddDate(0, 0, 1).Format(time.DateOnly),
		},
	}
	if o.DeliverOn != "" {
		value["start"] = o.DeliverOn
	}
	return rowOf(o), value, true
}

// firstOfMonth is the first day of the month n months after t.
func firstOfMonth(t time.Time, n int) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, n, 0)
}

// setDeliveryDate records the day an order is expected to arrive.
func (s *store) setDeliveryDate(id int, day string) (*order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[id]
	if o == nil {
		return nil, false
	}
	o.DeliverOn = day
	return o, true
}

// shippingFor returns the order as a row plus the methods it can ship by.
func (s *store) shippingFor(id int) (map[string]any, []map[string]any, bool) {
	s.mu.Lock()
	o := s.orders[id]
	s.mu.Unlock()
	if o == nil {
		return nil, nil, false
	}
	return rowOf(o), shippingOptions(o), true
}

// --- writes ---

func (s *store) deleteCustomer(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.customers[id] == nil {
		return false
	}
	delete(s.customers, id)
	return true
}

func (s *store) setStatus(ids []int, status string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, id := range ids {
		if c := s.customers[id]; c != nil {
			c.Status = status
			n++
		}
	}
	return n
}

// saveCustomer validates server side and either reports field errors or
// writes. An id of 0 creates. The errors it returns are the map the form
// widget renders under its fields.
func (s *store) saveCustomer(in customerInput) (map[string]string, *customer) {
	errs := map[string]string{}
	if strings.TrimSpace(in.Name) == "" {
		errs["name"] = "Name must not be empty."
	}
	if !strings.Contains(in.Email, "@") {
		errs["email"] = "Enter a valid email address."
	}
	if in.Seats < 1 {
		errs["seats"] = "At least one seat is required."
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.customers {
		if id != in.ID && strings.EqualFold(c.Email, in.Email) {
			errs["email"] = "This email is already taken."
		}
	}
	if len(errs) > 0 {
		return errs, nil
	}

	c := s.customers[in.ID]
	if c == nil {
		c = &customer{ID: s.nextCustID, CreatedAt: "2026-07-26T00:00:00Z", Utilization: 0}
		s.nextCustID++
		s.customers[c.ID] = c
	}
	c.Name, c.Email, c.Company = in.Name, in.Email, in.Company
	c.Plan, c.Status = in.Plan, in.Status
	c.Seats, c.Notes, c.Notify = in.Seats, in.Notes, in.Notify
	if in.RenewsAt != "" {
		c.RenewsAt = in.RenewsAt + "T00:00:00Z"
	}
	if len(in.Scopes) > 0 {
		c.Scopes = in.Scopes
	}
	if in.Website != "" {
		c.Website = in.Website
	}
	return nil, c
}

// customerInput is the normalized shape both customer forms submit.
type customerInput struct {
	ID       int
	Name     string
	Email    string
	Company  string
	Plan     string
	Status   string
	Seats    int
	RenewsAt string
	Website  string
	Notes    string
	Scopes   []string
	Notify   bool
}

func (s *store) shipOrder(id int, method string) (*order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[id]
	if o == nil {
		return nil, false
	}
	o.Method = method
	o.Status = "shipped"
	return o, true
}

func (s *store) addExtras(id int, extras []string) (*order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[id]
	if o == nil {
		return nil, false
	}
	o.Extras = extras
	return o, true
}

// shippingOptions prices the ways this particular parcel can leave the
// warehouse. Weight and destination decide both the price and what is on
// offer, which is why the options are computed per call rather than authored
// once into the widget.
func shippingOptions(o *order) []map[string]any {
	price := gomukit.DescriptionItem{Label: "Price", Key: "price", Type: gomukit.ColNumber, Format: "currency:EUR"}
	arrives := gomukit.DescriptionItem{Label: "Arrives", Key: "eta", Type: gomukit.ColDate, Format: "date"}
	details := []map[string]any{itemConfig(price), itemConfig(arrives)}

	standard := 4.90 + o.WeightKg*0.80
	express := 14.90 + o.WeightKg*1.60
	drone := 29.00 + o.WeightKg*4.00

	opts := []map[string]any{
		{
			"value": "standard", "label": "Standard", "summary": "3 to 5 business days",
			"body":    "Handed to the postal service tonight and tracked as far as the local depot.",
			"bullets": []string{"Tracked to the depot", "No signature on delivery", "Insured to EUR 50"},
			"details": details,
			"data":    map[string]any{"price": round2(standard), "eta": "2026-07-31T10:00:00Z"},
			"default": true,
		},
		{
			"value": "express", "label": "Express", "summary": "next business day, before 12:00",
			"body":         "Collected by courier this afternoon and delivered to the door tomorrow morning.",
			"bullets":      []string{"Tracked end to end", "Signature required", "Insured to EUR 500"},
			"details":      details,
			"data":         map[string]any{"price": round2(express), "eta": "2026-07-27T12:00:00Z"},
			"badge":        "fastest",
			"badgeVariant": string(gomukit.BadgeSuccess),
		},
	}

	// Heavy parcels cannot fly, so the option is offered but disabled with the
	// reason in view rather than quietly dropped.
	drop := map[string]any{
		"value": "drone", "label": "Drone", "summary": "within the hour",
		"body":    "Flown from the Kreuzberg hub to a rooftop drop point. Weather permitting.",
		"details": details,
		"data":    map[string]any{"price": round2(drone), "eta": "2026-07-26T15:40:00Z"},
	}
	if o.WeightKg > 5 {
		drop["summary"] = "over the 5 kg flight limit"
		drop["disabled"] = true
		delete(drop, "details")
		delete(drop, "data")
	}
	return append(opts, drop)
}

// --- helpers ---

func rowOf(v any) map[string]any {
	rows, err := gomukit.RowsOf([]any{v})
	if err != nil || len(rows) == 0 {
		return map[string]any{}
	}
	return rows[0]
}

// itemConfig renders a DescriptionItem as the JSON shape the runtime reads
// for options delivered at call time. Authored items go through the widget
// config; these travel in structuredContent, so they are built by hand.
func itemConfig(i gomukit.DescriptionItem) map[string]any {
	m := map[string]any{"label": i.Label, "key": i.Key}
	if i.Type != "" {
		m["type"] = string(i.Type)
	}
	if i.Format != "" {
		m["format"] = i.Format
	}
	return m
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

func ptr[T any](v T) *T { return &v }

// ptrInt is ptr for the int-valued validation knobs, spelled out so the
// literals at the call sites stay untyped.
func ptrInt(v int) *int { return &v }
