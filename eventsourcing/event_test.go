package eventsourcing

import "testing"

func TestNewDomainEventDefaultsSchemaVersion(t *testing.T) {
	event, err := NewDomainEvent("order", "order-1", SubjectOrderCreated, map[string]string{
		"order_id": "order-1",
	}, 1)
	if err != nil {
		t.Fatalf("NewDomainEvent() error = %v", err)
	}

	if event.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", event.SchemaVersion)
	}
}
