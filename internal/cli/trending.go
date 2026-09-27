// trending.go — hand-written Slice A novel command (top-level).
// pp:data-source auto — live RAWG -added ordering with a local backlog join.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"game-goat-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// trendingRow flattens gameRow plus the local backlog flag.
type trendingRow struct {
	gameRow
	OnBacklog bool `json:"on_backlog"`
}

type trendingMeta struct {
	Source           string `json:"source"`
	Count            int    `json:"count"`
	BacklogAvailable bool   `json:"backlog_available"`
	BacklogMatched   int    `json:"backlog_matched"`
}

type trendingView struct {
	Meta    trendingMeta  `json:"meta"`
	Results []trendingRow `json:"results"`
}

// backlogGameIDs reads RAWG game ids from the local backlog table. The table
// is created by the backlog slice; this probe stays defensive so trending
// still works before that slice ships: a missing DB or table simply reports
// available=false and trending proceeds without backlog flags.
func backlogGameIDs(ctx context.Context) (map[int]bool, bool, error) {
	db, err := openStoreForRead(ctx, "game-goat-pp-cli")
	if err != nil {
		return nil, false, err
	}
	if db == nil {
		return nil, false, nil
	}
	defer db.Close()
	var table string
	// game_backlog is the canonical Slice C backlog table; the older aliases
	// stay probed defensively so a pre-slice database still joins cleanly.
	err = db.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name IN ('game_backlog','backlog','backlog_items') ORDER BY name DESC LIMIT 1`).Scan(&table)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	for _, query := range []string{
		"SELECT rawg_id FROM " + table,
		"SELECT game_id FROM " + table,
		"SELECT id FROM " + table,
		"SELECT json_extract(data, '$.rawg_id') FROM " + table,
		"SELECT json_extract(data, '$.id') FROM " + table,
	} {
		ids, scanErr := scanIntColumn(ctx, db, query)
		if scanErr == nil {
			if len(ids) == 0 {
				return map[int]bool{}, true, nil
			}
			return ids, true, nil
		}
	}
	return nil, true, fmt.Errorf("backlog table %q has no recognizable game-id column", table)
}

// scanIntColumn drains one integer-valued column into an id set, skipping
// NULL and non-numeric values (slug-style ids are not RAWG ids).
func scanIntColumn(ctx context.Context, db *store.Store, query string) (map[int]bool, error) {
	rows, err := db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[int]bool)
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if !v.Valid {
			continue
		}
		token := strings.TrimSpace(v.String)
		if token == "" {
			continue
		}
		if id, ok := parseGameID(token); ok {
			ids[id] = true
		}
	}
	return ids, rows.Err()
}

func newTrendingCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var days int

	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Recent releases gaining the most players, flagged against your backlog",
		Long: `Show recent releases players are adding fastest: RAWG -added ordering
restricted to games released within the last --days (default 90) — the
momentum view. All-time most-added lives in 'games popular'. Picks are
flagged against your local backlog; without the store the list still
prints with on_backlog=false.`,
		Example: strings.Trim(`
  game-goat-pp-cli trending --limit 10
  game-goat-pp-cli trending --limit 5 --json
  game-goat-pp-cli trending --limit 10 --json --select results.name,results.on_backlog
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "--limit=5;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trending")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "trending takes no positional arguments; use --limit to bound results")
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "--limit must be at least 1")
			}
			if days < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --days <n>", "--days must be at least 1")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			end := time.Now().UTC()
			start := end.AddDate(0, 0, -days)
			games, source, err := fetchGamesResults(cmd.Context(), cmd, c, flags, "auto", map[string]string{
				"ordering":  "-added",
				"dates":     start.Format("2006-01-02") + "," + end.Format("2006-01-02"),
				"page_size": strconv.Itoa(limit),
			})
			if err != nil {
				return err
			}
			if source == "" {
				source = "live"
			}
			backlogIDs, backlogAvailable, backlogErr := backlogGameIDs(cmd.Context())
			if backlogErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog: %v\n", backlogErr)
			} else if !backlogAvailable {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local backlog (run: game-goat-pp-cli backlog add <game>)")
			}
			rows := make([]trendingRow, 0, len(games))
			matched := 0
			for _, g := range games {
				on := backlogIDs != nil && backlogIDs[g.ID]
				if on {
					matched++
				}
				rows = append(rows, trendingRow{gameRow: toGameRow(g), OnBacklog: on})
			}
			gameRows := make([]gameRow, 0, len(rows))
			for _, r := range rows {
				gameRows = append(gameRows, r.gameRow)
			}
			view := trendingView{
				Meta: trendingMeta{
					Source:           source,
					Count:            len(rows),
					BacklogAvailable: backlogAvailable && backlogErr == nil,
					BacklogMatched:   matched,
				},
				Results: rows,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No trending games found.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), gameTableRows(gameRows, backlogIDs, true))
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of games to return (RAWG caps pages at 40)")
	cmd.Flags().IntVar(&days, "days", 90, "Release window in days: trending considers games released within this window")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTrendingCmd(flags))
	})
}
