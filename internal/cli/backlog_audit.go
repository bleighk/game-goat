// backlog_audit.go — hand-written Slice D novel command: backlog audit.
// pp:data-source local — a read/analysis view over the game_backlog table;
// no mutation happens here (recommendations name the exact follow-up
// command instead of executing it).
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"game-goat-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

const auditStaleDays = 90

// allBacklogRows loads every backlog row (newest added first). A missing
// store or table is an empty backlog, not an error.
func allBacklogRows(ctx context.Context) ([]store.GameBacklogRow, error) {
	db, err := backlogReadStore(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return []store.GameBacklogRow{}, nil
	}
	defer db.Close()
	rows, err := db.ListGameBacklog(ctx, store.ListGameBacklogFilter{Sort: "added", Limit: 500})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// backlogKeySets builds rawg-id and normalized-title lookup sets for
// backlog cross-checks in tonight/suggested/similar.
func backlogKeySets(rows []store.GameBacklogRow) (map[int]bool, map[string]bool) {
	ids := make(map[int]bool, len(rows))
	titles := make(map[string]bool, len(rows))
	for _, r := range rows {
		ids[r.RawgID] = true
		if t := normalizeGameTitle(r.Title); t != "" {
			titles[t] = true
		}
	}
	return ids, titles
}

// ----- pure decision-debt classification -----

// auditClassification is the per-row decision-debt verdict.
type auditClassification struct {
	ShelfDays     int
	Stale         bool
	SunkCost      bool
	DropCandidate bool
}

// classifyAuditRow applies the audit rules to one backlog row:
//   - stale: on the shelf >90 days, still status=backlog, never started
//     (hours == 0)
//   - sunk-cost: >=10 hours played but not finished, with a low or absent
//     user rating (the signal that the time is not buying joy)
//   - drop candidate: stale AND nothing anchoring it (unrated, no notes)
func classifyAuditRow(row store.GameBacklogRow, now time.Time) auditClassification {
	var cl auditClassification
	if added, err := time.Parse(time.RFC3339, row.AddedAt); err == nil {
		if days := int(now.Sub(added).Hours() / 24); days > 0 {
			cl.ShelfDays = days
		}
	}
	cl.Stale = cl.ShelfDays > auditStaleDays &&
		row.Status == store.GameBacklogStatusBacklog && row.Hours == 0
	cl.SunkCost = row.Hours >= 10 &&
		(row.Status == store.GameBacklogStatusBacklog || row.Status == store.GameBacklogStatusInProgress) &&
		row.UserRating < 3
	cl.DropCandidate = cl.Stale && row.UserRating == 0 && strings.TrimSpace(row.Notes) == ""
	return cl
}

// medianShelfDays returns the median age (days) of the given ages; 0 when
// there are no rows.
func medianShelfDays(days []int) float64 {
	if len(days) == 0 {
		return 0
	}
	sorted := make([]int, len(days))
	copy(sorted, days)
	sort.Ints(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[mid])
	}
	return (float64(sorted[mid-1]) + float64(sorted[mid])) / 2
}

// ----- audit view -----

type auditMeta struct {
	Source string `json:"source"`
	Total  int    `json:"total"`
	Prune  bool   `json:"prune,omitempty"`
}

type auditStats struct {
	Total           int     `json:"total"`
	Backlog         int     `json:"backlog"`
	InProgress      int     `json:"in_progress"`
	Finished        int     `json:"finished"`
	Dropped         int     `json:"dropped"`
	Unplayed        int     `json:"unplayed"`
	UnplayedPct     float64 `json:"unplayed_pct"`
	MedianShelfDays float64 `json:"median_shelf_days"`
	FinishedRatio   float64 `json:"finished_ratio"`
}

