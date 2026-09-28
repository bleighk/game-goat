// series.go — hand-written Slice B novel command (top-level).
// pp:data-source live — RAWG game-series spine in release-date play order.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"

	"github.com/spf13/cobra"
)

type seriesEntry struct {
	Order    int     `json:"order"`
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Released string  `json:"released"`
	Year     string  `json:"year,omitempty"`
	Rating   float64 `json:"rating"`
	Playtime int     `json:"playtime"`
}

type seriesMeta struct {
	Source     string               `json:"source"`
	Title      string               `json:"title"`
	Year       string               `json:"year,omitempty"`
	ResolvedBy string               `json:"resolved_by"`
	Via        string               `json:"via"`
	Anchor     string               `json:"anchor,omitempty"`
	Count      int                  `json:"count"`
	Ambiguous  []ambiguousCandidate `json:"ambiguous,omitempty"`
	Note       string               `json:"note,omitempty"`
}

type seriesView struct {
	Meta    seriesMeta    `json:"meta"`
	Results []seriesEntry `json:"results"`
}

// fetchSeriesGames walks the RAWG series endpoints for one game id:
// /games/{id}/game-series first; an empty result falls back to
// /games/{id}/parent-games (remakes and editions); still empty yields an
// actionable note pointing at games search.
func fetchSeriesEndpoint(ctx context.Context, c *client.Client, gameID int, endpoint string) ([]rawgGame, error) {
	var all []rawgGame
	for page := 1; page <= 5; page++ {
		data, err := c.Get(ctx, fmt.Sprintf("/games/%d/%s", gameID, endpoint), map[string]string{
			"page_size": "40",
			"page":      strconv.Itoa(page),
		})
		if err != nil {
			if page == 1 {
				return nil, classifyAPIErrorOnly(err)
			}
			break
		}
		var paged struct {
			Count   int        `json:"count"`
			Results []rawgGame `json:"results"`
		}
		if jerr := json.Unmarshal(data, &paged); jerr != nil {
			if page == 1 {
				return nil, fmt.Errorf("parsing /games/%d/%s response: %w", gameID, endpoint, jerr)
			}
			break
		}
		all = append(all, paged.Results...)
		if len(paged.Results) < 40 || len(all) >= paged.Count {
			break
		}
	}
	seen := map[int]bool{}
	out := make([]rawgGame, 0, len(all))
	for _, g := range all {
		if g.ID != 0 && !seen[g.ID] {
			seen[g.ID] = true
			out = append(out, g)
		}
	}
	return out, nil
}

func fetchSeriesGames(ctx context.Context, c *client.Client, gameID int, title string) ([]rawgGame, string, string, error) {
	games, err := fetchSeriesEndpoint(ctx, c, gameID, "game-series")
	if err != nil {
		return nil, "", "", err
	}
	if len(games) > 0 {
		return games, "game-series", "", nil
	}
	parents, err := fetchSeriesEndpoint(ctx, c, gameID, "parent-games")
	if err != nil {
		return nil, "", "", err
	}
	if len(parents) > 0 {
		return parents, "parent-games", "no game-series data; showing parent games (remakes/editions of the same title)", nil
	}
	return nil, "game-series", fmt.Sprintf("no game-series or parent-games data for %q; find related games with 'game-goat-pp-cli games search %s'", title, title), nil
}

// seriesAnchor loads a resolved anchor's detail when the resolver returned an
// id-only record (a bare RAWG id), so the anchor carries a name and release
// date. A title-resolved anchor already has its fields and passes through
// without a network call.
func seriesAnchor(ctx context.Context, c *client.Client, match rawgGame) (rawgGame, error) {
	if match.Name != "" {
		return match, nil
	}
	detail, err := fetchGameByID(ctx, c, match.ID)
	if err != nil {
		return match, err
	}
	detail.ID = match.ID
	return detail, nil
}

