// finishline.go — hand-written Slice D novel command (top-level).
// pp:data-source local — a progress projection over the game_backlog table;
// no mutation happens here.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"game-goat-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// ----- pure projection helpers (unit-tested) -----

// activeBacklogRows returns status=backlog/in-progress rows, oldest added
// first — the queue of games still to finish.
func activeBacklogRows(rows []store.GameBacklogRow) []store.GameBacklogRow {
	active := make([]store.GameBacklogRow, 0, len(rows))
	for _, r := range rows {
		if r.Status == store.GameBacklogStatusBacklog || r.Status == store.GameBacklogStatusInProgress {
			active = append(active, r)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return active[i].AddedAt < active[j].AddedAt })
	return active
}

// monthsSinceEarliestAdd measures the age of the backlog in months from the
// earliest added_at to now; 0 when there are no rows or the timestamps are
// unparsable (the divide-by-zero guard lives at the call site).
func monthsSinceEarliestAdd(rows []store.GameBacklogRow, now time.Time) float64 {
	earliest := time.Time{}
	for _, r := range rows {
		if t, err := time.Parse(time.RFC3339, r.AddedAt); err == nil && (earliest.IsZero() || t.Before(earliest)) {
			earliest = t
		}
	}
	if earliest.IsZero() {
		return 0
	}
	return now.Sub(earliest).Hours() / 24 / 30.44
}

// finishlineStats summarizes backlog progress. FinishedPerMonth uses a
// minimum one-month window so a just-started backlog doesn't project an
// infinite rate; MonthsToZeroBacklog is 0 when the rate can't be computed.
type finishlineStats struct {
	Total            int     `json:"total"`
	Backlog          int     `json:"backlog"`
	InProgress       int     `json:"in_progress"`
	Finished         int     `json:"finished"`
	Dropped          int     `json:"dropped"`
	FinishedPct      float64 `json:"finished_pct"`
	MonthsActive     float64 `json:"months_active"`
	FinishedPerMonth float64 `json:"finished_per_month"`
	MonthsToZero     float64 `json:"months_to_zero_backlog"`
}

func computeFinishlineStats(rows []store.GameBacklogRow, now time.Time) finishlineStats {
	st := finishlineStats{Total: len(rows)}
	for _, r := range rows {
		switch r.Status {
		case store.GameBacklogStatusBacklog:
			st.Backlog++
		case store.GameBacklogStatusInProgress:
			st.InProgress++
		case store.GameBacklogStatusFinished:
			st.Finished++
		case store.GameBacklogStatusDropped:
			st.Dropped++
		}
	}
	if st.Total > 0 {
		st.FinishedPct = float64(st.Finished) / float64(st.Total) * 100
	}
	st.MonthsActive = monthsSinceEarliestAdd(rows, now)
	if st.Finished > 0 {
		window := st.MonthsActive
		if window < 1 {
			window = 1 // guard divide-by-zero / instant-infinite rate
		}
		st.FinishedPerMonth = float64(st.Finished) / window
		remaining := st.Backlog + st.InProgress // dropped rows sit outside the active queue
		if remaining > 0 && st.FinishedPerMonth > 0 {
			st.MonthsToZero = math.Ceil(float64(remaining) / st.FinishedPerMonth)
		}
	}
	return st
}

// finishlineMilestone is one 10% checkpoint on the road to a finished
// backlog. Game names the title that would cross the checkpoint, counting
// oldest-active-first; empty once the checkpoint is already behind you.
type finishlineMilestone struct {
	Percent        int    `json:"percent"`
	FinishedNeeded int    `json:"finished_needed"`
	Game           string `json:"game,omitempty"`
	Status         string `json:"status"`
}

