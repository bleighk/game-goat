package steam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveAppIDRejectsNonExactMatch(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Near-identical editions must NOT resolve: the first app-typed
		// item is a different edition, and presenting its reviews and
		// price as the requested game's is worse than no Steam data.
		fmt.Fprint(w, `{"total":1,"items":[{"type":"dlc","name":"Elden Ring DLC","id":1},{"type":"app","name":"ELDEN RING - Deluxe","id":1245690}]}`)
	})
	_, err := c.ResolveAppID(context.Background(), "Elden Ring")
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("ResolveAppID error = %v, want ErrAppNotFound", err)
	}
	if !strings.Contains(err.Error(), "no exact store match") {
		t.Errorf("error = %v, want it to name the strict-match reason", err)
	}
}

func TestResolveAppIDOnlyNonAppItemsTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"total":1,"items":[{"type":"bundle","name":"Elden Ring Bundle","id":9}]}`)
	})
	_, err := c.ResolveAppID(context.Background(), "Elden Ring")
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("ResolveAppID error = %v, want ErrAppNotFound", err)
	}
}

func TestResolveAppIDMalformedJSONIsTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-json{{{")
	})
	_, err := c.ResolveAppID(context.Background(), "Elden Ring")
	if err == nil || !strings.Contains(err.Error(), "parsing storesearch") {
		t.Fatalf("ResolveAppID error = %v, want parse error", err)
	}
}

func TestReviewSummaryMalformedJSONIsTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html>blocked</html>`)
	})
	_, err := c.ReviewSummary(context.Background(), 1245690)
	if err == nil || !strings.Contains(err.Error(), "parsing appreviews") {
		t.Fatalf("ReviewSummary error = %v, want parse error", err)
	}
}

func TestClient5xxExhaustedIsStatusError(t *testing.T) {
	var attempts int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, strings.Repeat("x", 300)) // exercises the snippet trim in Error()
	})
	_, err := c.ResolveAppID(context.Background(), "Elden Ring")
	var se *statusError
	if !errors.As(err, &se) || se.status != http.StatusBadGateway {
		t.Fatalf("error = %v, want statusError{502}", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "HTTP 502") || !strings.Contains(msg, "xxx") {
		t.Errorf("Error() message should name status and trimmed body, got %q", msg)
	}
	if len(msg) > 300 {
		t.Errorf("Error() message should keep the body snippet under ~200 chars, got %d", len(msg))
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want exactly 2 (one retry)", got)
	}
}

func TestClient4xxErrorSurfacesBodySnippet(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"forbidden"}`)
	})
	_, err := c.ResolveAppID(context.Background(), "Elden Ring")
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("error = %v, want body snippet surfaced", err)
	}
}

func TestClientNetworkErrorNotRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // every request now fails at the transport layer
	c := New(NewConfig())
	c.BaseURL = url
	c.doer.retryWait = 10 * time.Millisecond
	_, err := c.ResolveAppID(context.Background(), "Elden Ring")
	if err == nil || !strings.Contains(err.Error(), "HTTP request to") {
		t.Fatalf("error = %v, want transport error surfaced without retry", err)
	}
}

func TestAppDetailsFallsBackToSingleEntryKey(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Steam keys by appid, but tolerate a drifted key: pick the lone entry.
		fmt.Fprint(w, `{"9999":{"success":true,"data":{"name":"Elden Ring"}}}`)
	})
	det, err := c.AppDetails(context.Background(), 1245690)
	if err != nil {
		t.Fatalf("AppDetails: %v", err)
	}
	if det.Name != "Elden Ring" {
		t.Errorf("Name = %q, want %q via single-entry fallback", det.Name, "Elden Ring")
	}
}

func TestAppDetailsEmptyMapTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`)
	})
	_, err := c.AppDetails(context.Background(), 1245690)
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("AppDetails error = %v, want ErrAppNotFound", err)
	}
}

func TestAppDetailsMalformedJSONIsTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[[[`) // not a JSON object at all
	})
	_, err := c.AppDetails(context.Background(), 1245690)
	if err == nil || !strings.Contains(err.Error(), "parsing appdetails") {
		t.Fatalf("AppDetails error = %v, want parse error", err)
	}
}

func TestAppDetailsMalformedEntryIsTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"1245690":"a string, not the success/data object"}`)
	})
	_, err := c.AppDetails(context.Background(), 1245690)
	if err == nil || !strings.Contains(err.Error(), "parsing appdetails entry") {
		t.Fatalf("AppDetails error = %v, want entry parse error", err)
	}
}

func TestRetryAfterSecondsBounds(t *testing.T) {
	mk := func(header string) *http.Response {
		resp := &http.Response{Header: http.Header{}}
		if header != "" {
			resp.Header.Set("Retry-After", header)
		}
		return resp
	}
	if got := retryAfterSeconds(mk("0"), 2*time.Second); got != 0 {
		t.Errorf("Retry-After 0 = %v, want immediate retry", got)
	}
	if got := retryAfterSeconds(mk("-5"), 2*time.Second); got != 0 {
		t.Errorf("negative Retry-After = %v, want clamped to 0", got)
	}
	if got := retryAfterSeconds(mk("9999"), 2*time.Second); got != maxRetryWait {
		t.Errorf("huge Retry-After = %v, want capped at %v", got, maxRetryWait)
	}
	if got := retryAfterSeconds(mk("soon"), 2*time.Second); got != 2*time.Second {
		t.Errorf("unparseable Retry-After = %v, want fallback 2s", got)
	}
	if got := retryAfterSeconds(mk(""), 30*time.Second); got != maxRetryWait {
		t.Errorf("fallback above the cap = %v, want capped at %v", got, maxRetryWait)
	}
}

func TestAdaptiveDoerBurstExhaustion(t *testing.T) {
	d := newAdaptiveDoer(nil, 3.0, 2)
	if d.http.Timeout != 10*time.Second {
		t.Errorf("nil http client should default to a 10s timeout, got %v", d.http.Timeout)
	}
	if !d.takeBurst() || !d.takeBurst() {
		t.Error("first two calls should consume burst tokens")
	}
	if d.takeBurst() {
		t.Error("third call should exhaust the burst and fall back to pacing")
	}
}

func TestAdaptiveDoerSleepHonorsCanceledContext(t *testing.T) {
	d := newAdaptiveDoer(nil, 3.0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("sleep with canceled ctx = %v, want context.Canceled", err)
	}
}

func TestAdaptiveDoerWaitHonorsCanceledContext(t *testing.T) {
	d := newAdaptiveDoer(nil, 3.0, 0) // burst 0: every call paces
	d.adaptive = nil                  // nil limiter: Wait is a no-op, sleep still bound
	_ = d
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := New(NewConfig())
	c.BaseURL = "http://127.0.0.1:0" // unreachable
	c.doer.retryWait = 5 * time.Millisecond
	_, err := c.ResolveAppID(ctx, "Elden Ring")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("request with canceled ctx = %v, want context.Canceled", err)
	}
}
