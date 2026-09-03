package auth

import (
	"crypto/subtle"
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/niaga-labs/lib-common/response"
)

// Service-to-service authentication for the /internal/* routes.
//
// Those routes reserve, deduct and restock inventory, reserve flash-sale
// allocations, approve agent commissions and create marketplace orders. Until
// NIAGA-114 not one of them checked anything: service-inventory's block carried
// the comment "should be protected by internal network/service mesh in
// production" and service-order's said "No auth required - called by marketplace
// service". There is no service mesh, and nginx proxies these paths, so anyone
// who could reach the gateway could move stock or invent an order.
//
// This is deliberately a single shared token rather than the database-backed
// APIKeyMiddleware in this package (which no service uses): callers are our own
// services on a private network, and a token they can read from the environment
// is the smallest thing that closes the hole. Per-service keys with scopes are a
// later question, not a blocker for this one.

const (
	// InternalTokenHeader carries the shared service token.
	InternalTokenHeader = "X-Internal-Token"

	// InternalTokenEnvVar is where every service reads it from.
	InternalTokenEnvVar = "INTERNAL_API_TOKEN"

	// DevInternalToken is the value .env.example ships, and the only value this
	// package will invent for you. It is a placeholder by design and is refused
	// outside development.
	DevInternalToken = "dev-internal-token" // secret-scan: allow
)

// InternalToken returns middleware requiring InternalTokenHeader to equal
// expected, compared in constant time.
//
// An empty expected token authenticates nobody. That matters: if it accepted
// anything when unconfigured, a service that failed to read its environment
// would silently serve its internal routes to the world — the same class of
// failure as the nil RBAC middleware in NIAGA-170.
func InternalToken(expected string) gin.HandlerFunc {
	expectedBytes := []byte(expected)

	return func(c *gin.Context) {
		if len(expectedBytes) == 0 {
			response.Unauthorized(c, "Internal API is not configured on this service")
			c.Abort()
			return
		}

		presented := c.GetHeader(InternalTokenHeader)
		if presented == "" {
			response.Unauthorized(c, "Internal service token required")
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(presented), expectedBytes) != 1 {
			response.Unauthorized(c, "Invalid internal service token")
			c.Abort()
			return
		}

		c.Next()
	}
}

// IsDevEnv reports whether appEnv is one of the environments where falling back
// to DevInternalToken is acceptable. Anything unrecognised counts as production,
// because guessing wrong in that direction is the safe way to be wrong.
func IsDevEnv(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "", "dev", "development", "local", "test":
		return true
	default:
		return false
	}
}

// ResolveInternalToken reads INTERNAL_API_TOKEN.
//
// In development it falls back to DevInternalToken so a fresh clone runs with no
// setup. Anywhere else a missing or placeholder value is an error, and the
// caller is expected to refuse to start rather than serve these routes with a
// token anyone can read off GitHub.
func ResolveInternalToken(appEnv string) (string, error) {
	token := strings.TrimSpace(os.Getenv(InternalTokenEnvVar))

	if IsDevEnv(appEnv) {
		if token == "" {
			return DevInternalToken, nil
		}
		return token, nil
	}

	if token == "" {
		return "", fmt.Errorf("%s is required when APP_ENV=%q: the internal routes "+
			"reserve stock, approve commissions and create orders", InternalTokenEnvVar, appEnv)
	}
	if token == DevInternalToken {
		return "", fmt.Errorf("%s is set to the development placeholder while APP_ENV=%q; "+
			"that value is published in every .env.example", InternalTokenEnvVar, appEnv)
	}

	return token, nil
}

// MustResolveInternalToken is ResolveInternalToken for a main() that should not
// start without one. Failing loudly at boot beats serving stock movements to
// anyone who asks.
func MustResolveInternalToken(appEnv string) string {
	token, err := ResolveInternalToken(appEnv)
	if err != nil {
		panic("SECURITY: " + err.Error())
	}
	return token
}
