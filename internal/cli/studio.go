// studio.go — hand-written Slice B novel command (top-level).
// pp:data-source live — developer timeline from RAWG /developers + /games
// with a defensive local backlog join.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"game-goat-pp-cli/internal/client"

	"github.com/spf13/cobra"
)

type studioEntry struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Released   string  `json:"released"`
	Year       string  `json:"year,omitempty"`
	Rating     float64 `json:"rating"`
	Metacritic *int    `json:"metacritic"`
	OnBacklog  bool    `json:"on_backlog"`
}

type studioMeta struct {
	Source           string `json:"source"`
	Developer        string `json:"developer"`
	DeveloperID      int    `json:"developer_id"`
	ResolvedBy       string `json:"resolved_by"`
	Count            int    `json:"count"`
	BacklogAvailable bool   `json:"backlog_available"`
	Note             string `json:"note,omitempty"`
}

type studioView struct {
	Meta    studioMeta    `json:"meta"`
	Results []studioEntry `json:"results"`
}

// resolveDeveloper maps a developer name to one RAWG developer record via
// /developers?search=. An exact (normalized) name match wins; otherwise the
// top result is used with a stderr note so typos surface.
func resolveDeveloper(ctx context.Context, cmd *cobra.Command, c *client.Client, name string) (rawgNamedRef, error) {
	data, err := c.Get(ctx, "/developers", map[string]string{
		"search":    name,
		"page_size": "5",
	})
	if err != nil {
		return rawgNamedRef{}, classifyAPIErrorOnly(err)
	}
	var resp struct {
		Results []rawgNamedRef `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return rawgNamedRef{}, fmt.Errorf("parsing RAWG /developers response: %w", err)
	}
	if len(resp.Results) == 0 {
		return rawgNamedRef{}, notFoundErr(fmt.Errorf("no developer matching %q; browse developers with 'game-goat-pp-cli developers list'", name))
	}
	normalized := normalizeGameTitle(name)
	for _, d := range resp.Results {
		if normalizeGameTitle(d.Name) == normalized {
			return d, nil
		}
	}
	dev := resp.Results[0]
	fmt.Fprintf(cmd.ErrOrStderr(), "note: resolved developer %q to top match %q\n", name, dev.Name)
	return dev, nil
}

func newStudioCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "studio <developer-name>",
		Short: "A developer's release timeline, newest first",
		Long: `Resolve a developer by name via RAWG /developers?search=, then list
their games from /games?developers=<id>&ordering=-released as a release
timeline with year, title, rating, metacritic, and a backlog flag from your
local store. Use this to see a studio's arc over time; use 'games search' to
look up a single title.`,
		Example: strings.Trim(`
  game-goat-pp-cli studio FromSoftware
  game-goat-pp-cli studio Nintendo --limit 10 --json
  game-goat-pp-cli studio FromSoftware --json --select results.name,results.year
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "developer=FromSoftware;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "studio")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <developer-name>", "a developer name is required")
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <developer-name> --limit <n>", "--limit must be at least 1")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			name := strings.Join(args, " ")
			dev, err := resolveDeveloper(ctx, cmd, c, name)
			if err != nil {
				return err
			}
			pageSize := limit
			note := ""
			if pageSize > 40 {
				pageSize = 40
				note = fmt.Sprintf("--limit capped at 40 (RAWG caps a single page); showing the newest %d games", pageSize)
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+note)
			}
			games, source, err := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
				"developers": strconv.Itoa(dev.ID),
				"ordering":   "-released",
				"page_size":  strconv.Itoa(pageSize),
			})
			if err != nil {
				return err
			}
			if source == "" {
				source = "live"
			}
			backlogIDs, backlogAvailable, backlogErr := backlogGameIDs(ctx)
			if backlogErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog: %v\n", backlogErr)
				backlogAvailable = false
			}
			entries := make([]studioEntry, 0, len(games))
			for _, g := range games {
				entries = append(entries, studioEntry{
					ID:         g.ID,
					Name:       g.Name,
					Released:   g.Released,
					Year:       yearOf(g.Released),
					Rating:     g.Rating,
					Metacritic: g.Metacritic,
					OnBacklog:  backlogIDs != nil && backlogIDs[g.ID],
				})
			}
			meta := studioMeta{
				Source:           source,
				Developer:        dev.Name,
				DeveloperID:      dev.ID,
				ResolvedBy:       "search",
				Count:            len(entries),
				BacklogAvailable: backlogAvailable && backlogErr == nil,
			}
			if note != "" {
				meta.Note = note
			}
			view := studioView{Meta: meta, Results: entries}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No games found for developer %q.\n", dev.Name)
				return nil
			}
			rows := make([]map[string]any, 0, len(entries))
			for _, e := range entries {
				metacritic := ""
				if e.Metacritic != nil {
					metacritic = strconv.Itoa(*e.Metacritic)
				}
				backlog := ""
				if e.OnBacklog {
					backlog = "yes"
				}
				rows = append(rows, map[string]any{
					"year":       orDash(e.Year),
					"name":       e.Name,
					"rating":     fmt.Sprintf("%.1f", e.Rating),
					"metacritic": metacritic,
					"backlog":    backlog,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum games to list (RAWG caps a single page at 40)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newStudioCmd(flags))
	})
}