type auditRowView struct {
	RawgID     int     `json:"rawg_id"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	Hours      float64 `json:"hours"`
	UserRating float64 `json:"user_rating"`
	AddedAt    string  `json:"added_at"`
	ShelfDays  int     `json:"shelf_days"`
	Reason     string  `json:"reason"`
}

type auditRecommendation struct {
	Action  string `json:"action"`
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type backlogAuditView struct {
	Meta            auditMeta             `json:"meta"`
	Stats           auditStats            `json:"stats"`
	Stale           []auditRowView        `json:"stale"`
	SunkCost        []auditRowView        `json:"sunk_cost"`
	DropCandidates  []auditRowView        `json:"drop_candidates"`
	Recommendations []auditRecommendation `json:"recommendations"`
}

func auditRowViewFrom(r store.GameBacklogRow, cl auditClassification, reason string) auditRowView {
	return auditRowView{
		RawgID: r.RawgID, Title: r.Title, Status: r.Status, Hours: r.Hours,
		UserRating: r.UserRating, AddedAt: r.AddedAt, ShelfDays: cl.ShelfDays, Reason: reason,
	}
}

// buildBacklogAudit computes the full decision-debt report from the local
// backlog rows. Pure: testable without a store or a cobra command.
func buildBacklogAudit(rows []store.GameBacklogRow, now time.Time, prune bool) backlogAuditView {
	stats := auditStats{Total: len(rows)}
	shelfAges := make([]int, 0, len(rows))
	view := backlogAuditView{
		Meta:            auditMeta{Source: "local", Total: len(rows), Prune: prune},
		Stats:           stats,
		Stale:           make([]auditRowView, 0),
		SunkCost:        make([]auditRowView, 0),
		DropCandidates:  make([]auditRowView, 0),
		Recommendations: make([]auditRecommendation, 0),
	}
	type classified struct {
		row store.GameBacklogRow
		cl  auditClassification
	}
	all := make([]classified, 0, len(rows))
	for _, r := range rows {
		cl := classifyAuditRow(r, now)
		all = append(all, classified{row: r, cl: cl})
		shelfAges = append(shelfAges, cl.ShelfDays)
		switch r.Status {
		case store.GameBacklogStatusBacklog:
			view.Stats.Backlog++
		case store.GameBacklogStatusInProgress:
			view.Stats.InProgress++
		case store.GameBacklogStatusFinished:
			view.Stats.Finished++
		case store.GameBacklogStatusDropped:
			view.Stats.Dropped++
		}
		if r.Hours == 0 {
			view.Stats.Unplayed++
		}
	}
	if view.Stats.Total > 0 {
		view.Stats.UnplayedPct = float64(view.Stats.Unplayed) / float64(view.Stats.Total)
		view.Stats.FinishedRatio = float64(view.Stats.Finished) / float64(view.Stats.Total)
	}
	view.Stats.MedianShelfDays = medianShelfDays(shelfAges)

	dropped := make(map[int]bool)
	for _, c := range all {
		if c.cl.Stale {
			view.Stale = append(view.Stale, auditRowViewFrom(c.row, c.cl,
				fmt.Sprintf("on the shelf %d days, never started", c.cl.ShelfDays)))
		}
		if c.cl.SunkCost && !c.cl.Stale {
			view.SunkCost = append(view.SunkCost, auditRowViewFrom(c.row, c.cl,
				fmt.Sprintf("%.0fh played with no finish in sight and a low/absent rating", c.row.Hours)))
		}
		if c.cl.DropCandidate {
			dropped[c.row.RawgID] = true
			view.DropCandidates = append(view.DropCandidates, auditRowViewFrom(c.row, c.cl,
				fmt.Sprintf("stale %d days, unplayed, unrated — pure decision debt", c.cl.ShelfDays)))
		}
	}

	// Recommendations: bounded, actionable, each naming the exact command.
	for _, c := range all {
		if len(view.Recommendations) >= 3 {
			break
		}
		if c.cl.DropCandidate {
			view.Recommendations = append(view.Recommendations, auditRecommendation{
				Action:  "drop",
				Command: fmt.Sprintf("game-goat-pp-cli backlog remove %q", c.row.Title),
				Reason:  fmt.Sprintf("%q sat unplayed for %d days with no rating or notes — remove it and shrink the debt", c.row.Title, c.cl.ShelfDays),
			})
		}
	}
	for _, c := range all {
		if len(view.Recommendations) >= 5 {
			break
		}
		if c.cl.Stale && !c.cl.DropCandidate {
			view.Recommendations = append(view.Recommendations, auditRecommendation{
				Action:  "start-or-shelve",
				Command: fmt.Sprintf("game-goat-pp-cli backlog show %d", c.row.RawgID),
				Reason:  fmt.Sprintf("%q is stale (%d days) but has a rating or notes attached — decide consciously instead of ignoring it", c.row.Title, c.cl.ShelfDays),
			})
		}
	}
	for _, c := range all {
		if len(view.Recommendations) >= 6 {
			break
		}
		if c.cl.SunkCost && !dropped[c.row.RawgID] {
			view.Recommendations = append(view.Recommendations, auditRecommendation{
				Action:  "finish-or-drop",
				Command: "game-goat-pp-cli queue",
				Reason:  fmt.Sprintf("%.0fh into %q with a low/absent rating — let the queue rank it, or drop it and stop paying sunk cost", c.row.Hours, c.row.Title),
			})
		}
	}

	if prune {
		// The prune view narrows the report to drop candidates and their
		// remove commands. The audit itself stays read-only.
		view.Stale = make([]auditRowView, 0)
		view.SunkCost = make([]auditRowView, 0)
		for i := len(view.Recommendations) - 1; i >= 0; i-- {
			if view.Recommendations[i].Action != "drop" {
				view.Recommendations = append(view.Recommendations[:i], view.Recommendations[i+1:]...)
			}
		}
	}
	return view
}

func newNovelBacklogAuditCmd(flags *rootFlags) *cobra.Command {
	var prune bool

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Backlog health: stale, sunk-cost, and drop-candidate decision debt",
		Long: `Audit the local backlog for decision debt — rows you added but never
acted on. The report covers:

  stale            added >90 days ago, still status=backlog, hours == 0
  sunk_cost        >=10h played but unfinished, rating low or absent
  drop_candidates  stale AND unrated AND no notes (nothing anchors them)
  stats            per-status counts, unplayed %, median shelf-time,
                   finished ratio
  recommendations  bounded, actionable, each naming the exact follow-up
                   command (backlog remove <title> / backlog show <id>)

The audit is a read-only analysis view: it never mutates the backlog. With
--prune the report narrows to drop candidates and their remove commands.
Use 'queue' for new picks or 'finishline' for in-progress ranking.`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog audit
  game-goat-pp-cli backlog audit --json
  game-goat-pp-cli backlog audit --prune --json --select drop_candidates
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--json",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backlog audit")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "backlog audit takes no positional arguments")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows, err := allBacklogRows(ctx)
			if err != nil {
				return fmt.Errorf("reading local backlog: %w", err)
			}
			view := buildBacklogAudit(rows, time.Now(), prune)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "backlog audit — %d rows: %d backlog, %d in-progress, %d finished, %d dropped\n",
				view.Stats.Total, view.Stats.Backlog, view.Stats.InProgress, view.Stats.Finished, view.Stats.Dropped)
			fmt.Fprintf(w, "  unplayed: %.0f%%   median shelf time: %.0f days   finished: %.0f%%\n",
				view.Stats.UnplayedPct*100, view.Stats.MedianShelfDays, view.Stats.FinishedRatio*100)
			if view.Stats.Total == 0 {
				fmt.Fprintln(w, "Your backlog is empty — nothing to audit. Add one: game-goat-pp-cli backlog add 'Hollow Knight'")
				return nil
			}
			sections := []struct {
				label string
				rows  []auditRowView
			}{
				{"stale", view.Stale},
				{"sunk-cost", view.SunkCost},
				{"drop candidates", view.DropCandidates},
			}
			for _, s := range sections {
				if prune && s.label != "drop candidates" {
					continue
				}
				if len(s.rows) == 0 {
					fmt.Fprintf(w, "  %s: none\n", s.label)
					continue
				}
				fmt.Fprintf(w, "  %s:\n", s.label)
				for _, r := range s.rows {
					fmt.Fprintf(w, "    %-40s %s\n", r.Title, r.Reason)
				}
			}
			if len(view.Recommendations) > 0 {
				fmt.Fprintln(w, "  recommendations:")
				for _, rec := range view.Recommendations {
					fmt.Fprintf(w, "    %-14s %s\n      why: %s\n", rec.Action, rec.Command, rec.Reason)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false, "narrow the report to drop candidates and their remove commands (still read-only)")
	return cmd
}
