package auth

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// APIKeyModel is the GORM model for API keys.
type APIKeyModel struct {
	ID          uint   `gorm:"primaryKey"`
	KeyID       string `gorm:"uniqueIndex;size:50;not null"`
	Name        string `gorm:"size:100;not null"`
	KeyHash     string `gorm:"uniqueIndex;size:64;not null"`
	KeyPrefix   string `gorm:"size:20;not null"`
	ServiceName string `gorm:"size:50;not null;index"`
	Scopes      string `gorm:"type:text"` // Comma-separated scopes
	RateLimit   int    `gorm:"default:1000"`
	ExpiresAt   *time.Time
	CreatedAt   time.Time `gorm:"not null"`
	LastUsedAt  *time.Time
	IsActive    bool `gorm:"default:true;index"`
}

func (APIKeyModel) TableName() string {
	return "api_keys"
}

// PostgresAPIKeyStore implements APIKeyStore using PostgreSQL.
type PostgresAPIKeyStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewPostgresAPIKeyStore creates a new PostgreSQL API key store.
func NewPostgresAPIKeyStore(db *gorm.DB, logger *zap.Logger) *PostgresAPIKeyStore {
	return &PostgresAPIKeyStore{
		db:     db,
		logger: logger,
	}
}

// AutoMigrate creates the API keys table if it doesn't exist.
func (s *PostgresAPIKeyStore) AutoMigrate() error {
	return s.db.AutoMigrate(&APIKeyModel{})
}

// Create stores a new API key.
func (s *PostgresAPIKeyStore) Create(ctx context.Context, key *APIKey) error {
	var expiresAt *time.Time
	if !key.ExpiresAt.IsZero() {
		expiresAt = &key.ExpiresAt
	}

	model := &APIKeyModel{
		KeyID:       key.ID,
		Name:        key.Name,
		KeyHash:     key.KeyHash,
		KeyPrefix:   key.KeyPrefix,
		ServiceName: key.ServiceName,
		Scopes:      joinScopes(key.Scopes),
		RateLimit:   key.RateLimit,
		ExpiresAt:   expiresAt,
		CreatedAt:   key.CreatedAt,
		IsActive:    key.IsActive,
	}

	return s.db.WithContext(ctx).Create(model).Error
}

// GetByID retrieves an API key by ID.
func (s *PostgresAPIKeyStore) GetByID(ctx context.Context, id string) (*APIKey, error) {
	var model APIKeyModel
	if err := s.db.WithContext(ctx).Where("key_id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return modelToAPIKey(&model), nil
}

// GetByHash retrieves an API key by its hash.
func (s *PostgresAPIKeyStore) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	var model APIKeyModel
	if err := s.db.WithContext(ctx).Where("key_hash = ? AND is_active = ?", hash, true).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return modelToAPIKey(&model), nil
}

// Update updates an API key.
func (s *PostgresAPIKeyStore) Update(ctx context.Context, key *APIKey) error {
	return s.db.WithContext(ctx).Model(&APIKeyModel{}).
		Where("key_id = ?", key.ID).
		Updates(map[string]interface{}{
			"name":       key.Name,
			"scopes":     joinScopes(key.Scopes),
			"rate_limit": key.RateLimit,
			"is_active":  key.IsActive,
		}).Error
}

// Delete removes an API key (soft delete by deactivating).
func (s *PostgresAPIKeyStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Model(&APIKeyModel{}).
		Where("key_id = ?", id).
		Update("is_active", false).Error
}

// ListByService retrieves all active API keys for a service.
func (s *PostgresAPIKeyStore) ListByService(ctx context.Context, serviceName string) ([]*APIKey, error) {
	var models []APIKeyModel
	if err := s.db.WithContext(ctx).
		Where("service_name = ? AND is_active = ?", serviceName, true).
		Find(&models).Error; err != nil {
		return nil, err
	}

	keys := make([]*APIKey, len(models))
	for i, m := range models {
		keys[i] = modelToAPIKey(&m)
	}
	return keys, nil
}

// UpdateLastUsed updates the last used timestamp.
func (s *PostgresAPIKeyStore) UpdateLastUsed(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Model(&APIKeyModel{}).
		Where("key_id = ?", id).
		Update("last_used_at", time.Now()).Error
}

// Helper functions
func modelToAPIKey(m *APIKeyModel) *APIKey {
	var expiresAt time.Time
	if m.ExpiresAt != nil {
		expiresAt = *m.ExpiresAt
	}

	var lastUsedAt time.Time
	if m.LastUsedAt != nil {
		lastUsedAt = *m.LastUsedAt
	}

	return &APIKey{
		ID:          m.KeyID,
		Name:        m.Name,
		KeyHash:     m.KeyHash,
		KeyPrefix:   m.KeyPrefix,
		ServiceName: m.ServiceName,
		Scopes:      splitScopes(m.Scopes),
		RateLimit:   m.RateLimit,
		ExpiresAt:   expiresAt,
		CreatedAt:   m.CreatedAt,
		LastUsedAt:  lastUsedAt,
		IsActive:    m.IsActive,
	}
}

func joinScopes(scopes []string) string {
	result := ""
	for i, s := range scopes {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

func splitScopes(s string) []string {
	if s == "" {
		return []string{}
	}
	scopes := []string{}
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				scopes = append(scopes, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		scopes = append(scopes, current)
	}
	return scopes
}

// CreateAPIKeyTable returns the SQL to create the API keys table.
func CreateAPIKeyTable() string {
	return `
CREATE TABLE IF NOT EXISTS api_keys (
    id BIGSERIAL PRIMARY KEY,
    key_id VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    service_name VARCHAR(50) NOT NULL,
    scopes TEXT,
    rate_limit INT DEFAULT 1000,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_api_keys_service ON api_keys (service_name);
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys (is_active);
`
}
