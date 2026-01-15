package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Step represents a single step in a saga.
type Step struct {
	Name       string
	Execute    func(ctx context.Context, data json.RawMessage) (json.RawMessage, error)
	Compensate func(ctx context.Context, data json.RawMessage) error
}

// Orchestrator coordinates saga execution with automatic compensation on failure.
type Orchestrator struct {
	store  SagaStore
	logger *zap.Logger
}

// NewOrchestrator creates a new saga orchestrator.
func NewOrchestrator(store SagaStore, logger *zap.Logger) *Orchestrator {
	return &Orchestrator{
		store:  store,
		logger: logger,
	}
}

// Execute runs a saga with the given steps.
func (o *Orchestrator) Execute(ctx context.Context, sagaType, correlationID string, data interface{}, steps []Step) error {
	// Create saga state
	saga, err := NewSagaState(sagaType, correlationID, len(steps), data)
	if err != nil {
		return fmt.Errorf("failed to create saga state: %w", err)
	}

	// Persist initial state
	if err := o.store.Create(ctx, saga); err != nil {
		return fmt.Errorf("failed to save saga: %w", err)
	}

	o.logger.Info("Saga started",
		zap.String("saga_id", saga.ID),
		zap.String("type", sagaType),
		zap.String("correlation_id", correlationID),
		zap.Int("steps", len(steps)))

	// Start the saga
	if err := saga.Start(); err != nil {
		return err
	}
	if err := o.store.Update(ctx, saga); err != nil {
		return err
	}

	// Execute each step
	for i, step := range steps {
		// Create step state
		stepState := &StepState{
			ID:        fmt.Sprintf("%s-step-%d", saga.ID, i),
			SagaID:    saga.ID,
			StepIndex: i,
			StepName:  step.Name,
			Status:    StepStatusRunning,
			Input:     saga.Data,
			StartedAt: time.Now(),
		}

		if err := o.store.SaveStep(ctx, stepState); err != nil {
			o.logger.Warn("Failed to save step state", zap.Error(err))
		}

		o.logger.Debug("Executing saga step",
			zap.String("saga_id", saga.ID),
			zap.Int("step", i),
			zap.String("step_name", step.Name))

		// Execute the step
		output, err := step.Execute(ctx, saga.Data)
		if err != nil {
			// Step failed - start compensation
			o.logger.Error("Saga step failed, starting compensation",
				zap.String("saga_id", saga.ID),
				zap.Int("step", i),
				zap.String("step_name", step.Name),
				zap.Error(err))

			// Update step as failed
			stepState.Status = StepStatusFailed
			stepState.Error = err.Error()
			now := time.Now()
			stepState.CompletedAt = &now
			o.store.SaveStep(ctx, stepState)

			// Mark saga as failed
			saga.Fail(err.Error())
			o.store.Update(ctx, saga)

			// Compensate completed steps in reverse order
			compErr := o.compensate(ctx, saga, steps, i-1)
			if compErr != nil {
				o.logger.Error("Compensation failed",
					zap.String("saga_id", saga.ID),
					zap.Error(compErr))
				return fmt.Errorf("saga failed and compensation failed: %w (original: %v)", compErr, err)
			}

			return fmt.Errorf("saga failed at step %d (%s): %w", i, step.Name, err)
		}

		// Update step as completed
		stepState.Status = StepStatusCompleted
		stepState.Output = output
		now := time.Now()
		stepState.CompletedAt = &now
		o.store.SaveStep(ctx, stepState)

		// Update saga data with step output for next step
		if output != nil {
			saga.Data = output
		}

		// Advance saga
		saga.AdvanceStep()
		o.store.Update(ctx, saga)
	}

	// Complete the saga
	if err := saga.Complete(); err != nil {
		return err
	}
	if err := o.store.Update(ctx, saga); err != nil {
		return err
	}

	o.logger.Info("Saga completed successfully",
		zap.String("saga_id", saga.ID),
		zap.String("type", sagaType))

	return nil
}

