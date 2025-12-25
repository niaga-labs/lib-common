package outbox

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Publisher defines the interface for publishing events
type Publisher interface {
	Publish(subject string, data []byte) error
}

// Processor polls the outbox table and publishes events
type Processor struct {
	outbox      *Outbox
	publisher   Publisher
	logger      *zap.Logger
	interval    time.Duration
	batchSize   int
	maxRetries  int
	done        chan struct{}
}

// ProcessorConfig holds configuration for the Processor
type ProcessorConfig struct {
	Interval   time.Duration // How often to poll for new events
	BatchSize  int           // Number of events to process per batch
	MaxRetries int           // Maximum retry attempts for failed events
}

// DefaultProcessorConfig returns default configuration
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		Interval:   5 * time.Second,
		BatchSize:  100,
		MaxRetries: 5,
	}
}

// NewProcessor creates a new outbox processor
func NewProcessor(outbox *Outbox, publisher Publisher, logger *zap.Logger, config ProcessorConfig) *Processor {
	return &Processor{
		outbox:     outbox,
		publisher:  publisher,
		logger:     logger,
		interval:   config.Interval,
		batchSize:  config.BatchSize,
		maxRetries: config.MaxRetries,
		done:       make(chan struct{}),
	}
}

// Start begins processing outbox events in a background goroutine
func (p *Processor) Start(ctx context.Context) {
	go p.run(ctx)
}

// Stop gracefully stops the processor
func (p *Processor) Stop() {
	close(p.done)
}

func (p *Processor) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Process immediately on start
	p.processBatch()

	for {
		select {
		case <-ticker.C:
			p.processBatch()
		case <-ctx.Done():
			p.logger.Info("Outbox processor stopping due to context cancellation")
			return
		case <-p.done:
			p.logger.Info("Outbox processor stopping")
			return
		}
	}
}

func (p *Processor) processBatch() {
	// Get unprocessed events
	events, err := p.outbox.GetUnprocessedEvents(p.batchSize)
	if err != nil {
		p.logger.Error("Failed to get unprocessed events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	p.logger.Debug("Processing outbox events", zap.Int("count", len(events)))

	for _, event := range events {
		if err := p.processEvent(event); err != nil {
			p.logger.Error("Failed to process event",
				zap.String("event_id", event.ID.String()),
				zap.String("event_type", event.EventType),
				zap.Error(err))

			// Mark as failed for retry
			if markErr := p.outbox.MarkFailed(event.ID, err.Error()); markErr != nil {
				p.logger.Error("Failed to mark event as failed",
					zap.String("event_id", event.ID.String()),
					zap.Error(markErr))
			}
			continue
		}

		// Mark as processed
		if err := p.outbox.MarkProcessed(event.ID); err != nil {
			p.logger.Error("Failed to mark event as processed",
				zap.String("event_id", event.ID.String()),
				zap.Error(err))
		}
	}

	// Also retry failed events
	p.retryFailedEvents()
}

func (p *Processor) processEvent(event Event) error {
	// Build the subject from aggregate type and event type
	// e.g., "order.created", "inventory.restocked"
	subject := event.EventType

	return p.publisher.Publish(subject, event.Payload)
}

func (p *Processor) retryFailedEvents() {
	events, err := p.outbox.GetFailedEvents(p.maxRetries, p.batchSize)
	if err != nil {
		p.logger.Error("Failed to get failed events for retry", zap.Error(err))
		return
	}

	for _, event := range events {
		p.logger.Info("Retrying failed event",
			zap.String("event_id", event.ID.String()),
			zap.Int("retry_count", event.RetryCount))

		if err := p.processEvent(event); err != nil {
			p.logger.Error("Retry failed",
				zap.String("event_id", event.ID.String()),
				zap.Int("retry_count", event.RetryCount+1),
				zap.Error(err))

			if markErr := p.outbox.MarkFailed(event.ID, err.Error()); markErr != nil {
				p.logger.Error("Failed to update retry count", zap.Error(markErr))
			}
			continue
		}

		if err := p.outbox.MarkProcessed(event.ID); err != nil {
			p.logger.Error("Failed to mark retried event as processed", zap.Error(err))
		}
	}
}

// ProcessNow forces immediate processing (useful for testing)
func (p *Processor) ProcessNow() {
	p.processBatch()
}
