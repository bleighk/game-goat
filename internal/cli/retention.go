// retention.go — hand-written Slice D novel command (top-level).
// pp:data-source live — resolve one game, then read RAWG's unique
// added_by_status counts to compute beaten/dropped/playing/yet percentages
// and an aspirational-trap verdict.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// ----- pure helpers (unit-tested) -----

// retentionStats is the crowd-retention verdict derived from RAWG's
// added_by_status counts for one game.
type retentionStats struct {
	Total            int     `json:"total"` // sum of all added_by_status counts
	Beaten           int     `json:"beaten"`
	Dropped          int     `json:"dropped"`
	Playing          int     `json:"playing"`
	Yet              int     `json:"yet"`
	Owned            int     `json:"owned"`
	Toplay           int     `json:"toplay"`
	BeatenPct        float64 `json:"beaten_pct"`
	DroppedPct       float64 `json:"dropped_pct"`
	PlayingPct       float64 `json:"playing_pct"`
	YetPct           float64 `json:"yet_pct"`
	OwnedPct         float64 `json:"owned_pct"`
	ToplayPct        float64 `json:"toplay_pct"`
	AspirationalTrap bool    `json:"aspirational_trap"`
	Verdict          string  `json:"verdict"`
}

// computeRetentionStats turns RAWG added_by_status counts into the
// community's completion and drop verdict. The aspirational-trap flag fires
// when intent (yet + toplay) outweighs finishes (beaten): more people mean to
// play the game than have ever seen its credits. Pure: unit-tested without a
// network. Missing keys count as zero.
func computeRetentionStats(counts map[string]int) retentionStats {
	st := retentionStats{
		Beaten:  counts["beaten"],
		Dropped: counts["dropped"],
		Playing: counts["playing"],
		Yet:     counts["yet"],
		Owned:   counts["owned"],
		Toplay:  counts["toplay"],
	}
	for _, v := range counts {
		st.Total += v
	}
	if st.Total > 0 {
		st.BeatenPct = float64(st.Beaten) / float64(st.Total) * 100
		st.DroppedPct = float64(st.Dropped) / float64(st.Total) * 100
		st.PlayingPct = float64(st.Playing) / float64(st.Total) * 100
		st.YetPct = float64(st.Yet) / float64(st.Total) * 100
		st.OwnedPct = float64(st.Owned) / float64(st.Total) * 100
		st.ToplayPct = float64(st.Toplay) / float64(st.Total) * 100
	}
	st.AspirationalTrap = (st.Yet+st.Toplay) > st.Beaten && st.Total > 0
	switch {
	case st.Total == 0:
		st.Verdict = "no community signal yet"
	case st.AspirationalTrap:
		st.Verdict = "aspirational-trap"
	case st.BeatenPct >= 50:
		st.Verdict = "community-retained"
	default:
		st.Verdict = "mixed"
	}
	return st
}

// retentionStatusRows renders the counts as ordered (status, players, share)
// table rows; the interesting statuses come first.
func retentionStatusRows(st retentionStats) []map[string]any {
	type statusCount struct {
		name  string
		count int
		pct   float64
	}
	all := []statusCount{
		{"beaten", st.Beaten, st.BeatenPct},
		{"yet", st.Yet, st.YetPct},
		{"dropped", st.Dropped, st.DroppedPct},
		{"playing", st.Playing, st.PlayingPct},
		{"owned", st.Owned, st.OwnedPct},
		{"toplay", st.Toplay, st.ToplayPct},
	}
	rows := make([]map[string]any, 0, len(all))
	for _, s := range all {
		share := "-"
		if s.pct > 0 {
			share = fmt.Sprintf("%.1f%%", s.pct)
		}
		rows = append(rows, map[string]any{
			"status": s.name, "players": strconv.Itoa(s.count), "share": share,
		})
	}
	return rows
}

// ----- view -----

type retentionMeta struct {
	Source    string               `json:"source"`
	Game      string               `json:"game"`
	GameID    int                  `json:"game_id"`
	Added     int                  `json:"added"`
	Ambiguous []ambiguousCandidate `json:"ambiguous,omitempty"`
}

type retentionView struct {
	Meta  retentionMeta  `json:"meta"`
	Stats retentionStats `json:"stats"`
}

