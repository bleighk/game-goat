// games_top_rated.go — hand-written Slice A novel command for games top-rated.
// pp:data-source live — rating ordering (-rating) is computed by RAWG.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newGamesTopRatedCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "top-rated",
		Short: "Browse the highest-rated games",
		Long:  "Browse games ordered by RAWG's -rating score: the community's highest-rated releases.",
		Example: strings.Trim(`
  game-goat-pp-cli games top-rated --limit 10
  game-goat-pp-cli games top-rated --limit 5 --json
  game-goat-pp-cli games top-rated --limit 10 --json --select results.name,results.rating
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "games top-rated")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "games top-rated takes no positional arguments; use --limit to bound results")
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
				"ordering":  "-rating",
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
			addNovelCommandIfAbsent(gamesCmd, newGamesTopRatedCmd(flags))
		}
	})
}
