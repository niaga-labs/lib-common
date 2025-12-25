package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Event represents an outbox event to be published
type Event struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AggregateType string          `gorm:"type:varchar(100);not null;index" json:"aggregate_type"`
	AggregateID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"aggregate_id"`
	EventType     string          `gorm:"type:varchar(100);not null;index" json:"event_type"`
	Payload       json.RawMessage `gorm:"type:jsonb;not null" json:"payload"`
	CreatedAt     time.Time       `gorm:"not null;index" json:"created_at"`
	ProcessedAt   *time.Time      `gorm:"index" json:"processed_at,omitempty"`
	Error         *string         `gorm:"type:text" json:"error,omitempty"`
	RetryCount    int             `gorm:"default:0" json:"retry_count"`
}

// TableName specifies the table name for Event
func (Event) TableName() string {
	return "outbox.events"
}

// BeforeCreate hook to generate UUID and set created time
func (e *Event) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	return nil
}

// Outbox provides transactional outbox pattern implementation
type Outbox struct {
	db *gorm.DB
}

// NewOutbox creates a new Outbox instance
func NewOutbox(db *gorm.DB) *Outbox {
	return &Outbox{db: db}
}

// PublishInTransaction saves an event within the given transaction
// This ensures the event is atomically saved with the business operation
func (o *Outbox) PublishInTransaction(tx *gorm.DB, event *Event) error {
	return tx.Create(event).Error
}

// Publish saves an event using the default database connection
// Use PublishInTransaction when you need to include the event in an existing transaction
func (o *Outbox) Publish(event *Event) error {
	return o.db.Create(event).Error
}

// CreateEvent is a helper to create an Event with proper JSON marshaling
func CreateEvent(aggregateType string, aggregateID uuid.UUID, eventType string, payload interface{}) (*Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Event{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       data,
		CreatedAt:     time.Now(),
	}, nil
}

// GetUnprocessedEvents retrieves events that haven't been processed yet
func (o *Outbox) GetUnprocessedEvents(limit int) ([]Event, error) {
	var events []Event
	err := o.db.Where("processed_at IS NULL").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// MarkProcessed marks an event as successfully processed
func (o *Outbox) MarkProcessed(eventID uuid.UUID) error {
	now := time.Now()
	return o.db.Model(&Event{}).
		Where("id = ?", eventID).
		Update("processed_at", now).Error
}

// MarkFailed marks an event as failed with an error message
func (o *Outbox) MarkFailed(eventID uuid.UUID, errMsg string) error {
	return o.db.Model(&Event{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"error":       errMsg,
			"retry_count": gorm.Expr("retry_count + 1"),
		}).Error
}

// GetFailedEvents retrieves events that failed processing
func (o *Outbox) GetFailedEvents(maxRetries int, limit int) ([]Event, error) {
	var events []Event
	err := o.db.Where("processed_at IS NULL AND error IS NOT NULL AND retry_count < ?", maxRetries).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// CleanupProcessedEvents removes old processed events
func (o *Outbox) CleanupProcessedEvents(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return o.db.Where("processed_at IS NOT NULL AND processed_at < ?", cutoff).
		Delete(&Event{}).Error
}
