package eventsourcing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// EventModel is the GORM model for the event store.
type EventModel struct {
	ID            uint            `gorm:"primaryKey"`
	EventID       string          `gorm:"uniqueIndex;size:100;not null"`
	AggregateType string          `gorm:"size:100;not null;index:idx_aggregate"`
	AggregateID   string          `gorm:"size:100;not null;index:idx_aggregate"`
	EventType     string          `gorm:"size:100;not null;index"`
	EventData     json.RawMessage `gorm:"type:jsonb;not null"`
	Metadata      json.RawMessage `gorm:"type:jsonb"`
	Version       int             `gorm:"not null;index:idx_aggregate"`
	OccurredAt    time.Time       `gorm:"not null"`
	CreatedAt     time.Time       `gorm:"not null"`
}

func (EventModel) TableName() string {
	return "event_store"
}

// PostgresEventStore implements EventStore using PostgreSQL.
type PostgresEventStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewPostgresEventStore creates a new PostgreSQL event store.
func NewPostgresEventStore(db *gorm.DB, logger *zap.Logger) *PostgresEventStore {
	return &PostgresEventStore{
		db:     db,
		logger: logger,
	}
}

// AutoMigrate creates the event store table if it doesn't exist.
func (s *PostgresEventStore) AutoMigrate() error {
	return s.db.AutoMigrate(&EventModel{})
}

// Append appends events to the store with optimistic locking.
func (s *PostgresEventStore) Append(ctx context.Context, events []*DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get the first event to determine aggregate info
		first := events[0]

		// Check current version for optimistic concurrency control
		currentVersion, err := s.getLatestVersionTx(tx, first.AggregateType, first.AggregateID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if currentVersion != expectedVersion {
			s.logger.Warn("Version conflict",
				zap.String("aggregate_type", first.AggregateType),
				zap.String("aggregate_id", first.AggregateID),
				zap.Int("expected", expectedVersion),
				zap.Int("current", currentVersion))
			return ErrConcurrencyConflict
		}

		// Insert all events
		for _, event := range events {
			model := &EventModel{
				EventID:       event.ID,
				AggregateType: event.AggregateType,
				AggregateID:   event.AggregateID,
				EventType:     event.EventType,
				EventData:     event.EventData,
				Metadata:      event.Metadata,
				Version:       event.Version,
				OccurredAt:    event.OccurredAt,
				CreatedAt:     time.Now(),
			}

			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}

		s.logger.Debug("Events appended",
			zap.String("aggregate_type", first.AggregateType),
			zap.String("aggregate_id", first.AggregateID),
			zap.Int("count", len(events)))

		return nil
	})
}

// Load loads all events for a given aggregate.
func (s *PostgresEventStore) Load(ctx context.Context, aggregateType, aggregateID string) ([]*DomainEvent, error) {
	return s.LoadFrom(ctx, aggregateType, aggregateID, 0)
}

