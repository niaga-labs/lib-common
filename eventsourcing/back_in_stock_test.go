package eventsourcing

import (
	"encoding/json"
	"strings"
	"testing"
)

// backInStockRequiredKeys are the keys service-notification's template needs to
// render the email. A payload missing any of them produces an email with a hole
// in it, or none at all — and nothing upstream reports that, because publishing
// succeeded (NIAGA-166, NIAGA-178).
var backInStockRequiredKeys = []string{
	"subscription_id",
	"customer_id",
	"customer_email",
	"product_id",
	"product_name",
	"stock_quantity",
}

// A fully-populated payload must emit every key the consumer depends on.
// `omitempty` on a required field would be the bug: the consumer cannot tell
// "absent" from "empty".
func TestBackInStockEmitsEveryRequiredKey(t *testing.T) {
	p := CustomerBackInStockPayload{
		SubscriptionID: "11111111-1111-1111-1111-111111111111",
		CustomerID:     "22222222-2222-2222-2222-222222222222",
		CustomerEmail:  "siti@example.com",
		CustomerName:   "Siti",
		ProductID:      "33333333-3333-3333-3333-333333333333",
		ProductName:    "Kain Batik",
		ProductSlug:    "kain-batik",
		StockQuantity:  4,
	}

	if m := missing(backInStockRequiredKeys, marshalledKeys(t, p)); len(m) > 0 {
		t.Fatalf("%s is missing required keys: %v", SubjectCustomerBackInStock, m)
	}
}

// stock_quantity must survive as a NUMBER and must be present even at zero.
//
// Zero is meaningful here rather than absent: it means the restock that
// triggered this has already sold out again, and an email saying so is different
// from an email with no quantity at all. `omitempty` on an int would erase
// exactly that case.
func TestBackInStockKeepsAZeroQuantity(t *testing.T) {
	raw, err := json.Marshal(CustomerBackInStockPayload{StockQuantity: 0})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"stock_quantity":0`) {
		t.Errorf("a zero quantity was dropped from the payload: %s", raw)
	}
}

// The variant fields are optional, and must actually be omitted when absent —
// an empty string renders as a blank line in the template rather than being
// skipped by it.
func TestBackInStockOmitsAbsentVariantFields(t *testing.T) {
	raw, err := json.Marshal(CustomerBackInStockPayload{
		CustomerEmail: "siti@example.com",
		ProductName:   "Kain Batik",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"variant_id", "variant_sku", "variant_name"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("%s should be omitted when empty, got: %s", key, raw)
		}
	}
}

// Every subject this package declares must also be in SubjectDomains, because
// that map is what tells a reader which service owns a subject. A subject
// declared and left out of it is invisible to anything that walks the catalog.
//
// Go cannot enumerate a package's constants, so this checks the subjects added
// most recently and the ones a reader is most likely to add beside them, rather
// than claiming to be exhaustive. The README table is the exhaustive record
// (NIAGA-117).
func TestNewSubjectsAreRegisteredInSubjectDomains(t *testing.T) {
	for _, s := range []string{
		SubjectCustomerBackInStock,
		SubjectCustomerCreated,
		SubjectAgentCommissionPaid,
		SubjectInventoryProductRestocked,
		SubjectMarketplaceSyncCompleted,
	} {
		domain, ok := SubjectDomains[s]
		if !ok {
			t.Errorf("%q is declared but missing from SubjectDomains", s)
			continue
		}
		if domain == "" {
			t.Errorf("%q maps to an empty domain", s)
		}
	}
}

// The subject has to be canonical. A subject that does not start with "events."
// reaches no consumer, and nothing reports it — a JetStream consumer whose
// filter matches nothing is a healthy consumer that never fires (NIAGA-116).
func TestBackInStockSubjectIsCanonical(t *testing.T) {
	if !strings.HasPrefix(SubjectCustomerBackInStock, "events.") {
		t.Errorf("subject %q is not canonical", SubjectCustomerBackInStock)
	}
	if SubjectCustomerBackInStock != "events.customer.back_in_stock" {
		t.Errorf("subject = %q; changing a published subject silently breaks every consumer", SubjectCustomerBackInStock)
	}
}
