package monitoring

import (
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SentryConfig holds configuration for Sentry
type SentryConfig struct {
	DSN              string
	Environment      string
	Release          string
	ServiceName      string
	TracesSampleRate float64
}

// SentryMonitor provides Sentry integration for monitoring
type SentryMonitor struct {
	logger *zap.Logger
	config *SentryConfig
}

// NewSentryMonitor creates a new Sentry monitor instance
func NewSentryMonitor(config *SentryConfig, logger *zap.Logger) (*SentryMonitor, error) {
	if config.DSN == "" {
		// Skip Sentry initialization if DSN is not provided
		return &SentryMonitor{
			logger: logger,
			config: config,
		}, nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              config.DSN,
		Environment:      config.Environment,
		Release:          config.Release,
		TracesSampleRate: config.TracesSampleRate,
		AttachStacktrace: true,
	})
	if err != nil {
		return nil, err
	}

	return &SentryMonitor{
		logger: logger,
		config: config,
	}, nil
}

// GinMiddleware returns a Gin middleware for Sentry
func (m *SentryMonitor) GinMiddleware() gin.HandlerFunc {
	if m.config.DSN == "" {
		// Return a no-op middleware if Sentry is not configured
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return sentrygin.New(sentrygin.Options{
		Repanic: true,
	})
}

// RecoveryMiddleware returns a recovery middleware that reports panics to Sentry
func (m *SentryMonitor) RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				if m.config.DSN != "" {
					sentry.CurrentHub().Recover(err)
				}
				if m.logger != nil {
					m.logger.Error("Panic recovered", zap.Any("error", err))
				}
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}

// Flush flushes any buffered events to Sentry
func (m *SentryMonitor) Flush(timeout time.Duration) {
	if m.config.DSN != "" {
		sentry.Flush(timeout)
	}
}

// CaptureError reports an error to Sentry
func (m *SentryMonitor) CaptureError(err error) {
	if m.config.DSN != "" {
		sentry.CaptureException(err)
	}
	if m.logger != nil {
		m.logger.Error("Error captured", zap.Error(err))
	}
}

// CaptureMessage reports a message to Sentry
func (m *SentryMonitor) CaptureMessage(msg string) {
	if m.config.DSN != "" {
		sentry.CaptureMessage(msg)
	}
	if m.logger != nil {
		m.logger.Info("Message captured", zap.String("message", msg))
	}
}
