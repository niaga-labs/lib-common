package eventsourcing

import (
	"context"
	"errors"
)

var (
	// ErrEventNotFound is returned when an event is not found.
	ErrEventNotFound = errors.New("event not found")

	// ErrConcurrencyConflict is returned when there's a version conflict.
	ErrConcurrencyConflict = errors.New("concurrency conflict: aggregate version mismatch")

	// ErrInvalidAggregateID is returned when aggregate ID is invalid.
	ErrInvalidAggregateID = errors.New("invalid aggregate id")
)

// EventStore defines the interface for persisting and retrieving domain events.
type EventStore interface {
	// Append appends events to the store for a given aggregate.
	// Returns ErrConcurrencyConflict if the expected version doesn't match.
	Append(ctx context.Context, events []*DomainEvent, expectedVersion int) error

	// Load loads all events for a given aggregate.
	Load(ctx context.Context, aggregateType, aggregateID string) ([]*DomainEvent, error)

	// LoadFrom loads events for an aggregate starting from a specific version.
	LoadFrom(ctx context.Context, aggregateType, aggregateID string, fromVersion int) ([]*DomainEvent, error)

	// LoadByEventType loads all events of a specific type.
	LoadByEventType(ctx context.Context, eventType string, limit int) ([]*DomainEvent, error)

	// GetLatestVersion returns the latest version for an aggregate.
	GetLatestVersion(ctx context.Context, aggregateType, aggregateID string) (int, error)
}

// EventPublisher defines the interface for publishing domain events.
type EventPublisher interface {
	// Publish publishes events to subscribers.
	Publish(ctx context.Context, events []*DomainEvent) error
}

// EventSubscriber defines the interface for subscribing to domain events.
type EventSubscriber interface {
	// Subscribe subscribes to events of the given types.
	Subscribe(ctx context.Context, eventTypes []string, handler EventHandler) error
}

// EventHandler is a function that handles domain events.
type EventHandler func(ctx context.Context, event *DomainEvent) error

// StreamPosition represents the position in an event stream.
type StreamPosition struct {
	AggregateType string
	AggregateID   string
	Version       int
}

// Snapshot represents a snapshot of an aggregate's state.
type Snapshot struct {
	AggregateType string
	AggregateID   string
	Version       int
	Data          []byte
	CreatedAt     int64
}

// SnapshotStore defines the interface for storing aggregate snapshots.
type SnapshotStore interface {
	// Save saves a snapshot.
	Save(ctx context.Context, snapshot *Snapshot) error

	// Load loads the latest snapshot for an aggregate.
	Load(ctx context.Context, aggregateType, aggregateID string) (*Snapshot, error)

	// Delete deletes snapshots older than a given version.
	Delete(ctx context.Context, aggregateType, aggregateID string, beforeVersion int) error
}
