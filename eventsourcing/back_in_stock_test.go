package eventsourcing

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
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

// EVERY subject this package declares must also be in SubjectDomains, because
// that map is what tells a reader which service owns a subject. A subject
// declared and left out of it is invisible to anything that walks the catalog.
//
// This reads catalog.go's OWN SOURCE rather than a hand-maintained list. A list
// would carry the identical flaw it is meant to catch: a subject added next
// month and left out of the LIST is exactly as invisible as one left out of the
// map. Parsing the file makes the check self-updating, which is the only version
// of this test worth having (raised in review of NIAGA-123).
func TestEverySubjectIsRegisteredInSubjectDomains(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "catalog.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing catalog.go: %v", err)
	}

	declared := map[string]string{} // const name -> subject string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if !strings.HasPrefix(name.Name, "Subject") || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				declared[name.Name] = strings.Trim(lit.Value, `"`)
			}
		}
	}

	// A parse that found nothing is a passing test that checked nothing — the
	// failure mode ~/.claude/rules/verification.md exists for.
	if len(declared) < 20 {
		t.Fatalf("only %d Subject constants parsed out of catalog.go; the parse is wrong, not the catalog", len(declared))
	}

	for name, subject := range declared {
		domain, ok := SubjectDomains[subject]
		if !ok {
			t.Errorf("%s (%q) is declared but missing from SubjectDomains", name, subject)
			continue
		}
		if domain == "" {
			t.Errorf("%s (%q) maps to an empty domain", name, subject)
		}
		if !strings.HasPrefix(subject, "events.") {
			t.Errorf("%s (%q) is not canonical; it would reach no consumer", name, subject)
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

// product_url is the email's call-to-action, and it follows the convention every
// other templated email here uses: reset_url, verification_url and cart_url are
// all built by the PUBLISHER and handed over whole.
//
// The point is that service-notification never learns the storefront's route
// shape. Give it a slug instead and it owns the base URL and the /products/:slug
// pattern, so a storefront route change has to be made in a service that has
// nothing to do with the storefront.
//
// It is optional rather than required because service-customer has no storefront
// base URL configured yet (checked 2026-09-06) — NIAGA-123's publisher has to add
// that config. An absent product_url is an email without a working link, not a
// broken one, so this is a soft edge rather than a hard failure.
func TestProductURLIsCarriedWholeAndOmittedWhenAbsent(t *testing.T) {
	withURL, err := json.Marshal(CustomerBackInStockPayload{
		ProductURL: "https://shop.example.com/products/kain-batik",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(withURL), `"product_url":"https://shop.example.com/products/kain-batik"`) {
		t.Errorf("the full URL was not carried through: %s", withURL)
	}

	without, err := json.Marshal(CustomerBackInStockPayload{ProductName: "Kain Batik"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(without), "product_url") {
		t.Errorf("product_url should be omitted when empty, got: %s", without)
	}
}
