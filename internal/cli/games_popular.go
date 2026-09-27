// games_popular.go — hand-written Slice A novel command for games popular.
// pp:data-source live — popularity ordering (-added) is computed by RAWG.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newGamesPopularCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "popular",
		Short: "Browse the most-added games right now",
		Long:  "Browse games ordered by RAWG's -added count: the games players are adding to their libraries fastest.",
		Example: strings.Trim(`
  game-goat-pp-cli games popular --limit 10
  game-goat-pp-cli games popular --limit 5 --json
  game-goat-pp-cli games popular --limit 10 --json --select results.name,results.added
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "games popular")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "games popular takes no positional arguments; use --limit to bound results")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "--limit must be at least 1")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			games, source, err := fetchGamesResults(cmd.Context(), cmd, c, flags, "live", map[string]string{
				"ordering":  "-added",
				"page_size": strconv.Itoa(limit),
			})
			if err != nil {
				return err
			}
			return renderGamesListView(cmd, flags, games, source)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of games to return (RAWG caps pages at 40)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		gamesCmd, _, err := root.Find([]string{"games"})
		if err == nil {
			addNovelCommandIfAbsent(gamesCmd, newGamesPopularCmd(flags))
		}
	})
}
