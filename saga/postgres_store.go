package saga

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SagaModel is the GORM model for saga state.
type SagaModel struct {
	ID            uint            `gorm:"primaryKey"`
	SagaID        string          `gorm:"uniqueIndex;size:100;not null"`
	Type          string          `gorm:"size:100;not null;index"`
	CorrelationID string          `gorm:"size:100;not null;index"`
	Status        string          `gorm:"size:20;not null;index"`
	CurrentStep   int             `gorm:"not null"`
	TotalSteps    int             `gorm:"not null"`
	Data          json.RawMessage `gorm:"type:jsonb"`
	Error         string          `gorm:"type:text"`
	CompletedAt   *time.Time
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (SagaModel) TableName() string {
	return "saga_states"
}

// StepModel is the GORM model for saga step state.
type StepModel struct {
	ID          uint            `gorm:"primaryKey"`
	StepID      string          `gorm:"uniqueIndex;size:100;not null"`
	SagaID      string          `gorm:"size:100;not null;index"`
	StepIndex   int             `gorm:"not null"`
	StepName    string          `gorm:"size:100;not null"`
	Status      string          `gorm:"size:20;not null"`
	Input       json.RawMessage `gorm:"type:jsonb"`
	Output      json.RawMessage `gorm:"type:jsonb"`
	Error       string          `gorm:"type:text"`
	StartedAt   time.Time       `gorm:"not null"`
	CompletedAt *time.Time
}

func (StepModel) TableName() string {
	return "saga_steps"
}

// PostgresSagaStore implements SagaStore using PostgreSQL.
type PostgresSagaStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewPostgresSagaStore creates a new PostgreSQL saga store.
func NewPostgresSagaStore(db *gorm.DB, logger *zap.Logger) *PostgresSagaStore {
	return &PostgresSagaStore{
		db:     db,
		logger: logger,
	}
}

// AutoMigrate creates the saga tables if they don't exist.
func (s *PostgresSagaStore) AutoMigrate() error {
	return s.db.AutoMigrate(&SagaModel{}, &StepModel{})
}

// Create creates a new saga.
func (s *PostgresSagaStore) Create(ctx context.Context, saga *SagaState) error {
	model := &SagaModel{
		SagaID:        saga.ID,
		Type:          saga.Type,
		CorrelationID: saga.CorrelationID,
		Status:        string(saga.Status),
		CurrentStep:   saga.CurrentStep,
		TotalSteps:    saga.TotalSteps,
		Data:          saga.Data,
		Error:         saga.Error,
		CompletedAt:   saga.CompletedAt,
		CreatedAt:     saga.CreatedAt,
		UpdatedAt:     saga.UpdatedAt,
	}

	return s.db.WithContext(ctx).Create(model).Error
}

// Get retrieves a saga by ID.
func (s *PostgresSagaStore) Get(ctx context.Context, id string) (*SagaState, error) {
	var model SagaModel
	if err := s.db.WithContext(ctx).Where("saga_id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSagaNotFound
		}
		return nil, err
	}

	return &SagaState{
		ID:            model.SagaID,
		Type:          model.Type,
		CorrelationID: model.CorrelationID,
		Status:        SagaStatus(model.Status),
		CurrentStep:   model.CurrentStep,
		TotalSteps:    model.TotalSteps,
		Data:          model.Data,
		Error:         model.Error,
		CompletedAt:   model.CompletedAt,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}

// GetByCorrelationID retrieves a saga by correlation ID.
func (s *PostgresSagaStore) GetByCorrelationID(ctx context.Context, correlationID string) (*SagaState, error) {
	var model SagaModel
	if err := s.db.WithContext(ctx).Where("correlation_id = ?", correlationID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrSagaNotFound
		}
		return nil, err
	}

	return &SagaState{
		ID:            model.SagaID,
		Type:          model.Type,
		CorrelationID: model.CorrelationID,
		Status:        SagaStatus(model.Status),
		CurrentStep:   model.CurrentStep,
		TotalSteps:    model.TotalSteps,
		Data:          model.Data,
		Error:         model.Error,
		CompletedAt:   model.CompletedAt,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}

// Update updates an existing saga.
func (s *PostgresSagaStore) Update(ctx context.Context, saga *SagaState) error {
	return s.db.WithContext(ctx).Model(&SagaModel{}).
		Where("saga_id = ?", saga.ID).
		Updates(map[string]interface{}{
			"status":       string(saga.Status),
			"current_step": saga.CurrentStep,
			"data":         saga.Data,
			"error":        saga.Error,
			"completed_at": saga.CompletedAt,
			"updated_at":   saga.UpdatedAt,
		}).Error
}

// ListPending retrieves sagas that may need recovery.
func (s *PostgresSagaStore) ListPending(ctx context.Context, olderThan time.Duration) ([]*SagaState, error) {
	var models []SagaModel
	cutoff := time.Now().Add(-olderThan)

	err := s.db.WithContext(ctx).
		Where("status IN (?, ?) AND updated_at < ?",
			string(SagaStatusStarted), string(SagaStatusRunning), cutoff).
		Order("created_at ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	sagas := make([]*SagaState, len(models))
	for i, m := range models {
		sagas[i] = &SagaState{
			ID:            m.SagaID,
			Type:          m.Type,
			CorrelationID: m.CorrelationID,
			Status:        SagaStatus(m.Status),
			CurrentStep:   m.CurrentStep,
			TotalSteps:    m.TotalSteps,
			Data:          m.Data,
			Error:         m.Error,
			CompletedAt:   m.CompletedAt,
			CreatedAt:     m.CreatedAt,
			UpdatedAt:     m.UpdatedAt,
		}
	}

	return sagas, nil
}

// SaveStep saves a saga step state.
func (s *PostgresSagaStore) SaveStep(ctx context.Context, step *StepState) error {
	model := &StepModel{
		StepID:      step.ID,
		SagaID:      step.SagaID,
		StepIndex:   step.StepIndex,
		StepName:    step.StepName,
		Status:      string(step.Status),
		Input:       step.Input,
		Output:      step.Output,
		Error:       step.Error,
		StartedAt:   step.StartedAt,
		CompletedAt: step.CompletedAt,
	}

	// Upsert
	return s.db.WithContext(ctx).
		Where("step_id = ?", step.ID).
		Assign(map[string]interface{}{
			"status":       model.Status,
			"output":       model.Output,
			"error":        model.Error,
			"completed_at": model.CompletedAt,
		}).
		FirstOrCreate(model).Error
}

// GetSteps retrieves all steps for a saga.
func (s *PostgresSagaStore) GetSteps(ctx context.Context, sagaID string) ([]*StepState, error) {
	var models []StepModel
	err := s.db.WithContext(ctx).
		Where("saga_id = ?", sagaID).
		Order("step_index ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	steps := make([]*StepState, len(models))
	for i, m := range models {
		steps[i] = &StepState{
			ID:          m.StepID,
			SagaID:      m.SagaID,
			StepIndex:   m.StepIndex,
			StepName:    m.StepName,
			Status:      StepStatus(m.Status),
			Input:       m.Input,
			Output:      m.Output,
			Error:       m.Error,
			StartedAt:   m.StartedAt,
			CompletedAt: m.CompletedAt,
		}
	}

	return steps, nil
}

// CreateSagaTables returns the SQL to create saga tables.
func CreateSagaTables() string {
	return `
CREATE TABLE IF NOT EXISTS saga_states (
    id BIGSERIAL PRIMARY KEY,
    saga_id VARCHAR(100) UNIQUE NOT NULL,
    type VARCHAR(100) NOT NULL,
    correlation_id VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,
    current_step INT NOT NULL DEFAULT 0,
    total_steps INT NOT NULL,
    data JSONB,
    error TEXT,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_saga_states_type ON saga_states (type);
CREATE INDEX IF NOT EXISTS idx_saga_states_correlation ON saga_states (correlation_id);
CREATE INDEX IF NOT EXISTS idx_saga_states_status ON saga_states (status);
CREATE INDEX IF NOT EXISTS idx_saga_states_updated ON saga_states (updated_at);

CREATE TABLE IF NOT EXISTS saga_steps (
    id BIGSERIAL PRIMARY KEY,
    step_id VARCHAR(100) UNIQUE NOT NULL,
    saga_id VARCHAR(100) NOT NULL,
    step_index INT NOT NULL,
    step_name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_saga_steps_saga ON saga_steps (saga_id);
`
}
