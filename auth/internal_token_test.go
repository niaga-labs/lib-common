package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// generatedToken stands in for `openssl rand -base64 32` output: 44 characters,
// which is what every .env.example tells the operator to produce.
const generatedToken = "Zm9vYmFyYmF6cXV4Y29ycmVjdGhvcnNlYmF0dGVyeQ==" // secret-scan: allow

func TestAConfiguredTokenIsUsedEverywhere(t *testing.T) {
	t.Setenv(InternalTokenEnvVar, "  "+generatedToken+"  ")
	for _, env := range []string{"development", "production"} {
		got, err := ResolveInternalToken(env)
		if err != nil {
			t.Fatalf("APP_ENV=%q: %v", env, err)
		}
		if got != generatedToken {
			t.Errorf("APP_ENV=%q: got %q — surrounding whitespace should be trimmed", env, got)
		}
	}
}

// NIAGA-216. The guard was written against the value the SERVICE .env.example
// files ship and knew nothing about the one infra-platform/.env.example ships,
// so the token an operator gets by copying that file and running docker compose
// up without editing it booted a production-mode stack and answered /health 200.
// The four cases below are the ticket's done-when.
func TestEveryPublishedPlaceholderIsRefusedOutsideDevelopment(t *testing.T) {
	refused := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"the service .env.example placeholder", DevInternalToken},
		{"the infra-platform .env.example placeholder", PlaceholderInternalToken},
		{"any other CHANGE_ME_ value", "CHANGE_ME_EXACTLY_32_BYTES_LONG_"}, // secret-scan: allow
		{"a lowercase change_me_ value", "change_me_generate_with_openssl_rand_base64_32"},
		{"a short hand-typed token", "internal"}, // secret-scan: allow
	}

	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(InternalTokenEnvVar, tc.token)
			if _, err := ResolveInternalToken("production"); err == nil {
				t.Errorf("%q was accepted in production", tc.token)
			}
		})
	}

	t.Run("a generated token still starts the service", func(t *testing.T) {
		t.Setenv(InternalTokenEnvVar, generatedToken)
		got, err := ResolveInternalToken("production")
		if err != nil {
			t.Fatalf("a generated token was refused: %v", err)
		}
		if got != generatedToken {
			t.Errorf("got %q, want the configured token", got)
		}
	})
}

// The refusals are for production only. A developer who has pasted the compose
// placeholder into a local .env must still be able to run the stack, because
// treating development as permissive is what NIAGA-210 deliberately kept.
func TestThePlaceholdersStillWorkInDevelopment(t *testing.T) {
	for _, token := range []string{DevInternalToken, PlaceholderInternalToken, "short"} { // secret-scan: allow
		t.Setenv(InternalTokenEnvVar, token)
		got, err := ResolveInternalToken("development")
		if err != nil {
			t.Errorf("development refused %q: %v", token, err)
		}
		if got != token {
			t.Errorf("development: got %q, want %q", got, token)
		}
	}
}

// The message has to name the value's problem, not just say no — an operator
// reading a boot panic needs to know which line of which file to edit.
func TestTheRefusalNamesTheProblem(t *testing.T) {
	t.Setenv(InternalTokenEnvVar, PlaceholderInternalToken)
	_, err := ResolveInternalToken("production")
	if err == nil {
		t.Fatal("the compose placeholder was accepted in production")
	}
	for _, want := range []string{InternalTokenEnvVar, PlaceholderPrefix, ".env.example"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}

	t.Setenv(InternalTokenEnvVar, "internal") // secret-scan: allow
	_, err = ResolveInternalToken("production")
	if err == nil {
		t.Fatal("an 8-character token was accepted in production")
	}
	if !strings.Contains(err.Error(), "openssl rand") {
		t.Errorf("the length refusal does not say how to generate one: %v", err)
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