// compensate runs compensation for completed steps in reverse order.
func (o *Orchestrator) compensate(ctx context.Context, saga *SagaState, steps []Step, fromStep int) error {
	for i := fromStep; i >= 0; i-- {
		step := steps[i]
		if step.Compensate == nil {
			o.logger.Debug("No compensation for step",
				zap.String("saga_id", saga.ID),
				zap.Int("step", i),
				zap.String("step_name", step.Name))
			continue
		}

		// Update step status to compensating
		stepState := &StepState{
			ID:        fmt.Sprintf("%s-step-%d", saga.ID, i),
			SagaID:    saga.ID,
			StepIndex: i,
			StepName:  step.Name,
			Status:    StepStatusCompensating,
			StartedAt: time.Now(),
		}
		o.store.SaveStep(ctx, stepState)

		o.logger.Debug("Compensating saga step",
			zap.String("saga_id", saga.ID),
			zap.Int("step", i),
			zap.String("step_name", step.Name))

		if err := step.Compensate(ctx, saga.Data); err != nil {
			o.logger.Error("Failed to compensate step",
				zap.String("saga_id", saga.ID),
				zap.Int("step", i),
				zap.String("step_name", step.Name),
				zap.Error(err))

			stepState.Status = StepStatusFailed
			stepState.Error = err.Error()
			now := time.Now()
			stepState.CompletedAt = &now
			o.store.SaveStep(ctx, stepState)

			return fmt.Errorf("compensation failed at step %d (%s): %w", i, step.Name, err)
		}

		stepState.Status = StepStatusCompensated
		now := time.Now()
		stepState.CompletedAt = &now
		o.store.SaveStep(ctx, stepState)
	}

	// Mark saga as compensated
	saga.Compensate()
	o.store.Update(ctx, saga)

	o.logger.Info("Saga compensation completed",
		zap.String("saga_id", saga.ID))

	return nil
}

// Resume attempts to resume a pending saga.
func (o *Orchestrator) Resume(ctx context.Context, sagaID string, steps []Step) error {
	saga, err := o.store.Get(ctx, sagaID)
	if err != nil {
		return err
	}

	if saga.IsTerminal() {
		return ErrSagaAlreadyCompleted
	}

	o.logger.Info("Resuming saga",
		zap.String("saga_id", saga.ID),
		zap.Int("current_step", saga.CurrentStep))

	// Resume from current step
	for i := saga.CurrentStep; i < len(steps); i++ {
		step := steps[i]

		o.logger.Debug("Resuming saga step",
			zap.String("saga_id", saga.ID),
			zap.Int("step", i),
			zap.String("step_name", step.Name))

		output, err := step.Execute(ctx, saga.Data)
		if err != nil {
			saga.Fail(err.Error())
			o.store.Update(ctx, saga)

			// Compensate completed steps
			compErr := o.compensate(ctx, saga, steps, i-1)
			if compErr != nil {
				return fmt.Errorf("saga resume failed and compensation failed: %w", compErr)
			}

			return fmt.Errorf("saga resume failed at step %d: %w", i, err)
		}

		if output != nil {
			saga.Data = output
		}
		saga.AdvanceStep()
		o.store.Update(ctx, saga)
	}

	saga.Complete()
	o.store.Update(ctx, saga)

	return nil
}

// RecoverPending finds and processes pending sagas older than the given duration.
func (o *Orchestrator) RecoverPending(ctx context.Context, olderThan time.Duration, stepsByType map[string][]Step) error {
	sagas, err := o.store.ListPending(ctx, olderThan)
	if err != nil {
		return err
	}

	o.logger.Info("Found pending sagas for recovery",
		zap.Int("count", len(sagas)),
		zap.Duration("older_than", olderThan))

	for _, saga := range sagas {
		steps, ok := stepsByType[saga.Type]
		if !ok {
			o.logger.Warn("No steps registered for saga type",
				zap.String("saga_id", saga.ID),
				zap.String("type", saga.Type))
			continue
		}

		if err := o.Resume(ctx, saga.ID, steps); err != nil {
			o.logger.Error("Failed to recover saga",
				zap.String("saga_id", saga.ID),
				zap.Error(err))
		}
	}

	return nil
}
