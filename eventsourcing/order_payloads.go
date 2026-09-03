package eventsourcing

import "strings"

// The payload contract for the two order subjects that carry customer-facing
// notifications, SubjectOrderCreated and SubjectOrderStatusChanged.
//
// It lives here because it is a contract between two services that never import
// each other, and until NIAGA-166 there was nowhere for it to live — so the two
// sides drifted, silently, in five separate ways at once:
//
//   - service-order published new_status; service-notification read status. A
//     missing field is not an error to encoding/json, so Status was "", the
//     handler's switch fell through to default, logged "Unknown order status"
//     and returned nil. No shipped email could ever be sent.
//   - customer_email and customer_name were never published at all, so the
//     confirmation email was addressed to "".
//   - the shipping address was published as address and read as
//     shipping_address, and as an object on one side against a string on the
//     other, so even correcting the name would have turned a silent miss into
//     an unmarshal error.
//   - order items were published as {product_id, unit_price} and read as
//     {name, price}, so every line on the confirmation email would have shown
//     a blank name at a price of zero.
//   - the SMS leg passed an email address where a phone number belongs.
//
// Every one of those was invisible in production: nothing errored, nothing
// retried, nothing appeared in a dead-letter queue. The order simply moved to
// shipped and the customer heard nothing.
//
// So the rule this file exists to enforce: a publisher's output must satisfy
// this contract, and a consumer must decode into it. Neither side may define
// its own copy. service-order proves conformance in
// internal/events/order_contract_test.go; service-notification decodes these
// types directly.

// OrderShippingAddress is the delivery address as published. It is an object,
// not a preformatted string: presentation belongs to whoever renders it, and
// the SMS leg needs Phone on its own.
type OrderShippingAddress struct {
	Name     string `json:"name,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	Address  string `json:"address,omitempty"`
	Address2 string `json:"address2,omitempty"`
	City     string `json:"city,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country,omitempty"`
}

// OneLine renders the address for an email or SMS body, skipping empty parts so
// a sparse address does not come out full of stray commas.
func (a OrderShippingAddress) OneLine() string {
	parts := []string{a.Address, a.Address2, a.City, a.Postcode, a.State, a.Country}
	kept := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

// OrderEventItem is one line of an order as it appears on both subjects.
//
// ProductName and UnitPrice are the fields a customer-facing template needs. A
// consumer that wants a display name must read ProductName — there is no name
// field, and there never was one, which is why every item on the confirmation
// email rendered blank.
type OrderEventItem struct {
	ProductID   string  `json:"product_id"`
	VariantID   string  `json:"variant_id,omitempty"`
	ProductName string  `json:"product_name"`
	VariantName string  `json:"variant_name,omitempty"`
	SKU         string  `json:"sku,omitempty"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

// OrderCreatedPayload is the contract for SubjectOrderCreated.
//
// A publisher may add fields — service-order carries customization_* on each
// item, which no notification consumer reads — but it may not rename or drop
// one of these without breaking every consumer silently.
type OrderCreatedPayload struct {
	SchemaVersion   int                  `json:"schema_version"`
	OrderID         string               `json:"order_id"`
	OrderNumber     string               `json:"order_number"`
	CustomerID      string               `json:"customer_id"`
	CustomerName    string               `json:"customer_name"`
	CustomerEmail   string               `json:"customer_email"`
	CustomerPhone   string               `json:"customer_phone,omitempty"`
	Items           []OrderEventItem     `json:"items"`
	Total           float64              `json:"total"`
	ShippingAddress OrderShippingAddress `json:"shipping_address"`
}

// OrderStatusChangedPayload is the contract for SubjectOrderStatusChanged.
//
// The status field is new_status, not status. service-order published
// new_status from the start and service-inventory has always read it; it was
// service-notification that read status and therefore received nothing. The
// publisher's name wins because two of the three sides already agreed on it.
type OrderStatusChangedPayload struct {
	SchemaVersion int    `json:"schema_version"`
	OrderID       string `json:"order_id"`
	OrderNumber   string `json:"order_number"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
	CustomerPhone string `json:"customer_phone,omitempty"`
	OldStatus     string `json:"old_status"`
	NewStatus     string `json:"new_status"`

	// Present on a shipped order. TrackingURL is empty when the courier
	// integration did not supply one — render the number alone in that case
	// rather than an empty link.
	TrackingNumber string `json:"tracking_number,omitempty"`
	TrackingURL    string `json:"tracking_url,omitempty"`

	Notes string `json:"notes,omitempty"`
}

// DeliverableTo reports whether the payload carries somewhere to send an email.
// A consumer should skip and say so, rather than call an email service with an
// empty To — which is exactly what service-notification did.
func (p OrderStatusChangedPayload) DeliverableTo() bool { return p.CustomerEmail != "" }

// DeliverableTo reports the same for the confirmation email.
func (p OrderCreatedPayload) DeliverableTo() bool { return p.CustomerEmail != "" }
