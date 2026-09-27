// config.go — Steam source configuration.
//
// Steam is a KEYLESS source: every endpoint this client calls
// (storesearch, appreviews, appdetails) needs no credential, so the config
// carries rate-limit policy only — no credential fields by design.

package steam

const (
	// DefaultRateLimit is the sustained requests/sec the Steam client paces at.
	DefaultRateLimit = 3.0
	// DefaultBurst is how many requests may fire back-to-back before the
	// sustained rate takes over (token-bucket capacity).
	DefaultBurst = 5
)

// Config is the Steam source configuration. RateLimit is the only field:
// every Steam endpoint this client calls is keyless.
type Config struct {
	// RateLimit is the sustained requests per second (token-bucket refill).
	RateLimit float64
}

// NewConfig returns the default Steam configuration: 3 requests/sec
// sustained, burst 5, adaptive backoff on HTTP 429.
func NewConfig() *Config {
	return &Config{RateLimit: DefaultRateLimit}
}