func newNovelRetentionCmd(flags *rootFlags) *cobra.Command {
	var year string

	cmd := &cobra.Command{
		Use:   "retention <game>",
		Short: "The community's completion and drop verdict on one game",
		Long: `Crowd retention: how a game actually holds the people who add it.
Resolves one game (title or RAWG id) and reads RAWG's unique
added_by_status counts — beaten / dropped / playing / yet / owned /
toplay — as percentages of everyone who added it, plus an
aspirational-trap flag: intent (yet + toplay) outweighing finishes
(beaten) means the community buys the idea of the game more than it
plays it.

Remake collisions on the title are flagged ambiguous; pin with --year.

For rating scores (RAWG/Metacritic/Steam) use 'ratings' instead.`,
		Example: strings.Trim(`
  game-goat-pp-cli retention "Elden Ring"
  game-goat-pp-cli retention "God of War" --year 2018 --json
  game-goat-pp-cli retention 3498 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "game=Elden Ring",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "retention")
			}
			if len(args) != 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <game> [--year <yyyy>]", "retention takes exactly one game (quoted title or RAWG id), e.g. retention \"Elden Ring\"")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <game> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2016")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var view retentionView
			if id, ok := parseGameID(args[0]); ok {
				raw, gerr := c.Get(ctx, "/games/"+strconv.Itoa(id), nil)
				if gerr != nil {
					return classifyAPIErrorOnly(gerr)
				}
				detail, perr := parseRetentionDetail(raw)
				if perr != nil {
					return perr
				}
				if detail.ID == 0 && detail.Name == "" {
					return notFoundErr(fmt.Errorf("no game with RAWG id %d; find ids with 'game-goat-pp-cli games search'", id))
				}
				view = retentionView{
					Meta:  retentionMeta{Source: "live", Game: detail.Name, GameID: detail.ID, Added: detail.Added},
					Stats: computeRetentionStats(detail.AddedByStatus),
				}
			} else {
				seed, candidates, rerr := resolveTitleForMultiSource(ctx, cmd, c, flags, args[0], year)
				if rerr != nil {
					return rerr
				}
				raw, gerr := c.Get(ctx, "/games/"+strconv.Itoa(seed.ID), nil)
				if gerr != nil {
					return classifyAPIErrorOnly(gerr)
				}
				detail, perr := parseRetentionDetail(raw)
				if perr != nil {
					return perr
				}
				view = retentionView{
					Meta:  retentionMeta{Source: "live", Game: seed.Name, GameID: seed.ID, Added: detail.Added, Ambiguous: candidates},
					Stats: computeRetentionStats(detail.AddedByStatus),
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			trap := "not an aspirational trap"
			if view.Stats.AspirationalTrap {
				trap = "ASPIRATIONAL TRAP"
			}
			fmt.Fprintf(w, "%s — community verdict: %s (%s)\n", view.Meta.Game, view.Stats.Verdict, trap)
			fmt.Fprintf(w, "based on %d community additions\n", view.Meta.Added)
			if err := printAutoTable(w, retentionStatusRows(view.Stats)); err != nil {
				return err
			}
			fmt.Fprintf(w, "verdict rule: intent (yet %d + toplay %d) vs finishes (beaten %d)\n", view.Stats.Yet, view.Stats.Toplay, view.Stats.Beaten)
			return nil
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "pin a remake collision to a 4-digit release year (e.g. 2018)")
	return cmd
}

// retentionDetail is the bounded slice of /games/{id} this command needs.
type retentionDetail struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	Added         int            `json:"added"`
	AddedByStatus map[string]int `json:"added_by_status"`
}

// parseRetentionDetail decodes the game-detail JSON into retentionDetail.
// Pure: unit-tested against JSON fixtures.
func parseRetentionDetail(raw []byte) (retentionDetail, error) {
	var d retentionDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return retentionDetail{}, fmt.Errorf("parsing RAWG game detail for retention: %w", err)
	}
	return d, nil
}

// parseGameID reports whether arg is a bare RAWG id (all digits).
func parseGameID(arg string) (int, bool) {
	if arg == "" {
		return 0, false
	}
	id, err := strconv.Atoi(arg)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
