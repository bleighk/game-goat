// cross_domain.go — shared adaptive rate-limited HTTP policy for sibling
// source clients. Generic by design: a future keyless source
// (internal/source/<name>/) can embed an adaptiveDoer instead of
// re-implementing token-bucket pacing and 429 backoff.

package steam

import (
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
// other 4xx statuses are never retried. A single backoff sleep is capped so
// a hostile Retry-After cannot pin the CLI.
const (
	maxRetryWait     = 10 * time.Second
	defaultRetryWait = 2 * time.Second
	// maxBodyBytes caps how much of a response body is read into memory.
	maxBodyBytes = 8 << 20
)

// adaptiveDoer paces outbound GETs with cliutil.AdaptiveLimiter (adaptive
// ceiling discovery) plus a small token-bucket burst, and retries at most
// once on HTTP 429/5xx. Exhausted 429 retries surface as
// *cliutil.RateLimitError so downstream commands never mistake a throttle
// for "no data exists".
type adaptiveDoer struct {
	http      *http.Client
	adaptive  *cliutil.AdaptiveLimiter
	retryWait time.Duration

	mu        sync.Mutex
	burstLeft int
}

func newAdaptiveDoer(httpClient *http.Client, ratePerSec float64, burst int) *adaptiveDoer {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &adaptiveDoer{
		http:      httpClient,
		adaptive:  cliutil.NewAdaptiveLimiter(ratePerSec),
		retryWait: defaultRetryWait,
		burstLeft: burst,
	}
}

// takeBurst consumes one burst token; false means the caller must pace at
// the sustained rate via the adaptive limiter.
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
	if wait := fallback; wait > maxRetryWait {
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

// get performs a policy-bound GET and returns the fully-read body. At most
// one retry on 429/5xx; network errors and other 4xx statuses fail
// immediately with typed errors.
func (d *adaptiveDoer) get(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if !d.takeBurst() {
			if err := d.adaptive.Wait(ctx); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
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