// LoadFrom loads events starting from a specific version.
func (s *PostgresEventStore) LoadFrom(ctx context.Context, aggregateType, aggregateID string, fromVersion int) ([]*DomainEvent, error) {
	var models []EventModel

	err := s.db.WithContext(ctx).
		Where("aggregate_type = ? AND aggregate_id = ? AND version >= ?", aggregateType, aggregateID, fromVersion).
		Order("version ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	events := make([]*DomainEvent, len(models))
	for i, m := range models {
		events[i] = &DomainEvent{
			ID:            m.EventID,
			AggregateType: m.AggregateType,
			AggregateID:   m.AggregateID,
			EventType:     m.EventType,
			EventData:     m.EventData,
			Metadata:      m.Metadata,
			Version:       m.Version,
			OccurredAt:    m.OccurredAt,
			CreatedAt:     m.CreatedAt,
		}
	}

	return events, nil
}

// LoadByEventType loads events by event type.
func (s *PostgresEventStore) LoadByEventType(ctx context.Context, eventType string, limit int) ([]*DomainEvent, error) {
	var models []EventModel

	query := s.db.WithContext(ctx).
		Where("event_type = ?", eventType).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	events := make([]*DomainEvent, len(models))
	for i, m := range models {
		events[i] = &DomainEvent{
			ID:            m.EventID,
			AggregateType: m.AggregateType,
			AggregateID:   m.AggregateID,
			EventType:     m.EventType,
			EventData:     m.EventData,
			Metadata:      m.Metadata,
			Version:       m.Version,
			OccurredAt:    m.OccurredAt,
			CreatedAt:     m.CreatedAt,
		}
	}

	return events, nil
}

// GetLatestVersion returns the latest version for an aggregate.
func (s *PostgresEventStore) GetLatestVersion(ctx context.Context, aggregateType, aggregateID string) (int, error) {
	var version int
	err := s.db.WithContext(ctx).
		Model(&EventModel{}).
		Where("aggregate_type = ? AND aggregate_id = ?", aggregateType, aggregateID).
		Select("COALESCE(MAX(version), 0)").
		Row().
		Scan(&version)
	return version, err
}

func (s *PostgresEventStore) getLatestVersionTx(tx *gorm.DB, aggregateType, aggregateID string) (int, error) {
	var version int
	err := tx.Model(&EventModel{}).
		Where("aggregate_type = ? AND aggregate_id = ?", aggregateType, aggregateID).
		Select("COALESCE(MAX(version), 0)").
		Row().
		Scan(&version)
	return version, err
}

// SnapshotModel is the GORM model for snapshots.
type SnapshotModel struct {
	ID            uint   `gorm:"primaryKey"`
	AggregateType string `gorm:"size:100;not null;uniqueIndex:idx_snapshot_aggregate"`
	AggregateID   string `gorm:"size:100;not null;uniqueIndex:idx_snapshot_aggregate"`
	Version       int    `gorm:"not null"`
	Data          []byte `gorm:"type:bytea;not null"`
	CreatedAt     int64  `gorm:"not null"`
}

func (SnapshotModel) TableName() string {
	return "event_snapshots"
}

// PostgresSnapshotStore implements SnapshotStore using PostgreSQL.
type PostgresSnapshotStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewPostgresSnapshotStore creates a new PostgreSQL snapshot store.
func NewPostgresSnapshotStore(db *gorm.DB, logger *zap.Logger) *PostgresSnapshotStore {
	return &PostgresSnapshotStore{
		db:     db,
		logger: logger,
	}
}

// AutoMigrate creates the snapshot table if it doesn't exist.
func (s *PostgresSnapshotStore) AutoMigrate() error {
	return s.db.AutoMigrate(&SnapshotModel{})
}

// Save saves a snapshot, replacing any existing one.
func (s *PostgresSnapshotStore) Save(ctx context.Context, snapshot *Snapshot) error {
	model := &SnapshotModel{
		AggregateType: snapshot.AggregateType,
		AggregateID:   snapshot.AggregateID,
		Version:       snapshot.Version,
		Data:          snapshot.Data,
		CreatedAt:     time.Now().Unix(),
	}

	// Upsert - update if exists, insert if not
	return s.db.WithContext(ctx).
		Where("aggregate_type = ? AND aggregate_id = ?", snapshot.AggregateType, snapshot.AggregateID).
		Assign(map[string]interface{}{
			"version":    model.Version,
			"data":       model.Data,
			"created_at": model.CreatedAt,
		}).
		FirstOrCreate(model).Error
}

// Load loads the latest snapshot for an aggregate.
func (s *PostgresSnapshotStore) Load(ctx context.Context, aggregateType, aggregateID string) (*Snapshot, error) {
	var model SnapshotModel
	err := s.db.WithContext(ctx).
		Where("aggregate_type = ? AND aggregate_id = ?", aggregateType, aggregateID).
		First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &Snapshot{
		AggregateType: model.AggregateType,
		AggregateID:   model.AggregateID,
		Version:       model.Version,
		Data:          model.Data,
		CreatedAt:     model.CreatedAt,
	}, nil
}

// Delete deletes snapshots older than a given version.
func (s *PostgresSnapshotStore) Delete(ctx context.Context, aggregateType, aggregateID string, beforeVersion int) error {
	return s.db.WithContext(ctx).
		Where("aggregate_type = ? AND aggregate_id = ? AND version < ?", aggregateType, aggregateID, beforeVersion).
		Delete(&SnapshotModel{}).Error
}

// CreateEventStoreTable returns the SQL to create the event store table.
func CreateEventStoreTable() string {
	return `
CREATE TABLE IF NOT EXISTS event_store (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(100) UNIQUE NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    metadata JSONB,
    version INT NOT NULL,
    occurred_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_store_aggregate
    ON event_store (aggregate_type, aggregate_id, version);
CREATE INDEX IF NOT EXISTS idx_event_store_event_type
    ON event_store (event_type);
CREATE INDEX IF NOT EXISTS idx_event_store_created_at
    ON event_store (created_at);

-- Unique constraint for optimistic concurrency
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_store_aggregate_version
    ON event_store (aggregate_type, aggregate_id, version);
`
}

// CreateSnapshotStoreTable returns the SQL to create the snapshot store table.
func CreateSnapshotStoreTable() string {
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS event_snapshots (
    id BIGSERIAL PRIMARY KEY,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id VARCHAR(100) NOT NULL,
    version INT NOT NULL,
    data BYTEA NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE (aggregate_type, aggregate_id)
);
`)
}
