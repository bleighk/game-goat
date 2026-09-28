// price_query.go — shared helpers for the IsThereAnyDeal price commands
// (price-history, prices). Keeps title/id resolution, country selection,
// currency-localised row shaping, and the buy-now verdict in one place.
//
// pp:data-source live — every row is fetched from api.isthereanydeal.com.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/itad"

	"github.com/spf13/cobra"
)

// itadCandidate is one ambiguous IsThereAnyDeal match surfaced to the user.
type itadCandidate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// priceMeta is the provenance + currency envelope shared by the price views.
type priceMeta struct {
	Source     string          `json:"source"` // "live"
	Game       string          `json:"game"`
	ItadID     string          `json:"itad_id"`
	Country    string          `json:"country"`
	Currency   string          `json:"currency,omitempty"`
	ResolvedBy string          `json:"resolved_by"` // title | id
	Since      string          `json:"since,omitempty"`
	Ambiguous  []itadCandidate `json:"ambiguous,omitempty"`
}

// priceDealRow is one storefront's current price, in the selected currency.
type priceDealRow struct {
	Shop     string  `json:"shop"`
	Amount   float64 `json:"amount"`
	Regular  float64 `json:"regular,omitempty"`
	Cut      int     `json:"cut"`
	Currency string  `json:"currency"`
	StoreLow float64 `json:"store_low,omitempty"`
	URL      string  `json:"url,omitempty"`
	At       string  `json:"at,omitempty"`
}

