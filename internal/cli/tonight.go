// tonight.go — hand-written Slice D top-level novel command.
// pp:data-source auto — live RAWG mood-recipe picks cross-checked against
// the local game_backlog (in-progress rows lead the results).
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"game-goat-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

const (
	tonightDefaultTimeMinutes = 60
	tonightFetchPageSize      = 20
	tonightMinLimit           = 1
	tonightMaxLimit           = 10
)

// tonightPick is one pick (or resume) in tonight's results.
type tonightPick struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Released  string   `json:"released,omitempty"`
	Rating    float64  `json:"rating"`
	Playtime  int      `json:"playtime"`
	Genres    []string `json:"genres"`
	OnBacklog bool     `json:"on_backlog"`
	Reason    string   `json:"reason,omitempty"`
}

type tonightMeta struct {
	Source      string `json:"source"`
	Mood        string `json:"mood"`
	TimeMinutes int    `json:"time_minutes"`
	Limit       int    `json:"limit"`
	Seed        int64  `json:"seed"`
	Count       int    `json:"count"`
	Note        string `json:"note,omitempty"`
}

type tonightView struct {
	Meta    tonightMeta   `json:"meta"`
	Results []tonightPick `json:"results"`
}

// fitsTimeBudget reports whether a RAWG average playtime (hours) fits a
// session budget of timeMinutes: the average completion in minutes must
// land within 1.5x the budget so a slow finish is still plausible tonight.
// RAWG playtime is average hours, so convert before comparing. Playtime 0
// means RAWG has no average — an unknown duration cannot be verified to
// fit the session, so it is excluded rather than treated as "very short".
func fitsTimeBudget(playtimeHours, timeMinutes int) bool {
	if playtimeHours <= 0 {
		return false
	}
	return playtimeHours*60 <= timeMinutes*3/2
}

// filterByPlaytimeBand keeps games whose RAWG average playtime fits the
// mood's band (minutes). Zero bounds are unbounded. Playtime 0 (RAWG
// unknown) never fits a band — an unmeasured duration cannot sit inside a
// playtime band.
func filterByPlaytimeBand(games []rawgGame, minMinutes, maxMinutes int) []rawgGame {
	out := make([]rawgGame, 0, len(games))
	for _, g := range games {
		if g.Playtime <= 0 {
			continue
		}
		minutes := g.Playtime * 60
		if minMinutes > 0 && minutes < minMinutes {
			continue
		}
		if maxMinutes > 0 && minutes > maxMinutes {
			continue
		}
		out = append(out, g)
	}
	return out
}

