// itad.go — typed IsThereAnyDeal price client.
//
// ITAD is API-keyed (header ITAD-API-Key). Three endpoints power the price
// commands, modeled on the published OpenAPI spec
// (https://docs.isthereanydeal.com/openapi.json):
//   - GET  /games/search/v1   (title -> game id)
//   - POST /games/prices/v3   (current deals + all/1y/3m historical lows)
//   - GET  /games/history/v2  (dated price-change log)
//
// The country query parameter is the currency-localisation control: ITAD
// returns every price in the selected ISO 3166-1 alpha-2 storefront region's
// local currency.

package itad

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Typed sentinel errors: callers translate these without string matching.
var (
	// ErrMissingAPIKey: no ITAD_API_KEY configured. Commands surface this as
	// an auth error with setup guidance rather than an HTTP 403.
	ErrMissingAPIKey = errors.New("itad: ITAD_API_KEY is not set")
	// ErrGameNotFound: the title matched no game in ITAD's catalog.
	ErrGameNotFound = errors.New("itad: no game matched the title")
)

// Game is one /games/search/v1 result.
type Game struct {
	ID     string `json:"id"` // ITAD UUID
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Type   string `json:"type"` // game | dlc | ...
	Mature bool   `json:"mature"`
}

// Money is an ITAD amount plus its ISO 4217 currency. The currency is chosen
// by the request's country parameter, which is the currency-localisation knob.
type Money struct {
	Amount    float64 `json:"amount"`
	AmountInt int     `json:"amountInt"`
	Currency  string  `json:"currency"`
}

// ShopRef identifies a storefront in a deal.
type ShopRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Deal is one current price at one shop.
type Deal struct {
	Shop      ShopRef `json:"shop"`
	Price     *Money  `json:"price"`
	Regular   *Money  `json:"regular"`
	Cut       int     `json:"cut"`
	StoreLow  *Money  `json:"storeLow,omitempty"`
	Voucher   *string `json:"voucher,omitempty"`
	Timestamp string  `json:"timestamp"`
	Expiry    *string `json:"expiry,omitempty"`
	URL       string  `json:"url"`
}

// HistoryLow is the all-time / 1-year / 3-month lowest price bundle returned
// by /games/prices/v3.
type HistoryLow struct {
	All *Money `json:"all"`
	Y1  *Money `json:"y1"`
	M3  *Money `json:"m3"`
}

// PriceResult is one game's entry in a /games/prices/v3 response.
type PriceResult struct {
	ID         string     `json:"id"`
	HistoryLow HistoryLow `json:"historyLow"`
	Deals      []Deal     `json:"deals"`
}

// DealDelta is the price portion of a history entry.
type DealDelta struct {
	Price   *Money `json:"price"`
	Regular *Money `json:"regular"`
	Cut     int    `json:"cut"`
}

// HistoryEntry is one dated price change from /games/history/v2.
type HistoryEntry struct {
	Timestamp string    `json:"timestamp"`
	Shop      ShopRef   `json:"shop"`
	Deal      DealDelta `json:"deal"`
}

// PriceOptions tunes a Prices call.
type PriceOptions struct {
	// DealsOnly omits prices with no active price cut.
	DealsOnly bool
	// Capacity caps how many prices to load per game (0 = no limit).
	Capacity int
}

// Client is the typed IsThereAnyDeal API client.
type Client struct {
	// BaseURL is overridable so tests can point at an httptest server.
	BaseURL string
	// APIKey is the ITAD personal API key.
	APIKey string
	// Country is the ISO 3166-1 alpha-2 storefront region.
	Country string
	doer    *adaptiveDoer
}

// New builds an ITAD client from cfg (nil falls back to NewConfig).
func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = NewConfig()
	}
	country := normalizeCountry(cfg.Country)
	if country == "" {
		country = DefaultCountry
	}
	rate := cfg.RateLimit
	if rate <= 0 {
		rate = DefaultRateLimit
	}
	return &Client{
		BaseURL: DefaultBaseURL,
		APIKey:  strings.TrimSpace(cfg.APIKey),
		Country: country,
		doer:    newAdaptiveDoer(nil, rate, DefaultBurst),
	}
}

func (c *Client) headers() map[string]string {
	h := map[string]string{
		"User-Agent": UserAgent,
		"Accept":     "application/json",
	}
	if c.APIKey != "" {
		h["ITAD-API-Key"] = c.APIKey
	}
	return h
}

func (c *Client) requireKey() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return ErrMissingAPIKey
	}
	return nil
}

func (c *Client) country() string {
	if country := normalizeCountry(c.Country); country != "" {
		return country
	}
	return DefaultCountry
}

// normalizeCountry upper-cases an ISO 3166-1 alpha-2 code, returning "" when
// the value is not exactly two ASCII letters.
func normalizeCountry(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 2 {
		return ""
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}
	return s
}

// normalizeTitle folds case, punctuation, and whitespace so a title search
// can prefer an exact match over a prefix-similar game.
func normalizeTitle(s string) string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.Join(fields, " ")
}