// priceLowRow is one historical-low window (all-time, 1 year, 3 months).
type priceLowRow struct {
	Window   string  `json:"window"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// priceChangeRow is one dated entry in the price-change log.
type priceChangeRow struct {
	At       string  `json:"at"`
	Shop     string  `json:"shop"`
	Amount   float64 `json:"amount"`
	Regular  float64 `json:"regular,omitempty"`
	Cut      int     `json:"cut"`
	Currency string  `json:"currency"`
}

// normalizeITADCountry validates an ISO 3166-1 alpha-2 country code. Empty is
// not an error (the caller falls back to env/default).
func normalizeITADCountry(value string) (string, error) {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" {
		return "", nil
	}
	if len(v) != 2 {
		return "", fmt.Errorf("--country must be a 2-letter ISO 3166-1 code (e.g. US, GB, DE), got %q", value)
	}
	for _, r := range v {
		if r < 'A' || r > 'Z' {
			return "", fmt.Errorf("--country must be a 2-letter ISO 3166-1 code (e.g. US, GB, DE), got %q", value)
		}
	}
	return v, nil
}

// resolveITADCountry applies the currency-localisation precedence: explicit
// --country, then ITAD_COUNTRY, then the US default.
func resolveITADCountry(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return normalizeITADCountry(flagValue)
	}
	if env := cliutil.EnvOverride("ITAD_COUNTRY"); strings.TrimSpace(env) != "" {
		if c, err := normalizeITADCountry(env); err == nil && c != "" {
			return c, nil
		}
	}
	return itad.DefaultCountry, nil
}

// isHexRune reports whether r is an ASCII hex digit.
func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// parseITADID reports whether arg is a bare ITAD UUID (8-4-4-4-12 hex).
func parseITADID(arg string) (string, bool) {
	s := strings.TrimSpace(arg)
	if len(s) != 36 {
		return "", false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", false
			}
		default:
			if !isHexRune(r) {
				return "", false
			}
		}
	}
	return strings.ToLower(s), true
}

// normalizeSince accepts an empty value, a YYYY-MM-DD date, or an RFC3339
// timestamp and returns an RFC3339 string for the ITAD since parameter.
func normalizeSince(value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("--since must be a YYYY-MM-DD date or RFC3339 timestamp, e.g. 2024-01-01")
}

// PATCH(amend-2026-09-28: ITAD key resolves env then stored credential)
// resolveITADKey resolves the IsThereAnyDeal credential with the same
// precedence the primary API key uses: an explicit env override wins, then the
// stored credential in credentials.toml. Storing it once lets price commands
// run in every shell without a per-session export.
func resolveITADKey(flags *rootFlags) (string, error) {
	// An explicit env key must work even when the config file is malformed or
	// unreadable, so resolve it before loading config.
	if v := strings.TrimSpace(cliutil.EnvOverride("ITAD_API_KEY")); v != "" {
		return v, nil
	}
	configPath := ""
	if flags != nil {
		configPath = flags.configPath
	}
	// config.Load resolves GAME_GOAT_CONFIG/--config the same way every other
	// command does, so the key comes from the same home the rest of the run uses.
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", configErr(fmt.Errorf("loading config for the IsThereAnyDeal credential: %w", err))
	}
	return strings.TrimSpace(cfg.ITADApiKey), nil
}

// newITADClient builds the IsThereAnyDeal client for the resolved country.
// A missing key is a code-4 auth error with setup guidance, not an HTTP 403.
func newITADClient(flags *rootFlags, country string) (*itad.Client, error) {
	if err := validateDataSourceStrategy(flags, "live"); err != nil {
		return nil, usageErr(err)
	}
	key, err := resolveITADKey(flags)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, authErr(fmt.Errorf("no IsThereAnyDeal API key; create a free key at https://isthereanydeal.com/apps/ then either export ITAD_API_KEY=\"<your-key>\" or store it once with: echo \"$ITAD_API_KEY\" | game-goat-pp-cli auth set-token --provider itad"))
	}
	cfg := itad.NewConfig()
	cfg.APIKey = key
	cfg.Country = country
	if flags != nil && flags.rateLimit > 0 {
		cfg.RateLimit = flags.rateLimit
	}
	return itad.New(cfg), nil
}

// classifyITADError maps ITAD transport errors onto the CLI's exit-code
// classes: 401/403 -> auth (4), 404 -> not found (3).
func classifyITADError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, itad.ErrMissingAPIKey) || itad.IsAuthError(err) {
		return authErr(fmt.Errorf("%w; create a free key at https://isthereanydeal.com/apps/ then store it with: echo \"$ITAD_API_KEY\" | game-goat-pp-cli auth set-token --provider itad", err))
	}
	if itad.IsNotFound(err) {
		return notFoundErr(err)
	}
	return err
}

// resolveITADGame maps the positional argument (ITAD id or title) to a game,
// returns the ambiguity candidates, and reports how it resolved.
// itadTitleAcceptable reports whether a non-exact IsThereAnyDeal hit may stand
// in for the query. The candidate's core title (leading article dropped) must
// begin with the query at a word boundary: "witcher 3" and "the witcher 3" both
// accept "The Witcher 3: Wild Hunt", while an unrelated match such as "hunt",
// "elden ringg", or "__printing_press_invalid__" is reported not-found rather
// than silently showing another game's prices.
// PATCH(amend-2026-09-28: accept abbreviated ITAD titles, reject unrelated ones)
func itadTitleAcceptable(query, candidate string) bool {
	q := stripITADArticle(normalizeGameTitle(query))
	c := stripITADArticle(normalizeGameTitle(candidate))
	if q == "" || c == "" {
		return false
	}
	if q == c {
		return true
	}
	if !strings.HasPrefix(c, q) {
		return false
	}
	rest := c[len(q):]
	return strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, ":")
}

// stripITADArticle drops a leading article so "the witcher 3" and "witcher 3"
// both compare against a candidate's core title.
func stripITADArticle(s string) string {
	for _, a := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, a) {
			return s[len(a):]
		}
	}
	return s
}

func resolveITADGame(ctx context.Context, cmd *cobra.Command, c *itad.Client, title string) (itad.Game, []itadCandidate, string, error) {
	if id, ok := parseITADID(title); ok {
		game, err := c.Info(ctx, id)
		if err != nil {
			if errors.Is(err, itad.ErrGameNotFound) || itad.IsNotFound(err) {
				return itad.Game{}, nil, "", notFoundErr(fmt.Errorf("no IsThereAnyDeal game with id %s", id))
			}
			return itad.Game{}, nil, "", classifyITADError(err)
		}
		return game, nil, "id", nil
	}
	game, candidates, exact, err := itad.ResolveGame(ctx, c, title, 20)
	if err != nil {
		if errors.Is(err, itad.ErrGameNotFound) {
			return itad.Game{}, nil, "", notFoundErr(fmt.Errorf("no game titled %q in IsThereAnyDeal; try a fuller title or 'game-goat-pp-cli games search'", title))
		}
		return itad.Game{}, nil, "", classifyITADError(err)
	}
	var ambiguous []itadCandidate
	if len(candidates) > 1 {
		ambiguous = make([]itadCandidate, 0, len(candidates))
		for _, g := range candidates {
			ambiguous = append(ambiguous, itadCandidate{ID: g.ID, Title: g.Title, Type: g.Type})
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "matched %d games titled %q; using %s — pass an ITAD id to pin another\n", len(candidates), game.Title, game.ID)
	}
	if !exact {
		// A non-exact hit is only a safe substitute when it continues the query
		// at a word boundary (e.g. "the witcher 3" -> "The Witcher 3: Wild
		// Hunt"). Otherwise the top search hit can be an unrelated game, so
		// report not-found and let the caller pass an id or a fuller title
		// rather than showing another game's prices.
		// PATCH(amend-2026-09-28: ITAD fuzzy fallback requires a continuation)
		if !itadTitleAcceptable(title, game.Title) {
			return itad.Game{}, nil, "", notFoundErr(fmt.Errorf("no game titled %q in IsThereAnyDeal; closest match was %q — try a fuller title or pass an ITAD id", title, game.Title))
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "no exact title match for %q; using %q (%s) — pass an ITAD id to pin the exact game\n", title, game.Title, game.ID)
	}
	return game, ambiguous, "title", nil
}

// ----- row shaping -----

func dealRow(d itad.Deal) priceDealRow {
	row := priceDealRow{
		Shop: d.Shop.Name,
		Cut:  d.Cut,
		URL:  d.URL,
		At:   d.Timestamp,
	}
	if d.Price != nil {
		row.Amount = d.Price.Amount
		row.Currency = d.Price.Currency
	}
	if d.Regular != nil {
		row.Regular = d.Regular.Amount
	}
	if d.StoreLow != nil {
		row.StoreLow = d.StoreLow.Amount
	}
	return row
}

func lowRow(window string, m *itad.Money) (priceLowRow, bool) {
	if m == nil {
		return priceLowRow{}, false
	}
	// A zero amount is a real price of "free", not missing data; only nil is absent.
	return priceLowRow{Window: window, Amount: m.Amount, Currency: m.Currency}, true
}

// buildLowRows orders historical lows all-time -> 1 year -> 3 months.
func buildLowRows(hl itad.HistoryLow) []priceLowRow {
	rows := make([]priceLowRow, 0, 3)
	for _, e := range []struct {
		window string
		money  *itad.Money
	}{{"all", hl.All}, {"1y", hl.Y1}, {"3m", hl.M3}} {
		if r, ok := lowRow(e.window, e.money); ok {
			rows = append(rows, r)
		}
	}
	return rows
}

// buildChanges returns the newest N price changes.
func buildChanges(entries []itad.HistoryEntry, limit int) []priceChangeRow {
	rows := make([]priceChangeRow, 0, len(entries))
	for _, e := range entries {
		row := priceChangeRow{At: e.Timestamp, Shop: e.Shop.Name, Cut: e.Deal.Cut}
		if e.Deal.Price != nil {
			row.Amount = e.Deal.Price.Amount
			row.Currency = e.Deal.Price.Currency
		}
		if e.Deal.Regular != nil {
			row.Regular = e.Deal.Regular.Amount
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ti, oki := parseITADTime(rows[i].At)
		tj, okj := parseITADTime(rows[j].At)
		switch {
		case oki && okj:
			if ti.Equal(tj) {
				return false
			}
			return ti.After(tj)
		case oki != okj:
			// Parseable timestamps always sort before unparseable ones.
			return oki
		default:
			return rows[i].At > rows[j].At
		}
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func currencyFromLows(rows []priceLowRow) string {
	for _, r := range rows {
		if r.Currency != "" {
			return r.Currency
		}
	}
	return ""
}

func currencyFromChanges(rows []priceChangeRow) string {
	for _, r := range rows {
		if r.Currency != "" {
			return r.Currency
		}
	}
	return ""
}

func lowestWindow(rows []priceLowRow, window string) *priceLowRow {
	for i := range rows {
		if rows[i].Window == window {
			return &rows[i]
		}
	}
	return nil
}

// parseITADTime parses an ITAD RFC3339 timestamp. ok is false for an
// unparseable value, which sorts last.
func parseITADTime(value string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// priceVerdict compares the current best price against the all-time low. A zero
// amount is a free price, not missing data.
func priceVerdict(current *priceDealRow, allTime *priceLowRow) string {
	if current == nil {
		return "no current price"
	}
	if current.Amount <= 0 {
		return "free right now"
	}
	if allTime == nil {
		return "no historical low recorded"
	}
	if allTime.Amount <= 0 {
		return "above historical low (previously free)"
	}
	switch {
	case current.Amount <= allTime.Amount*1.0001:
		return "at historical low — cheapest it has ever been"
	case current.Amount <= allTime.Amount*1.10:
		return "near historical low (within 10%)"
	default:
		pct := (current.Amount/allTime.Amount - 1) * 100
		return fmt.Sprintf("above historical low (%.0f%% over the all-time low of %.2f %s)", pct, allTime.Amount, allTime.Currency)
	}
}

// formatMoney renders an amount with its currency for the human table.
func formatMoney(amount float64, currency string) string {
	if currency == "" {
		return fmt.Sprintf("%.2f", amount)
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
}
