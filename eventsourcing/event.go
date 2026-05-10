package eventsourcing

import (
	"encoding/json"
	"time"
)

// DomainEvent represents a domain event that can be stored.
type DomainEvent struct {
	ID            string          `json:"id"`
	SchemaVersion int             `json:"schema_version"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	EventData     json.RawMessage `json:"event_data"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

// EventMetadata contains metadata about an event.
type EventMetadata struct {
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	ServiceName   string `json:"service_name,omitempty"`
	TraceID       string `json:"trace_id,omitempty"`
	SpanID        string `json:"span_id,omitempty"`
}

// NewDomainEvent creates a new domain event.
func NewDomainEvent(
	aggregateType string,
	aggregateID string,
	eventType string,
	eventData interface{},
	version int,
) (*DomainEvent, error) {
	data, err := json.Marshal(eventData)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &DomainEvent{
		ID:            generateEventID(),
		SchemaVersion: 1,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		EventData:     data,
		Version:       version,
		OccurredAt:    now,
		CreatedAt:     now,
	}, nil
}

// WithMetadata adds metadata to the event.
func (e *DomainEvent) WithMetadata(meta EventMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	e.Metadata = data
	return nil
}

// UnmarshalData unmarshals the event data into the provided struct.
func (e *DomainEvent) UnmarshalData(v interface{}) error {
	return json.Unmarshal(e.EventData, v)
}

// UnmarshalMetadata unmarshals the event metadata.
func (e *DomainEvent) UnmarshalMetadata() (*EventMetadata, error) {
	if len(e.Metadata) == 0 {
		return &EventMetadata{}, nil
	}
	var meta EventMetadata
	if err := json.Unmarshal(e.Metadata, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// generateEventID generates a unique event ID using timestamp and random bytes.
func generateEventID() string {
	return time.Now().Format("20060102150405.000000000")
}
