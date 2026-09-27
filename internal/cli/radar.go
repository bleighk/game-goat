// radar.go — hand-written Slice D novel command (top-level).
// pp:data-source auto — upcoming RAWG releases scored against a taste
// profile built from the local backlog (bounded live genre lookups).
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	radarTasteRowBudget = 10
	radarFetchPageSize  = 40
	radarWindowDays     = 90
)

// ----- pure helpers (unit-tested) -----

// radarTasteGenre is one genre sighting while building the taste profile.
type radarTasteGenre struct {
	Name  string
	Count int
}

// topTasteGenres ranks genres by how many profiled backlog games carried them.
func topTasteGenres(counts map[string]int, topN int) []string {
	type kv struct {
		name  string
		count int
	}
	list := make([]kv, 0, len(counts))
	for name, count := range counts {
		list = append(list, kv{name, count})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].count != list[j].count {
			return list[i].count > list[j].count
		}
		return list[i].name < list[j].name
	})
	if topN > 0 && len(list) > topN {
		list = list[:topN]
	}
	names := make([]string, 0, len(list))
	for _, g := range list {
		names = append(names, g.name)
	}
	return names
}

// tasteOverlap counts how many of the taste profile's genres a game shares.
func tasteOverlap(gameGenres []string, profile []string) int {
	set := make(map[string]bool, len(profile))
	for _, p := range profile {
		set[strings.ToLower(p)] = true
	}
	n := 0
	for _, g := range gameGenres {
		if set[strings.ToLower(g)] {
			n++
		}
	}
	return n
}

// tasteScore ranks a candidate: each shared genre is worth 10 points, RAWG
// rating breaks up the rest. Higher is better.
func tasteScore(overlap int, rating float64) float64 {
	return float64(overlap)*10 + rating
}

// radarReason names why a release is on the radar.
func radarReason(gameGenres []string, profile []string) string {
	overlap := tasteOverlap(gameGenres, profile)
	if overlap == 0 || len(profile) == 0 {
		return ""
	}
	shared := make([]string, 0, overlap)
	set := make(map[string]bool, len(profile))
	for _, p := range profile {
		set[strings.ToLower(p)] = true
	}
	for _, g := range gameGenres {
		if set[strings.ToLower(g)] && !containsStr(shared, g) {
			shared = append(shared, g)
		}
	}
	return fmt.Sprintf("matches your %s taste (%d/%d backlog genres)", strings.Join(shared, "/"), overlap, len(profile))
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// ----- view -----

type radarPick struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Released  string   `json:"released,omitempty"`
	Rating    float64  `json:"rating"`
	Genres    []string `json:"genres"`
	OnBacklog bool     `json:"on_backlog"`
	Reason    string   `json:"reason,omitempty"`
}

type radarMeta struct {
	Source      string   `json:"source"`
	Taste       string   `json:"taste"` // available | unavailable
	TasteGenres []string `json:"taste_genres,omitempty"`
	TasteRows   int      `json:"taste_rows"`
	WindowDays  int      `json:"window_days"`
	Limit       int      `json:"limit"`
	Count       int      `json:"count"`
	Note        string   `json:"note,omitempty"`
}

type radarView struct {
	Meta    radarMeta   `json:"meta"`
	Results []radarPick `json:"results"`
}

func newNovelRadarCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "radar",
		Short: "Upcoming releases that match your backlog's taste",
		Long: `Watch the next 90 days of releases through your own taste: the top
genres across your most recent backlog rows (bounded live RAWG lookups, at
most 10 rows) become a taste profile, then upcoming games are scored by
genre overlap with it plus RAWG rating. Each pick carries a reason like
"matches your RPG taste (2/3 backlog genres)" and an on_backlog flag.

With an empty backlog, no API key, or unreachable API the taste profile
degrades to "unavailable" and picks keep RAWG's plain -added order with no
reasons. For the unfiltered release list use: game-goat-pp-cli games upcoming`,
		Example: strings.Trim(`
  game-goat-pp-cli radar
  game-goat-pp-cli radar --limit 5 --json
  game-goat-pp-cli radar --limit 10 --json --select results.name,results.reason
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "--limit=5;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "radar")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "radar takes no positional arguments; use --limit to bound results")
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > 20 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <1-20>", "--limit must be between 1 and 20")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Taste profile: top genres across up to 10 most recent backlog rows.
			profile := []string{}
			tasteRows := 0
			tasteNote := ""
			if rows, rerr := allBacklogRows(ctx); rerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog for taste profile: %v\n", rerr)
			} else if len(rows) > 0 {
				counts := map[string]int{}
				budget := radarTasteRowBudget
				for _, r := range rows { // rows are newest-added first
					if budget <= 0 {
						break
					}
					if r.Title == "" {
						continue
					}
					budget--
					tasteRows++
					exact, _, _, gerr := resolveExactTitleMatches(ctx, cmd, c, flags, r.Title, "")
					if gerr != nil || len(exact) == 0 {
						continue
					}
					for _, g := range refNames(exact[0].Genres) {
						counts[g]++
					}
				}
				profile = topTasteGenres(counts, 3)
			}
			if len(profile) == 0 && tasteRows > 0 {
				tasteNote = "could not resolve genre data for recent backlog rows; falling back to RAWG's -added ordering"
			}

			today := time.Now()
			params := map[string]string{
				"ordering":  "-added",
				"page_size": strconv.Itoa(radarFetchPageSize),
				"dates":     fmt.Sprintf("%s,%s", today.Format("2006-01-02"), today.AddDate(0, 0, radarWindowDays).Format("2006-01-02")),
			}
			games, source, err := fetchGamesResults(ctx, cmd, c, flags, "auto", params)
			if err != nil {
				return err
			}
			if source == "" {
				source = "live"
			}

			backlogIDs, _, bidErr := backlogGameIDs(ctx)
			if bidErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog for on-backlog flags: %v\n", bidErr)
			}
			picks := make([]radarPick, 0, len(games))
			for _, g := range games {
				genres := refNames(g.Genres)
				picks = append(picks, radarPick{
					ID: g.ID, Name: g.Name, Released: g.Released, Rating: g.Rating,
					Genres:    genres,
					OnBacklog: backlogIDs[g.ID],
					Reason:    radarReason(genres, profile),
				})
			}
			if len(profile) > 0 {
				sort.SliceStable(picks, func(i, j int) bool {
					return tasteScore(tasteOverlap(picks[i].Genres, profile), picks[i].Rating) >
						tasteScore(tasteOverlap(picks[j].Genres, profile), picks[j].Rating)
				})
			}
			if len(picks) > limit {
				picks = picks[:limit]
			}

			taste := "unavailable"
			if len(profile) > 0 {
				taste = "available"
			} else if tasteRows == 0 {
				tasteNote = "no backlog rows to build a taste profile; add games with: game-goat-pp-cli backlog add 'Hollow Knight'"
			}
			view := radarView{
				Meta: radarMeta{
					Source: source, Taste: taste, TasteGenres: profile,
					TasteRows: tasteRows, WindowDays: radarWindowDays,
					Limit: limit, Count: len(picks), Note: tasteNote,
				},
				Results: picks,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(picks) == 0 {
				fmt.Fprintln(w, "No releases in the next 90 days. Try again later or browse: game-goat-pp-cli games upcoming")
				return nil
			}
			items := make([]map[string]any, 0, len(picks))
			for _, p := range picks {
				rating := ""
				if p.Rating > 0 {
					rating = fmt.Sprintf("%.1f", p.Rating)
				}
				items = append(items, map[string]any{
					"release": p.Name,
					"date":    orDash(p.Released),
					"rating":  rating,
					"reason":  p.Reason,
				})
			}
			return printAutoTable(w, items)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 5, "number of picks to show (1-20)")
	return cmd
}
