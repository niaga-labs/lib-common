package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTheTokenIsSentOnEveryRequest(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(InternalTokenHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewInternalHTTPClient("a-token", 5*time.Second) // secret-scan: allow

	// Post, not Do: the clients in service-order use the convenience methods,
	// which is exactly why the header goes on the transport and not the request.
	resp, err := client.Post(srv.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()

	if seen != "a-token" {
		t.Errorf("%s = %q, want the configured token", InternalTokenHeader, seen)
	}
}

func TestAnEmptyTokenSendsNoHeader(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header[http.CanonicalHeaderKey(InternalTokenHeader)]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := NewInternalHTTPClient("", time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()

	if present {
		t.Error("an unconfigured client sent the header; it should send nothing and take the 401")
	}
}

// RoundTrip must not mutate the request it is handed.
func TestTheOriginalRequestIsNotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	tr := &InternalTokenTransport{Token: "a-token"} // secret-scan: allow
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	resp.Body.Close()

	if got := req.Header.Get(InternalTokenHeader); got != "" {
		t.Errorf("the caller's request was mutated: %s = %q", InternalTokenHeader, got)
	}
}

// The client and the middleware must agree, or the guard passes nothing.
func TestTheClientSatisfiesTheMiddleware(t *testing.T) {
	const token = "shared-token" // secret-scan: allow

	gin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(InternalTokenHeader) != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer gin.Close()

	resp, err := NewInternalHTTPClient(token, time.Second).Get(gin.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("a client built with the same token got %d, want 200", resp.StatusCode)
	}
}