func newSeriesCmd(flags *rootFlags) *cobra.Command {
	var year string
	var limit int

	cmd := &cobra.Command{
		Use:   "series <title>",
		Short: "Franchise play order from the RAWG game-series endpoint",
		Long: `Resolve a game by title, pull every game RAWG links to it via
/games/{id}/game-series, and print a numbered franchise play order sorted
by release date with year, rating, and playtime. The anchor game itself is
included. An empty game-series falls back to parent-games (remakes and
editions). Titles shared by remakes are flagged as ambiguous; pin with
--year. A bare-numeric argument is a RAWG id, matching retention and games
get.`,
		Example: strings.Trim(`
  game-goat-pp-cli series "Yakuza"
  game-goat-pp-cli series "God of War" --limit 8 --json
  game-goat-pp-cli series "Yakuza" --json --select results.name,results.order
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Yakuza;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "series")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--year <yyyy>]", "a game title is required")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2018")
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --limit <n>", "--limit must be at least 1")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			title := strings.Join(args, " ")
			match, candidates, err := resolveTitleForMultiSource(ctx, cmd, c, flags, title, year)
			if err != nil {
				return err
			}
			// A bare RAWG id resolves to an ID-only record. Load its detail so
			// the anchor row carries a name and release date and sorts into the
			// correct play-order position instead of appearing blank and last.
			// PATCH(amend-2026-09-28: series fetches details for a bare-id anchor)
			if anchor, aerr := seriesAnchor(ctx, c, match); aerr == nil {
				match = anchor
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not load details for RAWG id %d: %v\n", match.ID, aerr)
			}
			games, via, note, err := fetchSeriesGames(ctx, c, match.ID, title)
			if err != nil {
				return err
			}
			if len(games) == 0 {
				// Franchise shorthand ("zelda") often resolves to an obscure
				// same-named entry with no series links. Re-anchor once to the
				// best-rated game in the title's search results.
				alt, _, aerr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
					"search":    title,
					"page_size": "8",
				})
				if aerr == nil {
					best := rawgGame{}
					for _, g := range alt {
						if g.ID != match.ID && g.Rating > best.Rating {
							best = g
						}
					}
					if best.ID != 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "note: no series data on %q (%s); re-anchored to best-rated match %q (%s)\n",
							match.Name, orDash(yearOf(match.Released)), best.Name, orDash(yearOf(best.Released)))
						match = best
						games, via, note, err = fetchSeriesGames(ctx, c, match.ID, title)
						if err != nil {
							return err
						}
					}
				}
			}
			if via == "parent-games" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+note)
			} else if note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+note)
			}
			// The series endpoints list only *other* games; the anchor is
			// itself a franchise entry and belongs in the play order.
			games = append(games, match)
			dedupe := map[int]bool{}
			uniq := make([]rawgGame, 0, len(games))
			for _, g := range games {
				if !dedupe[g.ID] {
					dedupe[g.ID] = true
					uniq = append(uniq, g)
				}
			}
			games = uniq
			// Release-date ascending play order; undated entries sort last.
			sort.SliceStable(games, func(i, j int) bool {
				if games[i].Released == "" {
					return false
				}
				if games[j].Released == "" {
					return true
				}
				return games[i].Released < games[j].Released
			})
			total := len(games)
			if len(games) > limit {
				games = games[:limit]
			}
			entries := make([]seriesEntry, 0, len(games))
			for i, g := range games {
				entries = append(entries, seriesEntry{
					Order:    i + 1,
					ID:       g.ID,
					Name:     g.Name,
					Released: g.Released,
					Year:     yearOf(g.Released),
					Rating:   g.Rating,
					Playtime: g.Playtime,
				})
			}
			resolvedBy := "title"
			if _, ok := parseGameID(title); ok {
				resolvedBy = "id"
			}
			meta := seriesMeta{
				Source:     "live",
				Title:      title,
				ResolvedBy: resolvedBy,
				Via:        via,
				Anchor:     match.Name,
				Count:      len(entries),
			}
			if year != "" {
				meta.Year = year
			}
			if candidates != nil {
				meta.Ambiguous = candidates
			}
			if note != "" {
				meta.Note = note
			}
			if total > len(entries) {
				meta.Note = strings.TrimSpace(meta.Note + fmt.Sprintf(" showing the first %d of %d games; raise --limit for more", len(entries), total))
			}
			view := seriesView{Meta: meta, Results: entries}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No series data found for anchor %q (%s). Pass a franchise entry title, e.g. series \"The Legend of Zelda: Breath of the Wild\".\n",
					meta.Anchor, orDash(yearOf(match.Released)))
				return nil
			}
			// Fixed column order: the play-order number leads so the
			// date-sorted list reads sorted. (printAutoTable prioritizes
			// columns by field name and would bury the order column.)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, strings.Join([]string{
				bold("ORDER"), bold("NAME"), bold("YEAR"), bold("RATING"), bold("PLAYTIME"),
			}, "\t"))
			for _, e := range entries {
				fmt.Fprintln(tw, strings.Join([]string{
					strconv.Itoa(e.Order),
					e.Name,
					orDash(e.Year),
					fmt.Sprintf("%.1f", e.Rating),
					strconv.Itoa(e.Playtime) + "h",
				}, "\t"))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if meta.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", meta.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin the title to a release year when remakes share the name (e.g. 2018)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum series games to return in play order")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSeriesCmd(flags))
	})
}
