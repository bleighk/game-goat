// prices.go — hand-written novel command (top-level).
// pp:data-source live — IsThereAnyDeal current prices across storefronts,
// cheapest first, localised to the --country currency. The storefront price
// list is RAWG's biggest blind spot; this surfaces it per region.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/itad"

	"github.com/spf13/cobra"
)

type pricesView struct {
	Meta       priceMeta      `json:"meta"`
	HistoryLow *priceLowRow   `json:"history_low,omitempty"`
	Results    []priceDealRow `json:"results"`
}

// buildPricesView shapes a /games/prices/v3 response into the prices view.
// Pure: unit-tested against fixtures.
func buildPricesView(game itad.Game, resolvedBy, country string, ambiguous []itadCandidate, prices []itad.PriceResult, limit int, dealsOnly bool) pricesView {
	var hl itad.HistoryLow
	var deals []itad.Deal
	if len(prices) > 0 {
		hl = prices[0].HistoryLow
		deals = prices[0].Deals
	}
	if dealsOnly {
		filtered := make([]itad.Deal, 0, len(deals))
		for _, d := range deals {
			if d.Cut > 0 {
				filtered = append(filtered, d)
			}
		}
		deals = filtered
	}
	ordered := itad.SortDealsByPrice(deals)
	rows := make([]priceDealRow, 0, len(ordered))
	for _, d := range ordered {
		if limit > 0 && len(rows) >= limit {
			break
		}
		rows = append(rows, dealRow(d))
	}
	currency := ""
	if len(rows) > 0 {
		currency = rows[0].Currency
	}
	lows := buildLowRows(hl)
	if currency == "" {
		currency = currencyFromLows(lows)
	}
	var low *priceLowRow
	if r, ok := lowRow("all", hl.All); ok {
		low = &r
	}
	return pricesView{
		Meta: priceMeta{
			Source:     "live",
			Game:       game.Title,
			ItadID:     game.ID,
			Country:    country,
			Currency:   currency,
			ResolvedBy: resolvedBy,
			Ambiguous:  ambiguous,
		},
		HistoryLow: low,
		Results:    rows,
	}
}

func renderPrices(cmd *cobra.Command, view pricesView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — prices across storefronts (%s, %s)\n", orDash(view.Meta.Game), orDash(view.Meta.Country), orDash(view.Meta.Currency))
	if len(view.Results) == 0 {
		fmt.Fprintln(w, "no storefront prices found")
		return nil
	}
	rows := make([]map[string]any, 0, len(view.Results))
	for _, r := range view.Results {
		cut := "-"
		if r.Cut > 0 {
			cut = fmt.Sprintf("%d%%", r.Cut)
		}
		rows = append(rows, map[string]any{
			"shop":  r.Shop,
			"price": formatMoney(r.Amount, r.Currency),
			"cut":   cut,
			"was":   formatMoney(r.Regular, r.Currency),
		})
	}
	if err := printAutoTable(w, rows); err != nil {
		return err
	}
	if view.HistoryLow != nil {
		fmt.Fprintf(w, "all-time low: %s\n", formatMoney(view.HistoryLow.Amount, view.HistoryLow.Currency))
	}
	return nil
}

func newPricesCmd(flags *rootFlags) *cobra.Command {
	var country string
	var limit int
	var dealsOnly bool

	cmd := &cobra.Command{
		Use:   "prices <title>",
		Short: "Current prices across storefronts, cheapest first, in the --country currency",
		Long: `Current storefront prices for one game from IsThereAnyDeal.

Resolves a title (or a bare ITAD id) and lists every storefront's current
price, cheapest first, plus the all-time low for context. Prices are
localised to the selected storefront country: --country takes an ISO 3166-1
alpha-2 code (default: ITAD_COUNTRY or US) and ITAD returns every amount in
that region's currency. Requires an IsThereAnyDeal key: either the ITAD_API_KEY
environment variable or one stored with 'auth set-token --provider itad'.
Create a free key at https://isthereanydeal.com/apps/.

Use --deals-only to show just storefronts with an active price cut, and
--limit to cap the row count. For the dated price history use
'price-history'.`,
		Example: strings.Trim(`
  game-goat-pp-cli prices "Elden Ring"
  game-goat-pp-cli prices "Elden Ring" --country DE --deals-only --json
  game-goat-pp-cli prices 018d937f-07fc-72ed-8517-d8e24cb1eb22 --limit 5`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Elden Ring;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "prices")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--country <iso>]", "a game title or ITAD id is required")
			}
			if limit < 1 || limit > 50 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --limit <1-50>", "--limit must be between 1 and 50")
			}
			resolvedCountry, cerr := resolveITADCountry(country)
			if cerr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --country <iso>", cerr.Error())
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := newITADClient(flags, resolvedCountry)
			if err != nil {
				return err
			}
			title := strings.Join(args, " ")
			game, ambiguous, resolvedBy, rerr := resolveITADGame(ctx, cmd, c, title)
			if rerr != nil {
				return rerr
			}
			prices, perr := c.Prices(ctx, []string{game.ID}, itad.PriceOptions{DealsOnly: dealsOnly})
			if perr != nil {
				return classifyITADError(perr)
			}

			view := buildPricesView(game, resolvedBy, resolvedCountry, ambiguous, prices, limit, dealsOnly)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderPrices(cmd, view)
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 storefront region; selects the price currency (default ITAD_COUNTRY or US)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum storefront prices to return (1-50)")
	cmd.Flags().BoolVar(&dealsOnly, "deals-only", false, "only show storefronts with an active price cut")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newPriceHistoryCmd(flags))
		addNovelCommandIfAbsent(root, newPricesCmd(flags))
	})
	whichIndex = append(whichIndex,
		whichEntry{Command: "price-history", Description: "Historical price tracking for one game: all-time / 1-year / 3-month lows plus a dated price-change log, with prices localised to a --country currency (needs ITAD_API_KEY)."},
		whichEntry{Command: "prices", Description: "Current prices across every storefront, cheapest first, localised to a --country currency (needs ITAD_API_KEY)."},
	)
}
