// moods_list.go — hand-written Slice D novel command: moods list.
// pp:data-source local — the mood vocabulary is a static curated map.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type moodsListMeta struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type moodsListView struct {
	Meta    moodsListMeta `json:"meta"`
	Results []moodEntry   `json:"results"`
}

func newNovelMoodsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the curated moods tonight --mood accepts",
		Long: `List every mood in the curated vocabulary with its description and its
RAWG recipe (genre/tag filters, ordering, typical playtime band). This is
the discoverability surface for tonight --mood: pick a name here, then run
'tonight --mood <name> --time <minutes>' for a time-boxed pick.`,
		Example: strings.Trim(`
  game-goat-pp-cli moods list
  game-goat-pp-cli moods list --json
  game-goat-pp-cli moods list --json --select results.name
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--json",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "moods list")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "moods list takes no positional arguments; browse the vocabulary with no flags")
			}
			view := moodsListView{
				Meta:    moodsListMeta{Source: "local", Count: len(gameMoods)},
				Results: allMoods(),
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			items := make([]map[string]any, 0, len(gameMoods))
			for _, m := range gameMoods {
				items = append(items, map[string]any{
					"mood":        m.Name,
					"description": m.Description,
					"recipe":      moodRecipeString(m),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	return cmd
}
