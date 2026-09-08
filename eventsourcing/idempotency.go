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
//
// THE ROW IS WRITTEN BEFORE THE HANDLER RUNS. That is deliberate — it is what
// stops two concurrent deliveries of the same event from both being processed —
// but it means the row is a CLAIM on the event, not proof that the work is done.
// A handler that FAILS must give the claim back with Release, or the redelivery
// is treated as a duplicate and acked without ever running. See Release.
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

// Release gives back the claim CheckAndMark took, so a later delivery of the same
// event is processed instead of being skipped as a duplicate.
//
// NIAGA-263. Without this, retry and dead-lettering were both dead for every
// consumer using this type, and silently: the handler returned an error, the
// consumer NAKed, JetStream redelivered, CheckAndMark reported "already
// processed", and the message was ACKED WITHOUT THE HANDLER RUNNING. Because the
// redelivery never reached the handler, NumDelivered never climbed to the
// max-deliver threshold either, so the DLQ branch was unreachable as well. The
// consumer logged "Handler error, will retry" — a promise the code could not
// keep, which is what made it invisible in operation.
//
// CALL IT ON THE HANDLER'S ERROR PATH, BEFORE NAKING — and NOT when routing to
// the DLQ. An event that has gone to the DLQ is finished, and its claim is what
// stops it being picked up again.
//
// Releasing is deliberately a delete rather than a status flag: the table means
// "these events are done or in flight", and the smallest correct change is to
// stop lying about the failed ones.
//
// THE HOLES THAT REMAIN -- plural, and an earlier draft of this comment said
// "the remaining hole" as though there were one:
//
//  1. A process that dies between claiming and releasing strands that one event.
//     Strictly better than the previous behaviour, where EVERY handler error
//     stranded one, but not nothing.
//  2. AckWait expiring while the handler is still running. The redelivery finds
//     the claim, acks, and terminates a message the first delivery is still
//     working on; when that one then fails, Release deletes the claim and the NAK
//     lands on an already-acked message. The event is lost, never reaches the
//     DLQ, and now leaves no row behind either -- previously the stale row was at
//     least a trace. This predates NIAGA-263 and is not fixed by it.
//  3. RouteToDLQ itself failing. The caller returns without acking or releasing,
//     and NumDelivered is already at MaxDeliver, so nothing redelivers. It writes
//     the events.failed row BEFORE terminating the message, so that error does not
//     mean the row is missing -- check events.failed before calling such an event
//     lost.
//
// REPLAYING FROM THE DLQ NEEDS THE CLAIM DELETED FIRST. Republishing a
// dead-lettered event with the same Nats-Msg-Id will find the retained row, be
// acked and skipped, silently. No replay tooling exists in this workspace today;
// whoever writes it must delete the events.processed row as part of the replay.
//
// A delete affecting 0 rows is not treated as an error: callers only reach here
// after CheckAndMark returned true, so 0 means something else removed the claim,
// which is anomalous but not this function's business to fail on.
func (c *IdempotencyChecker) Release(ctx context.Context, eventID, consumerName string) error {
	return c.db.WithContext(ctx).
		Where("event_id = ? AND consumer_name = ?", eventID, consumerName).
		Delete(&ProcessedEvent{}).Error
}
