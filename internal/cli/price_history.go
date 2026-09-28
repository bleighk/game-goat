// price_history.go — hand-written novel command (top-level).
// pp:data-source live — IsThereAnyDeal historical price tracking: resolve one
// game, then read its all-time / 1-year / 3-month lows and dated price-change
// log, localised to the --country storefront currency, and return a buy-now
// verdict. RAWG exposes no price data at all, so this is the CLI's only
// historical-price surface.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/itad"

	"github.com/spf13/cobra"
)

type priceHistoryView struct {
	Meta    priceMeta        `json:"meta"`
	Current *priceDealRow    `json:"current,omitempty"`
	Lows    []priceLowRow    `json:"lows"`
	Changes []priceChangeRow `json:"changes"`
	Verdict string           `json:"verdict"`
}

// buildPriceHistoryView shapes the ITAD responses into the price-history view.
// Pure: unit-tested against fixtures.
func buildPriceHistoryView(game itad.Game, resolvedBy, country, since string, ambiguous []itadCandidate, prices []itad.PriceResult, history []itad.HistoryEntry) priceHistoryView {
	var hl itad.HistoryLow
	var deals []itad.Deal
	if len(prices) > 0 {
		hl = prices[0].HistoryLow
		deals = prices[0].Deals
	}
	lows := buildLowRows(hl)
	ordered := itad.SortDealsByPrice(deals)
	var current *priceDealRow
	// Only a deal with an actual price becomes "current"; a nil price is
	// missing data, not a free game.
	if len(ordered) > 0 && ordered[0].Price != nil {
		row := dealRow(ordered[0])
		current = &row
	}
	currency := ""
	if current != nil {
		currency = current.Currency
	}
	if currency == "" {
		currency = currencyFromLows(lows)
	}
	changes := buildChanges(history, 25)
	if currency == "" {
		currency = currencyFromChanges(changes)
	}
	return priceHistoryView{
		Meta: priceMeta{
			Source:     "live",
			Game:       game.Title,
			ItadID:     game.ID,
			Country:    country,
			Currency:   currency,
			ResolvedBy: resolvedBy,
			Since:      since,
			Ambiguous:  ambiguous,
		},
		Current: current,
		Lows:    lows,
		Changes: changes,
		Verdict: priceVerdict(current, lowestWindow(lows, "all")),
	}
}

func renderPriceHistory(cmd *cobra.Command, view priceHistoryView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — price history (%s, %s)\n", orDash(view.Meta.Game), orDash(view.Meta.Country), orDash(view.Meta.Currency))
	if view.Current != nil {
		fmt.Fprintf(w, "current best: %s at %s", formatMoney(view.Current.Amount, view.Current.Currency), orDash(view.Current.Shop))
		if view.Current.Cut > 0 {
			fmt.Fprintf(w, " (-%d%%)", view.Current.Cut)
		}
		fmt.Fprintln(w)
	}
	if len(view.Lows) > 0 {
		fmt.Fprintln(w, "historical lows:")
		rows := make([]map[string]any, 0, len(view.Lows))
		for _, l := range view.Lows {
			rows = append(rows, map[string]any{
				"window": l.Window,
				"low":    formatMoney(l.Amount, l.Currency),
			})
		}
		if err := printAutoTable(w, rows); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "verdict: %s\n", view.Verdict)
	if len(view.Changes) > 0 {
		fmt.Fprintf(w, "recent price changes (%d):\n", len(view.Changes))
		rows := make([]map[string]any, 0, len(view.Changes))
		for _, ch := range view.Changes {
			rows = append(rows, map[string]any{
				"at":    ch.At,
				"shop":  ch.Shop,
				"price": formatMoney(ch.Amount, ch.Currency),
				"cut":   fmt.Sprintf("%d%%", ch.Cut),
			})
		}
		if err := printAutoTable(w, rows); err != nil {
			return err
		}
	}
	return nil
}

func newPriceHistoryCmd(flags *rootFlags) *cobra.Command {
	var country string
	var since string

	cmd := &cobra.Command{
		Use:   "price-history <title>",
		Short: "Historical price tracking: all-time / 1-year / 3-month lows and a dated change log",
		Long: `Historical price tracking for one game from IsThereAnyDeal.

Resolves a title (or a bare ITAD id) and reports its all-time, 1-year, and
3-month lowest prices, the current best storefront price, a dated
price-change log, and a buy-now verdict comparing the current price against
the all-time low.

Prices are localised to the selected storefront country: --country takes an
ISO 3166-1 alpha-2 code (default: ITAD_COUNTRY or US) and ITAD returns every
amount in that region's currency. Requires an IsThereAnyDeal key: either the
ITAD_API_KEY environment variable or one stored with 'auth set-token --provider
itad'. Create a free key at https://isthereanydeal.com/apps/.

Remake collisions on the title are flagged ambiguous on stderr and in
meta.ambiguous; pass an ITAD id to pin one.`,
		Example: strings.Trim(`
  game-goat-pp-cli price-history "Elden Ring"
  game-goat-pp-cli price-history "Hades" --country GB --json
  game-goat-pp-cli price-history 018d937f-07fc-72ed-8517-d8e24cb1eb22 --json`, "\n"),
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
				return writeDryRun(cmd.OutOrStdout(), flags, "price-history")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--country <iso>]", "a game title or ITAD id is required")
			}
			resolvedCountry, cerr := resolveITADCountry(country)
			if cerr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --country <iso>", cerr.Error())
			}
			sinceValue, serr := normalizeSince(since)
			if serr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --since <date>", serr.Error())
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
			prices, perr := c.Prices(ctx, []string{game.ID}, itad.PriceOptions{Capacity: 1})
			if perr != nil {
				return classifyITADError(perr)
			}
			history, herr := c.History(ctx, game.ID, sinceValue)
			if herr != nil {
				return classifyITADError(herr)
			}

			view := buildPriceHistoryView(game, resolvedBy, resolvedCountry, sinceValue, ambiguous, prices, history)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderPriceHistory(cmd, view)
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 storefront region; selects the price currency (default ITAD_COUNTRY or US)")
	cmd.Flags().StringVar(&since, "since", "", "only load price changes after this date (YYYY-MM-DD or RFC3339; ITAD default is 3 months)")
	return cmd
}
