// queue.go — hand-written Slice C novel command (top-level).
// pp:data-source auto — local backlog rows enriched with live RAWG ratings
// (bounded lookups), degrading to wait-time-only ranking when offline.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"game-goat-pp-cli/internal/client"
	"game-goat-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// queueRatingLookupBudget caps live RAWG lookups per queue run: at most 10
// titles get a rating; the rest rank by wait time (rating 0).
const queueRatingLookupBudget = 10

// Reasons are part of the command's contract (documented in --help):
// they must stay stable for scripts and agent consumers.
const (
	queueReasonStarted  = "already started — pick up where you left off"
	queueReasonTopRated = "highest-rated on your backlog"
	queueReasonWaiting  = "waiting longest since "
	queueReasonOffline  = "offline pick (no RAWG rating)"
)

type queuePick struct {
	RawgID int     `json:"rawg_id"`
	Title  string  `json:"title"`
	Rating float64 `json:"rating"`
	Hours  float64 `json:"hours"`
	Status string  `json:"status"`
	Reason string  `json:"reason"`
}

type queueMeta struct {
	Source  string `json:"source"`
	Count   int    `json:"count"`
	Ratings string `json:"ratings"` // live | offline
	Lookups int    `json:"lookups"` // successful live rating lookups
	Limit   int    `json:"limit"`
}

type queueView struct {
	Meta    queueMeta   `json:"meta"`
	Results []queuePick `json:"results"`
}

// queueRatings looks up the live RAWG rating for each distinct title among
// the first `budget` rows. Returns the per-title ratings (0.0 when a lookup
// fails or finds no exact match) and how many lookups succeeded.
func queueRatings(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, rows []store.GameBacklogRow) (map[string]float64, int) {
	ratings := make(map[string]float64)
	seen := make(map[string]bool)
	succeeded := 0
	budget := queueRatingLookupBudget
	for _, r := range rows {
		if budget <= 0 {
			break
		}
		if r.Title == "" || seen[r.Title] {
			continue
		}
		seen[r.Title] = true
		budget--
		exact, _, _, rerr := resolveExactTitleMatches(ctx, cmd, c, flags, r.Title, "")
		if rerr != nil {
			continue
		}
		succeeded++
		if len(exact) == 0 {
			continue
		}
		ratings[r.Title] = exact[0].Rating
	}
	return ratings, succeeded
}

// queueRatingCell formats a RAWG rating (0-5) for the human table.
func queueRatingCell(r float64) string {
	if r <= 0 {
		return "-"
	}
	return strconv.FormatFloat(r, 'f', 1, 64)
}

func newQueueCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Next-play picks from your backlog, with reasons",
		Long: `Pick what to play next from your local backlog. Picks come in tiers,
and each pick carries a reason:

  1. in-progress rows first, least played first — "already started — pick up
     where you left off"
  2. then the highest RAWG-rated backlog-status rows — "highest-rated on
     your backlog" (live rating lookups, capped at 10 titles per run; titles
     beyond the budget rank by wait time)
  3. then the longest-waiting remaining entries — "waiting longest since
     <added-at>"

Finished rows are never picked. If the RAWG key is missing or unreachable,
the whole queue degrades to wait-time-only ranking with the reason
"offline pick (no RAWG rating)" and a stderr notice.`,
		Example: strings.Trim(`
  game-goat-pp-cli queue
  game-goat-pp-cli queue --limit 5 --json
  game-goat-pp-cli queue --limit 1 --json --select results.title,results.reason
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "--limit=3;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "queue")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "queue takes no positional arguments; use --limit")
			}
			if limit < 1 || limit > 10 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <1-10>", "--limit must be between 1 and 10 picks")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows, err := backlogRowsForQueue(ctx)
			if err != nil {
				return err
			}

			picks := make([]queuePick, 0, limit)
			emptyView := queueView{
				Meta:    queueMeta{Source: "local", Count: 0, Ratings: "live", Limit: limit},
				Results: picks,
			}
			if len(rows) == 0 {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), emptyView, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Your backlog is empty. Add games first: game-goat-pp-cli backlog add 'Hollow Knight'")
				return nil
			}

			// Live rating enrichment for the backlog-status tier. Ratings are
			// optional: any failure degrades the ranking, never the command.
			ratings := map[string]float64{}
			lookups := 0
			offline := false
			if c, clientErr := flags.newClient(); clientErr != nil {
				offline = true
			} else {
				ratings, lookups = queueRatings(ctx, cmd, c, flags, rows)
				if lookups == 0 {
					offline = true
				}
			}
			if offline {
				fmt.Fprintln(cmd.ErrOrStderr(), "no RAWG ratings available (key missing or API unreachable); ranking by wait time only")
			}

			var inProgress, backlogStatus, dropped []store.GameBacklogRow
			for _, r := range rows {
				switch r.Status {
				case store.GameBacklogStatusInProgress:
					inProgress = append(inProgress, r)
				case store.GameBacklogStatusBacklog:
					backlogStatus = append(backlogStatus, r)
				case store.GameBacklogStatusDropped:
					dropped = append(dropped, r)
				}
			}

			// Offline ranking: wait time only (added_at asc), which is the
			// order backlogRowsForQueue already produced within each tier.
			if !offline {
				sort.SliceStable(backlogStatus, func(i, j int) bool {
					ri, rj := ratings[backlogStatus[i].Title], ratings[backlogStatus[j].Title]
					if ri != rj {
						return ri > rj
					}
					return backlogStatus[i].AddedAt < backlogStatus[j].AddedAt
				})
			}

			for _, r := range inProgress {
				if len(picks) >= limit {
					break
				}
				picks = append(picks, queuePick{
					RawgID: r.RawgID, Title: r.Title, Rating: ratings[r.Title],
					Hours: r.Hours, Status: r.Status, Reason: queueReasonStarted,
				})
			}
			for _, r := range backlogStatus {
				if len(picks) >= limit {
					break
				}
				picks = append(picks, queuePick{
					RawgID: r.RawgID, Title: r.Title, Rating: ratings[r.Title],
					Hours: r.Hours, Status: r.Status, Reason: queueReasonTopRated,
				})
			}
			for _, r := range dropped {
				if len(picks) >= limit {
					break
				}
				picks = append(picks, queuePick{
					RawgID: r.RawgID, Title: r.Title, Rating: ratings[r.Title],
					Hours: r.Hours, Status: r.Status,
					Reason: queueReasonWaiting + backlogDateCell(r.AddedAt),
				})
			}
			if offline {
				// Degraded picks: same wait-time order, one honest reason.
				for i := range picks {
					if picks[i].Status != store.GameBacklogStatusInProgress {
						picks[i].Reason = queueReasonOffline
						picks[i].Rating = 0
					}
				}
			}

			view := queueView{
				Meta: queueMeta{
					Source:  "auto",
					Count:   len(picks),
					Ratings: "live", Lookups: lookups, Limit: limit,
				},
				Results: picks,
			}
			if offline {
				view.Meta.Ratings = "offline"
				view.Meta.Source = "local"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			items := make([]map[string]any, 0, len(picks))
			for _, p := range picks {
				items = append(items, map[string]any{
					"title":  p.Title,
					"status": p.Status,
					"hours":  backlogHoursCell(p.Hours),
					"rating": queueRatingCell(p.Rating),
					"reason": p.Reason,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 3, "number of picks to show (1-10)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newQueueCmd(flags))
	})
}
