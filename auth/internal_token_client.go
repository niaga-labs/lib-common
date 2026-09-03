package auth

import (
	"net/http"
	"time"
)

// Client-side half of the internal-token guard (NIAGA-114).
//
// The service clients that call /internal/* routes build requests in a dozen
// places each, mostly through http.Client.Post, with no shared helper to hang a
// header on. Setting the header in a RoundTripper covers every call a client
// makes -- including the ones somebody adds next year without reading this.

// InternalTokenTransport sets the internal service token on every request.
type InternalTokenTransport struct {
	// Base is the underlying transport. nil means http.DefaultTransport.
	Base http.RoundTripper

	// Token is sent as InternalTokenHeader. An empty token sets no header, so a
	// misconfigured caller gets a clear 401 from the callee rather than sending
	// an empty credential that might match an equally empty expectation.
	Token string
}

// RoundTrip implements http.RoundTripper.
func (t *InternalTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	if t.Token == "" {
		return base.RoundTrip(req)
	}

	// RoundTrip must not modify the request it is given.
	clone := req.Clone(req.Context())
	clone.Header.Set(InternalTokenHeader, t.Token)
	return base.RoundTrip(clone)
}

// NewInternalHTTPClient returns an http.Client that authenticates itself to the
// internal routes of other services.
func NewInternalHTTPClient(token string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &InternalTokenTransport{Token: token},
	}
}
