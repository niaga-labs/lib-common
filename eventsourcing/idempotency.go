package eventsourcing

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProcessedEvent records that a durable consumer has handled an event.
type ProcessedEvent struct {
	EventID      string    `gorm:"primaryKey;size:100;not null" json:"event_id"`
	ConsumerName string    `gorm:"primaryKey;size:150;not null" json:"consumer_name"`
	ProcessedAt  time.Time `gorm:"not null" json:"processed_at"`
}

// TableName returns the shared idempotency table name.
func (ProcessedEvent) TableName() string {
	return "events.processed"
}

// IdempotencyChecker provides per-consumer event deduplication.
type IdempotencyChecker struct {
	db *gorm.DB
}

// NewIdempotencyChecker creates a Postgres-backed idempotency checker.
func NewIdempotencyChecker(db *gorm.DB) *IdempotencyChecker {
	return &IdempotencyChecker{db: db}
}

// CheckAndMark inserts an event/consumer pair if it has not been processed.
// It returns true when the caller should process the event, and false when the
// event is a duplicate that should be acked and skipped.
func (c *IdempotencyChecker) CheckAndMark(ctx context.Context, eventID, consumerName string) (bool, error) {
	row := &ProcessedEvent{
		EventID:      eventID,
		ConsumerName: consumerName,
		ProcessedAt:  time.Now(),
	}

	result := c.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(row)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}
