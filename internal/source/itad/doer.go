// doer.go — adaptive rate-limited HTTP policy for the ITAD source client.
//
// Mirrors internal/source/steam's cross_domain.go policy (token-bucket burst
// + cliutil.AdaptiveLimiter pacing + 429/5xx backoff) but adds POST, which
// the ITAD prices and history-low endpoints require.

package itad

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil"
)

// Retry policy bounds: at most 1 retry per HTTP call, only on 429/5xx;
// other 4xx statuses are never retried. A backoff sleep is capped so a hostile
// Retry-After cannot pin the CLI.
const (
	maxRetryWait     = 10 * time.Second
	defaultRetryWait = 2 * time.Second
	// maxBodyBytes caps how much of a response body is read into memory.
	maxBodyBytes = 8 << 20
	// httpTimeout bounds every outbound ITAD request.
	httpTimeout = 10 * time.Second
)

// adaptiveDoer paces outbound requests with cliutil.AdaptiveLimiter plus a
// small token-bucket burst, and retries at most once on HTTP 429/5xx.
// Exhausted 429 retries surface as *cliutil.RateLimitError so downstream
// commands never mistake a throttle for "no price data exists".
type adaptiveDoer struct {
	http      *http.Client
	adaptive  *cliutil.AdaptiveLimiter
	retryWait time.Duration

	mu        sync.Mutex
	burstLeft int
}

func newAdaptiveDoer(httpClient *http.Client, ratePerSec float64, burst int) *adaptiveDoer {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: httpTimeout}
	}
	return &adaptiveDoer{
		http:      httpClient,
		adaptive:  cliutil.NewAdaptiveLimiter(ratePerSec),
		retryWait: defaultRetryWait,
		burstLeft: burst,
	}
}

// takeBurst consumes one burst token; false means the caller must pace at the
// sustained rate via the adaptive limiter.
func (d *adaptiveDoer) takeBurst() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.burstLeft > 0 {
		d.burstLeft--
		return true
	}
	return false
}

// retryAfterSeconds parses the Retry-After header (delta-seconds form);
// missing or unparseable yields the fallback wait. Waits are capped at
// maxRetryWait; a zero header means retry immediately.
func retryAfterSeconds(resp *http.Response, fallback time.Duration) time.Duration {
	header := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if secs, err := strconv.ParseInt(header, 10, 64); err == nil {
		if secs < 0 {
			secs = 0
		}
		wait := time.Duration(secs) * time.Second
		if wait > maxRetryWait {
			wait = maxRetryWait
		}
		return wait
	}
	if fallback > maxRetryWait {
		return maxRetryWait
	}
	return fallback
}

// sleep waits ctx-aware for the backoff window.
func (d *adaptiveDoer) sleep(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// statusError is a typed non-retryable HTTP status failure.
type statusError struct {
	status int
	url    string
	body   string
}

func (e *statusError) Error() string {
	msg := fmt.Sprintf("HTTP %d for %s", e.status, e.url)
	if snippet := strings.TrimSpace(e.body); snippet != "" {
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		msg += ": " + snippet
	}
	return msg
}

// do performs a policy-bound request and returns the fully-read body. At most
// one retry on 429/5xx; network errors and other 4xx statuses fail immediately
// with typed errors.
func (d *adaptiveDoer) do(ctx context.Context, method, rawURL string, headers map[string]string, payload []byte) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if !d.takeBurst() {
			if err := d.adaptive.Wait(ctx); err != nil {
				return nil, err
			}
		}
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := d.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request to %s: %w", rawURL, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		switch {
		case resp.StatusCode == http.StatusOK:
			d.adaptive.OnSuccess()
			return body, nil
		case resp.StatusCode == http.StatusTooManyRequests:
			d.adaptive.OnRateLimit()
			if attempt == 1 {
				return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: retryAfterSeconds(resp, d.retryWait)}
			}
			if serr := d.sleep(ctx, retryAfterSeconds(resp, d.retryWait)); serr != nil {
				return nil, serr
			}
		case resp.StatusCode >= 500:
			if attempt == 1 {
				return nil, &statusError{status: resp.StatusCode, url: rawURL, body: string(body)}
			}
			if serr := d.sleep(ctx, d.retryWait); serr != nil {
				return nil, serr
			}
		default: // other 4xx: never retried
			return nil, &statusError{status: resp.StatusCode, url: rawURL, body: string(body)}
		}
	}
	return nil, fmt.Errorf("HTTP request to %s: exhausted retries", rawURL)
}

func (d *adaptiveDoer) get(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	return d.do(ctx, http.MethodGet, rawURL, headers, nil)
}

func (d *adaptiveDoer) post(ctx context.Context, rawURL string, headers map[string]string, payload []byte) ([]byte, error) {
	return d.do(ctx, http.MethodPost, rawURL, headers, payload)
}