// Search maps a title to candidate games via /games/search/v1.
func (c *Client) Search(ctx context.Context, title string, results int) ([]Game, error) {
	if err := c.requireKey(); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("title", title)
	if results > 0 {
		q.Set("results", strconv.Itoa(results))
	}
	body, err := c.doer.get(ctx, c.BaseURL+"/games/search/v1?"+q.Encode(), c.headers())
	if err != nil {
		return nil, err
	}
	var games []Game
	if err := json.Unmarshal(body, &games); err != nil {
		return nil, fmt.Errorf("parsing ITAD search response: %w", err)
	}
	return games, nil
}

// Prices loads current deals and historical lows for the given game ids.
// The request country selects the currency of every returned amount.
func (c *Client) Prices(ctx context.Context, ids []string, opts PriceOptions) ([]PriceResult, error) {
	if err := c.requireKey(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("country", c.country())
	if opts.DealsOnly {
		q.Set("deals", "true")
	}
	if opts.Capacity > 0 {
		q.Set("capacity", strconv.Itoa(opts.Capacity))
	}
	payload, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	body, err := c.doer.post(ctx, c.BaseURL+"/games/prices/v3?"+q.Encode(), c.headers(), payload)
	if err != nil {
		return nil, err
	}
	var results []PriceResult
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("parsing ITAD prices response: %w", err)
	}
	return results, nil
}

// History loads the dated price-change log for one game. Since is an optional
// RFC 3339 timestamp or YYYY-MM-DD date; empty lets ITAD use its default
// (last 3 months).
func (c *Client) History(ctx context.Context, id, since string) ([]HistoryEntry, error) {
	if err := c.requireKey(); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("id", id)
	q.Set("country", c.country())
	if since != "" {
		q.Set("since", since)
	}
	body, err := c.doer.get(ctx, c.BaseURL+"/games/history/v2?"+q.Encode(), c.headers())
	if err != nil {
		return nil, err
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing ITAD history response: %w", err)
	}
	return entries, nil
}

// Info loads a game's basic record by ITAD id via /games/info/v2. Used by the
// commands' bare-id path so meta.game is populated instead of empty.
func (c *Client) Info(ctx context.Context, id string) (Game, error) {
	if err := c.requireKey(); err != nil {
		return Game{}, err
	}
	q := url.Values{}
	q.Set("id", id)
	body, err := c.doer.get(ctx, c.BaseURL+"/games/info/v2?"+q.Encode(), c.headers())
	if err != nil {
		return Game{}, err
	}
	var game Game
	if err := json.Unmarshal(body, &game); err != nil {
		return Game{}, fmt.Errorf("parsing ITAD info response: %w", err)
	}
	if game.ID == "" && game.Title == "" {
		return Game{}, ErrGameNotFound
	}
	return game, nil
}

// ResolveGame maps a title to one ITAD game and reports whether the match was
// exact. Exact normalized-title matches of type "game" win over DLC/soundtracks
// and fuzzy hits; when several exact "game" matches exist (remakes sharing a
// name) the best-ranked one is returned alongside the full candidate set so the
// caller can warn and let the user pin an ITAD id. When no exact match exists
// the first game-typed hit is returned with exact=false so the caller can warn
// instead of silently substituting a different game. Pure ranking: the only I/O
// is the search call.
func ResolveGame(ctx context.Context, c *Client, title string, results int) (Game, []Game, bool, error) {
	if results <= 0 {
		results = 20
	}
	games, err := c.Search(ctx, title, results)
	if err != nil {
		return Game{}, nil, false, err
	}
	if len(games) == 0 {
		return Game{}, nil, false, ErrGameNotFound
	}

	want := normalizeTitle(title)
	exactGames := make([]Game, 0, len(games))
	for _, g := range games {
		if normalizeTitle(g.Title) == want && strings.EqualFold(g.Type, "game") {
			exactGames = append(exactGames, g)
		}
	}
	if len(exactGames) > 0 {
		return exactGames[0], exactGames, true, nil
	}

	// No exact title: prefer the best-ranked game-typed hit, then any hit.
	// exact=false tells the caller to warn — this fallback is a guess.
	for _, g := range games {
		if strings.EqualFold(g.Type, "game") {
			return g, nil, false, nil
		}
	}
	return games[0], nil, false, nil
}

// SortDealsByPrice orders deals cheapest-first. Deals without a price sort
// last; ties keep their original order.
func SortDealsByPrice(deals []Deal) []Deal {
	out := make([]Deal, len(deals))
	copy(out, deals)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := out[i].Price, out[j].Price
		switch {
		case pi == nil && pj == nil:
			return false
		case pi == nil:
			return false
		case pj == nil:
			return true
		default:
			return pi.Amount < pj.Amount
		}
	})
	return out
}

// IsAuthError reports whether err is an ITAD HTTP 401/403 (a rejected or
// missing key), so commands can translate it into a code-4 auth error.
func IsAuthError(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.status == 401 || se.status == 403
	}
	return false
}

// IsNotFound reports whether err is an ITAD HTTP 404.
func IsNotFound(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.status == 404
	}
	return false
}
