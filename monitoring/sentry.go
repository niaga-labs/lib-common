package monitoring

import (
	"fmt"
	"net/http"
	"runtime/debug"
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
	Debug            bool
	SampleRate       float64
	TracesSampleRate float64
}

// SentryMonitor provides error tracking capabilities
type SentryMonitor struct {
	config *SentryConfig
	logger *zap.Logger
}

// NewSentryMonitor initializes Sentry and returns a monitor instance
func NewSentryMonitor(config *SentryConfig, logger *zap.Logger) (*SentryMonitor, error) {
	if config.DSN == "" {
		logger.Warn("Sentry DSN not configured, error tracking disabled")
		return &SentryMonitor{config: config, logger: logger}, nil
	}

	// Set defaults
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}
	if config.TracesSampleRate == 0 {
		config.TracesSampleRate = 0.1 // Sample 10% of transactions for performance
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              config.DSN,
		Environment:      config.Environment,
		Release:          config.Release,
		Debug:            config.Debug,
		SampleRate:       config.SampleRate,
		TracesSampleRate: config.TracesSampleRate,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Add service name tag to all events
			if event.Tags == nil {
				event.Tags = make(map[string]string)
			}
			event.Tags["service"] = config.ServiceName
			return event
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sentry: %w", err)
	}

	logger.Info("Sentry initialized",
		zap.String("service", config.ServiceName),
		zap.String("environment", config.Environment),
	)

	return &SentryMonitor{config: config, logger: logger}, nil
}

// CaptureError captures an error with optional additional context
func (m *SentryMonitor) CaptureError(err error, tags map[string]string, extra map[string]interface{}) {
	if m.config.DSN == "" {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		for k, v := range extra {
			scope.SetExtra(k, v)
		}
		sentry.CaptureException(err)
	})
}

// CaptureMessage captures a message with level
func (m *SentryMonitor) CaptureMessage(message string, level sentry.Level, tags map[string]string) {
	if m.config.DSN == "" {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		sentry.CaptureMessage(message)
	})
}

// GinMiddleware returns Gin middleware for Sentry
func (m *SentryMonitor) GinMiddleware() gin.HandlerFunc {
	if m.config.DSN == "" {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return sentrygin.New(sentrygin.Options{
		Repanic: true,
	})
}

// RecoveryMiddleware returns a Gin middleware that captures panics
func (m *SentryMonitor) RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())

				// Log the panic
				m.logger.Error("Panic recovered",
					zap.Any("panic", r),
					zap.String("stack", stack),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				// Capture to Sentry
				if m.config.DSN != "" {
					hub := sentry.GetHubFromContext(c.Request.Context())
					if hub == nil {
						hub = sentry.CurrentHub().Clone()
					}

					hub.WithScope(func(scope *sentry.Scope) {
						scope.SetTag("panic", "true")
						scope.SetExtra("stack_trace", stack)
						scope.SetExtra("path", c.Request.URL.Path)
						scope.SetExtra("method", c.Request.Method)

						if err, ok := r.(error); ok {
							hub.CaptureException(err)
						} else {
							hub.CaptureMessage(fmt.Sprintf("Panic: %v", r))
						}
					})
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// Flush waits for all events to be sent to Sentry
func (m *SentryMonitor) Flush(timeout time.Duration) {
	if m.config.DSN != "" {
		sentry.Flush(timeout)
	}
}

// SetUser sets user context for current request
func SetUser(c *gin.Context, userID, email string) {
	if hub := sentry.GetHubFromContext(c.Request.Context()); hub != nil {
		hub.Scope().SetUser(sentry.User{
			ID:    userID,
			Email: email,
		})
	}
}

// AddBreadcrumb adds a breadcrumb to the current scope
func AddBreadcrumb(c *gin.Context, category, message string, data map[string]interface{}) {
	if hub := sentry.GetHubFromContext(c.Request.Context()); hub != nil {
		hub.AddBreadcrumb(&sentry.Breadcrumb{
			Category:  category,
			Message:   message,
			Data:      data,
			Level:     sentry.LevelInfo,
			Timestamp: time.Now(),
		}, nil)
	}
}

// StartTransaction starts a new Sentry transaction for performance monitoring
func StartTransaction(c *gin.Context, name, operation string) *sentry.Span {
	hub := sentry.GetHubFromContext(c.Request.Context())
	if hub == nil {
		return nil
	}

	span := sentry.StartSpan(c.Request.Context(), operation,
		sentry.WithTransactionName(name),
	)
	return span
}
