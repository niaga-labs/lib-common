package outbox

import (
	"testing"

	"github.com/google/uuid"
)

type recordingPublisher struct {
	subject string
	data    []byte
	headers map[string]string
}

func (p *recordingPublisher) Publish(subject string, data []byte) error {
	p.subject = subject
	p.data = data
	return nil
}

func (p *recordingPublisher) PublishWithHeaders(subject string, data []byte, headers map[string]string) error {
	p.subject = subject
	p.data = data
	p.headers = headers
	return nil
}

func TestProcessEventPublishesOutboxIDAsNATSMsgID(t *testing.T) {
	eventID := uuid.New()
	publisher := &recordingPublisher{}
	processor := &Processor{publisher: publisher}

	err := processor.processEvent(Event{
		ID:        eventID,
		EventType: "events.order.created",
		Payload:   []byte(`{"order_id":"order-1"}`),
	})
	if err != nil {
		t.Fatalf("processEvent() error = %v", err)
	}

	if publisher.subject != "events.order.created" {
		t.Fatalf("subject = %q, want events.order.created", publisher.subject)
	}

	if got := publisher.headers["Nats-Msg-Id"]; got != eventID.String() {
		t.Fatalf("Nats-Msg-Id = %q, want %q", got, eventID.String())
	}
}
