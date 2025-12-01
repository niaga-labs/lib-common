package database

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

// Connect establishes a connection to PostgreSQL database
func Connect(config PostgresConfig, zapLogger *zap.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		config.SSLMode,
	)

	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Info)
	if zapLogger != nil {
		gormLogger = NewGormLogger(zapLogger)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying *sql.DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	zapLogger.Info("Successfully connected to PostgreSQL database",
		zap.String("host", config.Host),
		zap.String("database", config.Database),
	)

	return db, nil
}

// HealthCheck verifies the database connection is alive
func HealthCheck(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// NewGormLogger creates a GORM logger that uses Zap
func NewGormLogger(zapLogger *zap.Logger) logger.Interface {
	return &gormLogger{
		zap:                       zapLogger,
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
	}
}

type gormLogger struct {
	zap                       *zap.Logger
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return l
}

func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	l.zap.Info(msg, zap.Any("data", data))
}

func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.zap.Warn(msg, zap.Any("data", data))
}

func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.zap.Error(msg, zap.Any("data", data))
}

func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	if err != nil {
		l.zap.Error("database query error",
			zap.Error(err),
			zap.Duration("elapsed", elapsed),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
		)
	} else if elapsed > l.SlowThreshold {
		l.zap.Warn("slow query detected",
			zap.Duration("elapsed", elapsed),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
		)
	} else {
		l.zap.Debug("database query",
			zap.Duration("elapsed", elapsed),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
		)
	}
}
