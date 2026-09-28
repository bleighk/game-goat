// config.go — IsThereAnyDeal source configuration.
//
// Unlike the keyless Steam source, ITAD is API-keyed: every endpoint this
// client calls needs a personal API key, sent as the ITAD-API-Key header.
// The config carries the key, the storefront country (which selects the
// currency ITAD returns prices in), and rate-limit policy.

package itad

const (
	// DefaultBaseURL is the IsThereAnyDeal API host every endpoint lives on.
	DefaultBaseURL = "https://api.isthereanydeal.com"
	// DefaultCountry is the ISO 3166-1 alpha-2 storefront region used when no
	// country is selected. ITAD returns prices in the region's local currency,
	// so this constant is the currency-localisation default.
	DefaultCountry = "US"
	// DefaultRateLimit is the sustained requests/sec the ITAD client paces at.
	DefaultRateLimit = 3.0
	// DefaultBurst is how many requests may fire back-to-back before the
	// sustained rate takes over (token-bucket capacity).
	DefaultBurst = 5
	// UserAgent identifies the CLI to the ITAD API.
	UserAgent = "game-goat-pp-cli/0.1.0 (+printing-press)"
)

// Config is the ITAD source configuration.
type Config struct {
	// APIKey is the IsThereAnyDeal API key. Empty yields ErrMissingAPIKey on
	// every call; commands surface that as a code-4 auth error with setup
	// guidance instead of an HTTP 403.
	APIKey string
	// Country is the ISO 3166-1 alpha-2 storefront region. Empty falls back to
	// DefaultCountry. It is the currency-localisation control.
	Country string
	// RateLimit is the sustained requests per second (token-bucket refill).
	RateLimit float64
}

// NewConfig returns the default ITAD configuration: US storefront, 3
// requests/sec sustained, burst 5, adaptive backoff on HTTP 429.
func NewConfig() *Config {
	return &Config{Country: DefaultCountry, RateLimit: DefaultRateLimit}
}
