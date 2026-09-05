package eventsourcing

// Canonical NATS event subjects. Phase 10 removes the legacy subjects from
// service code after every publisher and durable consumer has migrated here.
//
// EVERY SUBJECT HERE HAS A PUBLISHER AND A CONSUMER, OR SAYS WHY NOT. The full
// subject > publisher > consumers table is in README.md and was measured rather
// than assumed (NIAGA-117). Two subjects are Reserved and two are published with
// no consumer; both cases are commented where they are declared.
const (
	SubjectUserRegistered             = "events.user.registered"
	SubjectUserPasswordResetRequested = "events.user.password_reset_requested"

	SubjectOrderCreated       = "events.order.created"
	SubjectOrderConfirmed     = "events.order.confirmed"
	SubjectOrderCancelled     = "events.order.cancelled"
	SubjectOrderStatusChanged = "events.order.status_changed"

	SubjectPaymentReceived = "events.order.payment.received"
	SubjectPaymentVerified = "events.order.payment.verified"
	SubjectPaymentRejected = "events.order.payment.rejected"
	SubjectPaymentRefunded = "events.order.payment.refunded"

	SubjectInventoryStockUpdated     = "events.inventory.stock.updated"
	SubjectInventoryProductRestocked = "events.inventory.product.restocked"

	SubjectCatalogProductCreated       = "events.catalog.product.created"
	SubjectCatalogProductUpdated       = "events.catalog.product.updated"
	SubjectCatalogProductDeleted       = "events.catalog.product.deleted"
	SubjectCatalogFlashSaleActivated   = "events.catalog.flash_sale.activated"
	SubjectCatalogFlashSaleDeactivated = "events.catalog.flash_sale.deactivated"

	SubjectSupportTicketCreated  = "events.support.ticket.created"
	SubjectSupportTicketReplied  = "events.support.ticket.replied"
	SubjectSupportTicketResolved = "events.support.ticket.resolved"

	// PUBLISHED BY service-marketplace, CONSUMED BY NOBODY (audited 2026-09-06,
	// NIAGA-117). Not a defect and not reserved — these are real events on the
	// wire that no service subscribes to yet. Recorded here so the next reader
	// does not have to grep for a consumer that does not exist.
	SubjectMarketplaceSyncCompleted = "events.marketplace.sync.completed"
	SubjectMarketplaceSyncFailed    = "events.marketplace.sync.failed"

	// RESERVED — DECLARED, NEVER PUBLISHED, NEVER CONSUMED (NIAGA-117).
	//
	// Audited 2026-09-06 across every repo in the workspace, by two independent
	// methods: a search for `eventsourcing.<Const>` in all Go files, and a
	// literal search for the subject STRING in every tracked file of every repo,
	// frontends included. Both agree — these two appear NOWHERE but this file.
	//
	// Kept rather than deleted, deliberately. Nothing imports them, so removing
	// them would be safe today; but they name work that is planned rather than
	// abandoned (service-customer exists, service-agent is legacy-hidden rather
	// than deleted), and the name is the only surviving record of the intent.
	// Deleting costs that and saves nothing. Re-check before publishing either:
	// a subject with no consumer is a message into a void, and the marketplace
	// pair below is already in that state.
	SubjectCustomerCreated     = "events.customer.created"
	SubjectAgentCommissionPaid = "events.agent.commission.paid"
)

var SubjectDomains = map[string]string{
	SubjectUserRegistered:              "user",
	SubjectUserPasswordResetRequested:  "user",
	SubjectOrderCreated:                "order",
	SubjectOrderConfirmed:              "order",
	SubjectOrderCancelled:              "order",
	SubjectOrderStatusChanged:          "order",
	SubjectPaymentReceived:             "order",
	SubjectPaymentVerified:             "order",
	SubjectPaymentRejected:             "order",
	SubjectPaymentRefunded:             "order",
	SubjectInventoryStockUpdated:       "inventory",
	SubjectInventoryProductRestocked:   "inventory",
	SubjectCatalogProductCreated:       "catalog",
	SubjectCatalogProductUpdated:       "catalog",
	SubjectCatalogProductDeleted:       "catalog",
	SubjectCatalogFlashSaleActivated:   "catalog",
	SubjectCatalogFlashSaleDeactivated: "catalog",
	SubjectSupportTicketCreated:        "support",
	SubjectSupportTicketReplied:        "support",
	SubjectSupportTicketResolved:       "support",
	SubjectMarketplaceSyncCompleted:    "marketplace",
	SubjectMarketplaceSyncFailed:       "marketplace",
	SubjectCustomerCreated:             "customer",
	SubjectAgentCommissionPaid:         "agent",
}

type UserRegisteredPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
	Name   string `json:"name,omitempty"`
}

type OrderEventPayload struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

type PaymentEventPayload struct {
	PaymentID string `json:"payment_id,omitempty"`
	OrderID   string `json:"order_id"`
	Provider  string `json:"provider,omitempty"`
	Amount    string `json:"amount,omitempty"`
}

type InventoryStockUpdatedPayload struct {
	ProductID string `json:"product_id"`
	SKU       string `json:"sku,omitempty"`
	Quantity  int    `json:"quantity"`
}

type CatalogProductPayload struct {
	ProductID   string `json:"product_id"`
	ProductType string `json:"product_type,omitempty"`
	Name        string `json:"name,omitempty"`
}

type SupportTicketPayload struct {
	TicketID   string `json:"ticket_id"`
	CustomerID string `json:"customer_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

type MarketplaceSyncPayload struct {
	Marketplace string `json:"marketplace"`
	SyncID      string `json:"sync_id,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}