// buildFinishlineMilestones lays out a checkpoint every 10% (10..100). The
// game at a checkpoint is the (needed - finished)-th row of the oldest-first
// active queue; checkpoints already reached report status "reached" with no
// game (bounded: at most 10 milestones).
func buildFinishlineMilestones(total, finished int, active []store.GameBacklogRow) []finishlineMilestone {
	out := make([]finishlineMilestone, 0, 10)
	if total == 0 {
		return out
	}
	for pct := 10; pct <= 100; pct += 10 {
		needed := int(math.Ceil(float64(pct) / 100 * float64(total)))
		if needed > total {
			needed = total
		}
		m := finishlineMilestone{Percent: pct, FinishedNeeded: needed}
		if needed <= finished {
			m.Status = "reached"
		} else if idx := needed - finished - 1; idx >= 0 && idx < len(active) {
			m.Status = "next"
			m.Game = active[idx].Title
		} else {
			m.Status = "beyond" // past the active shelf: needs new additions to reach
		}
		out = append(out, m)
	}
	return out
}

// ----- view -----

type finishlineMeta struct {
	Source string `json:"source"`
	Total  int    `json:"total"`
}

type finishlineView struct {
	Meta       finishlineMeta        `json:"meta"`
	Stats      finishlineStats       `json:"stats"`
	Milestones []finishlineMilestone `json:"milestones"`
}

// buildFinishlineView is pure: testable without a store or cobra command.
func buildFinishlineView(rows []store.GameBacklogRow, now time.Time) finishlineView {
	return finishlineView{
		Meta:       finishlineMeta{Source: "local", Total: len(rows)},
		Stats:      computeFinishlineStats(rows, now),
		Milestones: buildFinishlineMilestones(len(rows), countFinished(rows), activeBacklogRows(rows)),
	}
}

func countFinished(rows []store.GameBacklogRow) int {
	n := 0
	for _, r := range rows {
		if r.Status == store.GameBacklogStatusFinished {
			n++
		}
	}
	return n
}

func newNovelFinishlineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finishline",
		Short: "Backlog progress: finish rate, projection, and 10% milestones",
		Long: `Where backlog audit exposes decision debt, finishline measures motion:
how much of the backlog is finished, at what rate you finish games per month
since the backlog began, and — at that rate — how many months until the
backlog hits zero. The milestone table marks every 10% checkpoint and names
the game that would cross it, counting oldest-active-first.

The projection needs finished games to compute a rate; with zero finished it
reports months_to_zero_backlog 0 (not enough signal yet). Everything is
local: no RAWG call is made.`,
		Example: strings.Trim(`
  game-goat-pp-cli finishline
  game-goat-pp-cli finishline --json
  game-goat-pp-cli finishline --json --select stats.finished_pct,stats.months_to_zero_backlog
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--json",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "finishline")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "finishline takes no positional arguments")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows, err := allBacklogRows(ctx)
			if err != nil {
				return fmt.Errorf("reading local backlog: %w", err)
			}
			view := buildFinishlineView(rows, time.Now())
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			st := view.Stats
			fmt.Fprintf(w, "finishline — %d finished of %d (%.0f%%)\n", st.Finished, st.Total, st.FinishedPct)
			fmt.Fprintf(w, "  active: %d backlog, %d in-progress, %d dropped\n", st.Backlog, st.InProgress, st.Dropped)
			if st.Total == 0 {
				fmt.Fprintln(w, "Your backlog is empty — the finish line is already crossed. Add a game: game-goat-pp-cli backlog add 'Hollow Knight'")
				return nil
			}
			if st.FinishedPerMonth > 0 {
				fmt.Fprintf(w, "  pace: %.1f finished/month over %.1f months — zero backlog in ~%.0f months\n",
					st.FinishedPerMonth, st.MonthsActive, st.MonthsToZero)
			} else {
				fmt.Fprintln(w, "  pace: no finished games yet — finish one to start the projection (game-goat-pp-cli backlog add --status=finished)")
			}
			if len(view.Milestones) > 0 {
				fmt.Fprintln(w, "  milestones:")
				for _, m := range view.Milestones {
					game := m.Game
					if game == "" {
						game = "-"
					}
					fmt.Fprintf(w, "    %3d%%  %d finished  %-40s %s\n", m.Percent, m.FinishedNeeded, game, m.Status)
				}
			}
			return nil
		},
	}
	return cmd
}