// rankTonightPicks orders picks so the in-progress backlog lead comes
// first, then live picks already on the backlog, then the rest (RAWG's
// rating order is preserved inside each tier), bounded to limit.
func rankTonightPicks(lead *tonightPick, picks []tonightPick, limit int) []tonightPick {
	out := make([]tonightPick, 0, len(picks)+1)
	if lead != nil {
		out = append(out, *lead)
	}
	onBacklog := make([]tonightPick, 0, len(picks))
	rest := make([]tonightPick, 0, len(picks))
	for _, p := range picks {
		if p.OnBacklog {
			onBacklog = append(onBacklog, p)
		} else {
			rest = append(rest, p)
		}
	}
	out = append(out, onBacklog...)
	out = append(out, rest...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func newTonightCmd(flags *rootFlags) *cobra.Command {
	var mood string
	var timeMinutes, limit int
	var maxHours float64
	var seed int64

	cmd := &cobra.Command{
		Use:   "tonight",
		Short: "Pick what to play tonight, by mood and session length",
		Long: `Pick tonight's game. The mood resolves to a RAWG recipe (see 'moods list'),
results come back ordered by community rating, and every pick is filtered
to fit your session: a game's RAWG average playtime (hours, converted to
minutes) must land within 1.5x your --time budget and inside the mood's
typical playtime band.

Your local backlog participates: any pick already on it is flagged and
ranked first (reason: already on your backlog — start tonight), and if the
store has an in-progress row it leads the results (reason: pick up where
you left off).

With no --mood, a surprise-me mood is picked deterministically from the
vocabulary via --seed. --time is minutes; --max-hours is the same budget
in hours (e.g. --max-hours 4).`,
		Example: strings.Trim(`
  game-goat-pp-cli tonight --mood cozy --time 90
  game-goat-pp-cli tonight --mood quick-hit --time 30 --json
  game-goat-pp-cli tonight --mood cozy --max-hours 4
  game-goat-pp-cli tonight --limit 5 --json --select results.name
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "--mood=quick-hit;--time=30;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tonight")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --mood <mood> --time <minutes>", "tonight takes flags, not positional arguments")
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}
			if mood != "" {
				if _, ok := moodByName(mood); !ok {
					return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --mood <mood>", fmt.Sprintf("unknown --mood %q; %s", mood, validMoodsHint()))
				}
			}
			timeChanged := cmd.Flags().Changed("time")
			hoursChanged := cmd.Flags().Changed("max-hours")
			if timeChanged && hoursChanged {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --time <minutes> | --max-hours <hours>", "pass --time (minutes) or --max-hours, not both")
			}
			if timeMinutes < 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --time <minutes>", "--time cannot be negative")
			}
			if maxHours < 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --max-hours <hours>", "--max-hours cannot be negative")
			}
			budget := tonightDefaultTimeMinutes
			if timeChanged {
				budget = timeMinutes
			} else if hoursChanged {
				budget = int(maxHours * 60)
			}
			if budget < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --time <minutes>", "the session budget must be at least 1 minute (--time or --max-hours)")
			}
			if limit < tonightMinLimit || limit > tonightMaxLimit {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+fmt.Sprintf(" --limit <%d-%d>", tonightMinLimit, tonightMaxLimit), fmt.Sprintf("--limit must be between %d and %d", tonightMinLimit, tonightMaxLimit))
			}

			chosenMood := mood
			var m moodEntry
			if mood == "" {
				m = pickSurpriseMood(seed)
				chosenMood = m.Name
			} else if entry, ok := moodByName(mood); ok {
				m = entry
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := moodRecipeParams(m, tonightFetchPageSize)
			params["ordering"] = "-rating"
			games, source, err := fetchGamesResults(ctx, cmd, c, flags, "auto", params)
			if err != nil {
				return err
			}
			if source == "" {
				source = "live"
			}

			// Client-side band + budget filter (RAWG playtime is average hours).
			band := filterByPlaytimeBand(games, m.PlaytimeMin, m.PlaytimeMax)
			budgeted := make([]rawgGame, 0, len(band))
			for _, g := range band {
				if fitsTimeBudget(g.Playtime, budget) {
					budgeted = append(budgeted, g)
				}
			}
			note := ""
			if len(budgeted) == 0 && m.PlaytimeMin > 0 {
				// The session budget sits below the mood's typical entry
				// length; keep only the upper bounds (mood max + budget).
				for _, g := range games {
					if fitsTimeBudget(g.Playtime, budget) && (m.PlaytimeMax == 0 || g.Playtime*60 <= m.PlaytimeMax) {
						budgeted = append(budgeted, g)
					}
				}
				if len(budgeted) > 0 {
					note = fmt.Sprintf("%s games usually need %d+ minutes; relaxed the mood's minimum to fit your %d-minute session", m.Name, m.PlaytimeMin, budget)
				}
			}

			// Backlog cross-check: ids + normalized titles, plus the resume lead.
			rows, err := allBacklogRows(ctx)
			if err != nil {
				return fmt.Errorf("reading local backlog: %w", err)
			}
			backlogIDs, backlogTitles := backlogKeySets(rows)
			var lead *tonightPick
			for _, r := range rows { // rows are newest-added first
				if r.Status == store.GameBacklogStatusInProgress {
					lead = &tonightPick{
						ID: r.RawgID, Name: r.Title, Playtime: int(r.Hours),
						Genres: []string{}, OnBacklog: true,
						Reason: "pick up where you left off",
					}
					break
				}
			}

			picks := make([]tonightPick, 0, len(budgeted))
			for _, g := range budgeted {
				if lead != nil && g.ID == lead.ID {
					continue // the resume lead already surfaced this game
				}
				p := tonightPick{
					ID: g.ID, Name: g.Name, Released: g.Released, Rating: g.Rating,
					Playtime: g.Playtime, Genres: refNames(g.Genres),
				}
				if backlogIDs[g.ID] || backlogTitles[normalizeGameTitle(g.Name)] {
					p.OnBacklog = true
					p.Reason = "already on your backlog — start tonight"
				}
				picks = append(picks, p)
			}

			results := rankTonightPicks(lead, picks, limit)
			view := tonightView{
				Meta: tonightMeta{
					Source: source, Mood: chosenMood, TimeMinutes: budget,
					Limit: limit, Seed: seed, Count: len(results), Note: note,
				},
				Results: results,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(w, "No %s picks fit a %d-minute session. Try a bigger --time, another mood (%s), or browse: game-goat-pp-cli discover\n", chosenMood, budget, strings.Join(moodNames(), ", "))
				return nil
			}
			items := make([]map[string]any, 0, len(results))
			for _, p := range results {
				playtime := ""
				if p.Playtime > 0 {
					playtime = strconv.Itoa(p.Playtime) + "h"
				}
				rating := ""
				if p.Rating > 0 {
					rating = fmt.Sprintf("%.1f", p.Rating)
				}
				items = append(items, map[string]any{
					"pick":     p.Name,
					"rating":   rating,
					"playtime": playtime,
					"reason":   p.Reason,
				})
			}
			return printAutoTable(w, items)
		},
	}

	cmd.Flags().StringVar(&mood, "mood", "", "mood to pick for (see 'moods list'); default: surprise-me via --seed")
	cmd.Flags().IntVar(&timeMinutes, "time", tonightDefaultTimeMinutes, "session budget in minutes; picks' RAWG average playtime must fit within 1.5x this")
	cmd.Flags().Float64Var(&maxHours, "max-hours", 0, "session budget in hours, e.g. --max-hours 4 (alternative to --time)")
	cmd.Flags().IntVar(&limit, "limit", 3, "maximum picks to return (1-10)")
	cmd.Flags().Int64Var(&seed, "seed", 0, "surprise-me seed: with no --mood, the mood is moods[seed mod count]")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTonightCmd(flags))
	})
}
