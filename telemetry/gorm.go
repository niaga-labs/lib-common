package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const (
	gormSpanKey        = "otel:span"
	gormOperationKey   = "otel:operation"
	callbackBeforeName = "otel:before"
	callbackAfterName  = "otel:after"
)

// GORMTracer provides OpenTelemetry tracing for GORM operations
type GORMTracer struct {
	tracer      trace.Tracer
	serviceName string
	dbName      string
}

// NewGORMTracer creates a new GORM tracer
func NewGORMTracer(serviceName, dbName string) *GORMTracer {
	return &GORMTracer{
		tracer:      otel.Tracer(serviceName + "/gorm"),
		serviceName: serviceName,
		dbName:      dbName,
	}
}

// RegisterCallbacks registers tracing callbacks with GORM
func (t *GORMTracer) RegisterCallbacks(db *gorm.DB) error {
	// Create operations
	cb := db.Callback()

	// Register before callbacks
	if err := cb.Create().Before("gorm:create").Register(callbackBeforeName, t.before("INSERT")); err != nil {
		return err
	}
	if err := cb.Query().Before("gorm:query").Register(callbackBeforeName, t.before("SELECT")); err != nil {
		return err
	}
	if err := cb.Update().Before("gorm:update").Register(callbackBeforeName, t.before("UPDATE")); err != nil {
		return err
	}
	if err := cb.Delete().Before("gorm:delete").Register(callbackBeforeName, t.before("DELETE")); err != nil {
		return err
	}
	if err := cb.Row().Before("gorm:row").Register(callbackBeforeName, t.before("ROW")); err != nil {
		return err
	}
	if err := cb.Raw().Before("gorm:raw").Register(callbackBeforeName, t.before("RAW")); err != nil {
		return err
	}

	// Register after callbacks
	if err := cb.Create().After("gorm:create").Register(callbackAfterName, t.after()); err != nil {
		return err
	}
	if err := cb.Query().After("gorm:query").Register(callbackAfterName, t.after()); err != nil {
		return err
	}
	if err := cb.Update().After("gorm:update").Register(callbackAfterName, t.after()); err != nil {
		return err
	}
	if err := cb.Delete().After("gorm:delete").Register(callbackAfterName, t.after()); err != nil {
		return err
	}
	if err := cb.Row().After("gorm:row").Register(callbackAfterName, t.after()); err != nil {
		return err
	}
	if err := cb.Raw().After("gorm:raw").Register(callbackAfterName, t.after()); err != nil {
		return err
	}

	return nil
}

func (t *GORMTracer) before(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db.Statement.Context == nil {
			db.Statement.Context = context.Background()
		}

		// Get table name
		tableName := db.Statement.Table
		if tableName == "" && db.Statement.Schema != nil {
			tableName = db.Statement.Schema.Table
		}

		spanName := fmt.Sprintf("GORM %s %s", operation, tableName)

		ctx, span := t.tracer.Start(db.Statement.Context, spanName,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				semconv.DBSystemPostgreSQL,
				semconv.DBName(t.dbName),
				semconv.DBOperation(operation),
				attribute.String("db.table", tableName),
			),
		)

		db.Statement.Context = ctx
		db.InstanceSet(gormSpanKey, span)
		db.InstanceSet(gormOperationKey, operation)
	}
}

func (t *GORMTracer) after() func(*gorm.DB) {
	return func(db *gorm.DB) {
		spanValue, ok := db.InstanceGet(gormSpanKey)
		if !ok {
			return
		}

		span, ok := spanValue.(trace.Span)
		if !ok {
			return
		}
		defer span.End()

		// Add SQL statement (truncated for large queries)
		sql := db.Dialector.Explain(db.Statement.SQL.String(), db.Statement.Vars...)
		if len(sql) > 1000 {
			sql = sql[:1000] + "..."
		}
		span.SetAttributes(semconv.DBStatement(sql))

		// Add rows affected
		span.SetAttributes(attribute.Int64("db.rows_affected", db.RowsAffected))

		// Handle errors
		if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
			span.RecordError(db.Error)
			span.SetStatus(codes.Error, db.Error.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}

// TraceDB wraps a database operation with tracing
func TraceDB(ctx context.Context, db *gorm.DB, operation, table string) *gorm.DB {
	tracer := otel.Tracer("")
	spanName := fmt.Sprintf("DB %s %s", operation, table)

	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation(operation),
			attribute.String("db.table", table),
		),
	)

	return db.WithContext(ctx).Set(gormSpanKey, span)
}

// DBSpanFromContext extracts database span timing helper
type DBSpanTimer struct {
	ctx       context.Context
	span      trace.Span
	startTime time.Time
}

// StartDBSpan starts a database span timer
func StartDBSpan(ctx context.Context, operation, table string) *DBSpanTimer {
	tracer := otel.Tracer("")
	spanName := fmt.Sprintf("DB %s %s", operation, table)

	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation(operation),
			attribute.String("db.table", table),
		),
	)

	return &DBSpanTimer{
		ctx:       ctx,
		span:      span,
		startTime: time.Now(),
	}
}

// End completes the database span
func (t *DBSpanTimer) End(err error, rowsAffected int64) {
	t.span.SetAttributes(
		attribute.Int64("db.rows_affected", rowsAffected),
		attribute.Int64("db.duration_ms", time.Since(t.startTime).Milliseconds()),
	)

	if err != nil && err != gorm.ErrRecordNotFound {
		t.span.RecordError(err)
		t.span.SetStatus(codes.Error, err.Error())
	} else {
		t.span.SetStatus(codes.Ok, "")
	}

	t.span.End()
}

// Context returns the span context
func (t *DBSpanTimer) Context() context.Context {
	return t.ctx
}
