package eventsourcing

// Canonical NATS event subjects. Phase 10 removes the legacy subjects from
// service code after every publisher and durable consumer has migrated here.
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

	SubjectMarketplaceSyncCompleted = "events.marketplace.sync.completed"
	SubjectMarketplaceSyncFailed    = "events.marketplace.sync.failed"

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
