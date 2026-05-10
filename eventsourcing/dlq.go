package eventsourcing

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// FailedEvent records a message routed to the per-consumer DLQ table.
type FailedEvent struct {
	ID               uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	EventID          string          `gorm:"size:100;index" json:"event_id,omitempty"`
	ConsumerName     string          `gorm:"size:150;not null;index" json:"consumer_name"`
	Subject          string          `gorm:"size:255;not null" json:"subject"`
	Payload          json.RawMessage `gorm:"type:jsonb;not null" json:"payload"`
	Headers          json.RawMessage `gorm:"type:jsonb" json:"headers,omitempty"`
	Error            string          `gorm:"type:text;not null" json:"error"`
	Stream           string          `gorm:"size:150" json:"stream,omitempty"`
	Consumer         string          `gorm:"size:150" json:"consumer,omitempty"`
	StreamSequence   uint64          `json:"stream_sequence,omitempty"`
	ConsumerSequence uint64          `json:"consumer_sequence,omitempty"`
	DeliveredCount   uint64          `json:"delivered_count,omitempty"`
	FailedAt         time.Time       `gorm:"not null;index" json:"failed_at"`
}

// TableName returns the shared failed-event table name.
func (FailedEvent) TableName() string {
	return "events.failed"
}

// DLQRouter persists failed messages and terminates JetStream redelivery.
type DLQRouter struct {
	db           *gorm.DB
	consumerName string
	logger       *zap.Logger
}

// NewDLQRouter creates a Postgres-backed DLQ router.
func NewDLQRouter(db *gorm.DB, consumerName string, logger *zap.Logger) *DLQRouter {
	return &DLQRouter{
		db:           db,
		consumerName: consumerName,
		logger:       logger,
	}
}

// RouteToDLQ stores the failed message in events.failed and ack-terms it.
func (r *DLQRouter) RouteToDLQ(ctx context.Context, msg jetstream.Msg, handlerErr error) error {
	if handlerErr == nil {
		handlerErr = errors.New("message routed to DLQ")
	}

	failed := FailedEvent{
		ConsumerName: r.consumerName,
		Subject:      msg.Subject(),
		Payload:      append(json.RawMessage(nil), msg.Data()...),
		Error:        handlerErr.Error(),
		FailedAt:     time.Now(),
	}

	if eventID := eventIDFromMessage(msg); eventID != "" {
		failed.EventID = eventID
	}

	if headers := msg.Headers(); len(headers) > 0 {
		if data, err := json.Marshal(headers); err == nil {
			failed.Headers = data
		}
	}

	if meta, err := msg.Metadata(); err == nil {
		failed.Stream = meta.Stream
		failed.Consumer = meta.Consumer
		failed.StreamSequence = meta.Sequence.Stream
		failed.ConsumerSequence = meta.Sequence.Consumer
		failed.DeliveredCount = meta.NumDelivered
	}

	if err := r.db.WithContext(ctx).Create(&failed).Error; err != nil {
		return err
	}

	if err := msg.TermWithReason(handlerErr.Error()); err != nil {
		if termErr := msg.Term(); termErr != nil {
			return termErr
		}
	}

	if r.logger != nil {
		r.logger.Warn("Message routed to DLQ",
			zap.String("event_id", failed.EventID),
			zap.String("consumer", r.consumerName),
			zap.String("subject", failed.Subject),
			zap.Error(handlerErr))
	}

	return nil
}

func eventIDFromMessage(msg jetstream.Msg) string {
	if id := msg.Headers().Get(nats.MsgIdHdr); id != "" {
		return id
	}

	var event DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err == nil && event.ID != "" {
		return event.ID
	}

	return ""
}
