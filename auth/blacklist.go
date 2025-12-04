package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklist manages revoked JWT tokens
type TokenBlacklist struct {
	redis *redis.Client
}

// NewTokenBlacklist creates a new token blacklist manager
func NewTokenBlacklist(redisClient *redis.Client) *TokenBlacklist {
	return &TokenBlacklist{
		redis: redisClient,
	}
}

// BlacklistToken adds a token to the blacklist
func (tb *TokenBlacklist) BlacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	// Calculate TTL - how long until token naturally expires
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		// Token already expired, no need to blacklist
		return nil
	}

	key := fmt.Sprintf("blacklist:token:%s", tokenID)

	// Set with expiration matching the token's expiration
	err := tb.redis.Set(ctx, key, "revoked", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	return nil
}

// IsTokenBlacklisted checks if a token is blacklisted
func (tb *TokenBlacklist) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := fmt.Sprintf("blacklist:token:%s", tokenID)

	exists, err := tb.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check token blacklist: %w", err)
	}

	return exists > 0, nil
}

// BlacklistRefreshTokenFamily blacklists all tokens in a refresh token family
// This is used when a refresh token is reused (potential security breach)
func (tb *TokenBlacklist) BlacklistRefreshTokenFamily(ctx context.Context, familyID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}

	key := fmt.Sprintf("blacklist:family:%s", familyID)

	err := tb.redis.Set(ctx, key, "revoked", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to blacklist token family: %w", err)
	}

	return nil
}

// IsRefreshTokenFamilyBlacklisted checks if a token family is blacklisted
func (tb *TokenBlacklist) IsRefreshTokenFamilyBlacklisted(ctx context.Context, familyID string) (bool, error) {
	key := fmt.Sprintf("blacklist:family:%s", familyID)

	exists, err := tb.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check family blacklist: %w", err)
	}

	return exists > 0, nil
}

// CleanupExpired removes expired blacklist entries (for testing/maintenance)
// Note: Redis automatically removes keys when they expire, so this is rarely needed
func (tb *TokenBlacklist) CleanupExpired(ctx context.Context) error {
	// Redis handles this automatically with TTL
	// This function exists for manual cleanup if needed
	return nil
}
