package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// NIAGA-114. The routes this guards reserve, deduct and restock stock, reserve
// flash-sale allocations, approve commissions and create marketplace orders.
// Before this middleware they checked nothing at all.

const testToken = "test-internal-token" // secret-scan: allow

func guardedRouter(expected string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/internal/stock/reserve", InternalToken(expected), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reserved": true})
	})
	return r
}

func call(r *gin.Engine, header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/stock/reserve", nil)
	if header != "" {
		req.Header.Set(InternalTokenHeader, header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestNoHeaderIsRefused(t *testing.T) {
	if got := call(guardedRouter(testToken), "").Code; got != http.StatusUnauthorized {
		t.Fatalf("a call with no %s got %d, want 401", InternalTokenHeader, got)
	}
}

func TestWrongTokenIsRefused(t *testing.T) {
	if got := call(guardedRouter(testToken), "not-the-token").Code; got != http.StatusUnauthorized {
		t.Fatalf("a wrong token got %d, want 401", got)
	}
}

func TestCorrectTokenPasses(t *testing.T) {
	if got := call(guardedRouter(testToken), testToken).Code; got != http.StatusOK {
		t.Fatalf("the correct token got %d, want 200", got)
	}
}

// The important one. If an unconfigured service accepted anything, failing to
// read the environment would silently open these routes to the world — which is
// how NIAGA-170 happened one repo over.
func TestAnUnconfiguredServiceAuthenticatesNobody(t *testing.T) {
	r := guardedRouter("")
	for _, header := range []string{"", "anything", DevInternalToken} {
		if got := call(r, header).Code; got != http.StatusUnauthorized {
			t.Errorf("with no configured token, header %q got %d, want 401", header, got)
		}
	}
}

// A prefix must not pass. Comparing with == would be correct here too, but the
// constant-time compare is what stops a timing oracle on a shared secret.
func TestAPrefixOfTheTokenIsRefused(t *testing.T) {
	r := guardedRouter(testToken)
	for _, header := range []string{"test", "test-internal-toke", testToken + "x"} {
		if got := call(r, header).Code; got != http.StatusUnauthorized {
			t.Errorf("header %q got %d, want 401", header, got)
		}
	}
}

func TestResolveFallsBackOnlyInDevelopment(t *testing.T) {
	t.Setenv(InternalTokenEnvVar, "")

	for _, env := range []string{"", "dev", "development", "local", "test"} {
		got, err := ResolveInternalToken(env)
		if err != nil {
			t.Errorf("APP_ENV=%q: unexpected error %v", env, err)
		}
		if got != DevInternalToken {
			t.Errorf("APP_ENV=%q: got %q, want the dev placeholder", env, got)
		}
	}

	for _, env := range []string{"production", "staging", "prod", "anything-else"} {
		if _, err := ResolveInternalToken(env); err == nil {
			t.Errorf("APP_ENV=%q: expected an error when %s is unset", env, InternalTokenEnvVar)
		}
	}
}

// Shipping the published placeholder to production is the mistake worth catching
// at boot: it is in every .env.example in the workspace.
func TestThePlaceholderIsRefusedOutsideDevelopment(t *testing.T) {
	t.Setenv(InternalTokenEnvVar, DevInternalToken)

	if _, err := ResolveInternalToken("production"); err == nil {
		t.Error("the development placeholder was accepted in production")
	}
	if got, err := ResolveInternalToken("development"); err != nil || got != DevInternalToken {
		t.Errorf("development: got %q, %v", got, err)
	}
}

func TestAConfiguredTokenIsUsedEverywhere(t *testing.T) {
	t.Setenv(InternalTokenEnvVar, "  a-real-token  ") // secret-scan: allow
	for _, env := range []string{"development", "production"} {
		got, err := ResolveInternalToken(env)
		if err != nil {
			t.Fatalf("APP_ENV=%q: %v", env, err)
		}
		if got != "a-real-token" {
			t.Errorf("APP_ENV=%q: got %q — surrounding whitespace should be trimmed", env, got)
		}
	}
}

func TestIsDevEnvTreatsTheUnknownAsProduction(t *testing.T) {
	for _, env := range []string{"", "dev", "DEV", " development ", "local", "test"} {
		if !IsDevEnv(env) {
			t.Errorf("IsDevEnv(%q) = false, want true", env)
		}
	}
	for _, env := range []string{"production", "staging", "uat", "whatever"} {
		if IsDevEnv(env) {
			t.Errorf("IsDevEnv(%q) = true; an unrecognised environment must count as production", env)
		}
	}
}
