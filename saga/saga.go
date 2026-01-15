package saga

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// SagaStatus represents the current status of a saga.
type SagaStatus string

const (
	SagaStatusStarted     SagaStatus = "started"
	SagaStatusRunning     SagaStatus = "running"
	SagaStatusCompleted   SagaStatus = "completed"
	SagaStatusFailed      SagaStatus = "failed"
	SagaStatusCompensated SagaStatus = "compensated"
)

var (
	// ErrSagaNotFound is returned when a saga is not found.
	ErrSagaNotFound = errors.New("saga not found")

	// ErrSagaAlreadyCompleted is returned when trying to modify a completed saga.
	ErrSagaAlreadyCompleted = errors.New("saga already completed")

	// ErrInvalidTransition is returned for invalid state transitions.
	ErrInvalidTransition = errors.New("invalid saga state transition")
)

// SagaState represents the persisted state of a saga.
type SagaState struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	CorrelationID string          `json:"correlation_id"`
	Status        SagaStatus      `json:"status"`
	CurrentStep   int             `json:"current_step"`
	TotalSteps    int             `json:"total_steps"`
	Data          json.RawMessage `json:"data"`
	Error         string          `json:"error,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// StepState represents the state of an individual saga step.
type StepState struct {
	ID          string          `json:"id"`
	SagaID      string          `json:"saga_id"`
	StepIndex   int             `json:"step_index"`
	StepName    string          `json:"step_name"`
	Status      StepStatus      `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// StepStatus represents the status of a saga step.
type StepStatus string

const (
	StepStatusPending      StepStatus = "pending"
	StepStatusRunning      StepStatus = "running"
	StepStatusCompleted    StepStatus = "completed"
	StepStatusFailed       StepStatus = "failed"
	StepStatusCompensating StepStatus = "compensating"
	StepStatusCompensated  StepStatus = "compensated"
)

// SagaStore defines the interface for persisting saga state.
type SagaStore interface {
	// Create creates a new saga.
	Create(ctx context.Context, saga *SagaState) error

	// Get retrieves a saga by ID.
	Get(ctx context.Context, id string) (*SagaState, error)

	// GetByCorrelationID retrieves a saga by correlation ID.
	GetByCorrelationID(ctx context.Context, correlationID string) (*SagaState, error)

	// Update updates an existing saga.
	Update(ctx context.Context, saga *SagaState) error

	// ListPending retrieves sagas in running status that may need recovery.
	ListPending(ctx context.Context, olderThan time.Duration) ([]*SagaState, error)

	// SaveStep saves a saga step state.
	SaveStep(ctx context.Context, step *StepState) error

	// GetSteps retrieves all steps for a saga.
	GetSteps(ctx context.Context, sagaID string) ([]*StepState, error)
}

// NewSagaState creates a new saga state.
func NewSagaState(sagaType, correlationID string, totalSteps int, data interface{}) (*SagaState, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &SagaState{
		ID:            generateSagaID(),
		Type:          sagaType,
		CorrelationID: correlationID,
		Status:        SagaStatusStarted,
		CurrentStep:   0,
		TotalSteps:    totalSteps,
		Data:          dataJSON,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// Start marks the saga as running.
func (s *SagaState) Start() error {
	if s.Status != SagaStatusStarted {
		return ErrInvalidTransition
	}
	s.Status = SagaStatusRunning
	s.UpdatedAt = time.Now()
	return nil
}

// AdvanceStep moves to the next step.
func (s *SagaState) AdvanceStep() error {
	if s.Status != SagaStatusRunning {
		return ErrInvalidTransition
	}
	s.CurrentStep++
	s.UpdatedAt = time.Now()
	return nil
}

// Complete marks the saga as completed.
func (s *SagaState) Complete() error {
	if s.Status != SagaStatusRunning {
		return ErrInvalidTransition
	}
	s.Status = SagaStatusCompleted
	now := time.Now()
	s.CompletedAt = &now
	s.UpdatedAt = now
	return nil
}

// Fail marks the saga as failed.
func (s *SagaState) Fail(err string) error {
	s.Status = SagaStatusFailed
	s.Error = err
	s.UpdatedAt = time.Now()
	return nil
}

// Compensate marks the saga as compensated.
func (s *SagaState) Compensate() error {
	s.Status = SagaStatusCompensated
	now := time.Now()
	s.CompletedAt = &now
	s.UpdatedAt = now
	return nil
}

// UnmarshalData unmarshals the saga data into the provided struct.
func (s *SagaState) UnmarshalData(v interface{}) error {
	return json.Unmarshal(s.Data, v)
}

// UpdateData updates the saga data.
func (s *SagaState) UpdateData(data interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.Data = dataJSON
	s.UpdatedAt = time.Now()
	return nil
}

// IsTerminal returns true if the saga is in a terminal state.
func (s *SagaState) IsTerminal() bool {
	return s.Status == SagaStatusCompleted ||
		s.Status == SagaStatusFailed ||
		s.Status == SagaStatusCompensated
}

// generateSagaID generates a unique saga ID.
func generateSagaID() string {
	return time.Now().Format("20060102150405.000000000")
}
