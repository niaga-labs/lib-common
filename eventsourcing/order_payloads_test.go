package eventsourcing

import (
	"encoding/json"
	"sort"
	"testing"
)

// NIAGA-166. These tests pin the wire format, not the Go structs. Renaming a
// field is a breaking change to two services that never import each other, and
// the failure mode is silence: encoding/json does not error on a field the
// sender omitted, so the consumer gets a zero value and carries on.
//
// If you are here because one of these failed, you renamed or dropped a field
// on a published event. Fix the publisher, or change every consumer in the same
// change — do not edit the expectation to match.

// The exact keys every consumer of events.order.created may rely on.
var orderCreatedRequiredKeys = []string{
	"customer_email", "customer_id", "customer_name", "items", "order_id",
	"order_number", "schema_version", "shipping_address", "total",
}

// The exact keys every consumer of events.order.status_changed may rely on.
// Note new_status, not status.
var orderStatusChangedRequiredKeys = []string{
	"customer_email", "customer_name", "new_status", "old_status", "order_id",
	"order_number", "schema_version",
}

func marshalledKeys(t *testing.T, v interface{}) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshalling to a map: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func missing(want, got []string) []string {
	have := map[string]bool{}
	for _, g := range got {
		have[g] = true
	}
	var out []string
	for _, w := range want {
		if !have[w] {
			out = append(out, w)
		}
	}
	return out
}

// A fully-populated payload must emit every key a consumer depends on. omitempty
// on a required field would be a bug: the consumer cannot tell "absent" from
// "empty", which is the whole reason this ticket existed.
func TestOrderCreatedEmitsEveryRequiredKey(t *testing.T) {
	p := OrderCreatedPayload{
		SchemaVersion: 2,
		OrderID:       "11111111-1111-1111-1111-111111111111",
		OrderNumber:   "ORD-1",
		CustomerID:    "22222222-2222-2222-2222-222222222222",
		CustomerName:  "Siti",
		CustomerEmail: "siti@example.com",
		Items:         []OrderEventItem{{ProductID: "p1", ProductName: "Kain", Quantity: 1, UnitPrice: 10}},
		Total:         10,
	}
	if m := missing(orderCreatedRequiredKeys, marshalledKeys(t, p)); len(m) > 0 {
		t.Fatalf("events.order.created is missing required keys: %v", m)
	}
}

func TestOrderStatusChangedEmitsEveryRequiredKey(t *testing.T) {
	p := OrderStatusChangedPayload{
		SchemaVersion: 2,
		OrderID:       "11111111-1111-1111-1111-111111111111",
		OrderNumber:   "ORD-1",
		CustomerName:  "Siti",
		CustomerEmail: "siti@example.com",
		OldStatus:     "processing",
		NewStatus:     "shipped",
	}
	if m := missing(orderStatusChangedRequiredKeys, marshalledKeys(t, p)); len(m) > 0 {
		t.Fatalf("events.order.status_changed is missing required keys: %v", m)
	}
}

// The specific bug: the status field is new_status. A consumer reading status
// gets "" and its switch falls through to default, sending nothing.
func TestStatusFieldIsNewStatusAndNotStatus(t *testing.T) {
	raw, err := json.Marshal(OrderStatusChangedPayload{NewStatus: "shipped"})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if _, wrong := m["status"]; wrong {
		t.Error("payload emits \"status\"; the agreed field is \"new_status\" — " +
			"service-inventory and service-order both use new_status")
	}
	if got, ok := m["new_status"]; !ok || got != "shipped" {
		t.Errorf("new_status = %v (present=%v), want \"shipped\"", got, ok)
	}
}

// A published event must survive a round trip through the contract with every
// customer-facing field intact. This is the assertion that would have caught
// the blank item names and the missing recipient.
func TestOrderCreatedRoundTripKeepsWhatTheEmailNeeds(t *testing.T) {
	in := OrderCreatedPayload{
		SchemaVersion: 2,
		OrderID:       "o1",
		OrderNumber:   "ORD-9",
		CustomerName:  "Aminah",
		CustomerEmail: "aminah@example.com",
		CustomerPhone: "+60123456789",
		Total:         59.9,
		Items: []OrderEventItem{
			{ProductID: "p1", ProductName: "Batik Shirt", SKU: "BS-1", Quantity: 2, UnitPrice: 29.95},
		},
		ShippingAddress: OrderShippingAddress{
			Name: "Aminah", Phone: "+60123456789",
			Address: "12 Jalan Satu", City: "Kuantan", Postcode: "25000",
			State: "Pahang", Country: "Malaysia",
		},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var out OrderCreatedPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	if out.CustomerEmail != in.CustomerEmail {
		t.Errorf("customer_email lost: %q", out.CustomerEmail)
	}
	if !out.DeliverableTo() {
		t.Error("DeliverableTo() is false on a payload that carries an address")
	}
	if len(out.Items) != 1 {
		t.Fatalf("items lost: %d", len(out.Items))
	}
	if out.Items[0].ProductName != "Batik Shirt" {
		t.Errorf("product_name lost: %q — this is why every line rendered blank", out.Items[0].ProductName)
	}
	if out.Items[0].UnitPrice != 29.95 {
		t.Errorf("unit_price lost: %v", out.Items[0].UnitPrice)
	}
	if out.ShippingAddress.Phone != "+60123456789" {
		t.Errorf("shipping_address.phone lost: %q — the SMS leg needs it", out.ShippingAddress.Phone)
	}
}

func TestDeliverableToIsFalseWithoutARecipient(t *testing.T) {
	if (OrderCreatedPayload{}).DeliverableTo() {
		t.Error("an empty payload reports itself deliverable; that is how mail was sent to \"\"")
	}
	if (OrderStatusChangedPayload{NewStatus: "shipped"}).DeliverableTo() {
		t.Error("a payload with no customer_email reports itself deliverable")
	}
}

func TestOneLineSkipsEmptyParts(t *testing.T) {
	got := OrderShippingAddress{Address: "12 Jalan Satu", City: "Kuantan", Country: "Malaysia"}.OneLine()
	want := "12 Jalan Satu, Kuantan, Malaysia"
	if got != want {
		t.Errorf("OneLine() = %q, want %q", got, want)
	}
	if empty := (OrderShippingAddress{}).OneLine(); empty != "" {
		t.Errorf("an empty address renders %q, want the empty string", empty)
	}
}
