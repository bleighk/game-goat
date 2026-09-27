// games_upcoming.go — hand-written Slice A novel command for games upcoming.
// pp:data-source live — the release-date window is computed by RAWG.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newGamesUpcomingCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "upcoming",
		Short: "Browse games releasing in the next 90 days",
		Long:  "Browse unreleased and just-announced games with a release date inside the next 90 days, ordered by RAWG's -added count.",
		Example: strings.Trim(`
  game-goat-pp-cli games upcoming --limit 10
  game-goat-pp-cli games upcoming --limit 5 --json
  game-goat-pp-cli games upcoming --limit 20 --json --select results.name,results.released
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "games upcoming")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "games upcoming takes no positional arguments; use --limit to bound results")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "--limit must be at least 1")
			}
			today := time.Now()
			params := map[string]string{
				"ordering":  "-added",
				"page_size": strconv.Itoa(limit),
				"dates":     fmt.Sprintf("%s,%s", today.Format("2006-01-02"), today.AddDate(0, 0, 90).Format("2006-01-02")),
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			games, source, err := fetchGamesResults(cmd.Context(), cmd, c, flags, "live", params)
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
			addNovelCommandIfAbsent(gamesCmd, newGamesUpcomingCmd(flags))
		}
	})
}
