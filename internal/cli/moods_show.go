// moods_show.go — hand-written Slice D novel command: moods show <mood>.
// pp:data-source local — vocabulary detail plus copy-paste recipe commands.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type moodShowMeta struct {
	Source string `json:"source"`
	Mood   string `json:"mood"`
}

type moodShowRecipe struct {
	Label   string `json:"label"`
	Command string `json:"command"`
}

// moodShowRow embeds the vocabulary entry and adds copy-paste recipe
// commands with real flag values resolved from the entry.
type moodShowRow struct {
	moodEntry
	Recipes []moodShowRecipe `json:"recipes"`
}

type moodShowView struct {
	Meta    moodShowMeta  `json:"meta"`
	Results []moodShowRow `json:"results"`
}

// moodRecipes builds copy-paste commands with real flag values: one
// tonight pick and one discover browse, both derived from the recipe.
func moodRecipes(m moodEntry) []moodShowRecipe {
	tonightCmd := fmt.Sprintf("game-goat-pp-cli tonight --mood %s --time 90", m.Name)
	discoverCmd := "game-goat-pp-cli discover"
	if len(m.Recipe.Genres) > 0 {
		discoverCmd += " --genres " + strings.Join(m.Recipe.Genres, ",")
	}
	if len(m.Recipe.Tags) > 0 {
		discoverCmd += " --tags " + strings.Join(m.Recipe.Tags, ",")
	}
	discoverCmd += fmt.Sprintf(" --ordering %s --limit 10", m.Recipe.Ordering)
	return []moodShowRecipe{
		{Label: "pick one tonight", Command: tonightCmd},
		{Label: "browse the recipe", Command: discoverCmd},
	}
}

func newNovelMoodsShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <mood>",
		Short: "Show one mood in detail, with copy-paste recipe commands",
		Long: `Show one mood from the curated vocabulary: its description, its RAWG recipe
(genre/tag filters, ordering, typical playtime band), and copy-paste
commands that use the recipe for real — a tonight pick and a discover
browse. Unknown moods are a typed exit-6 miss listing the valid names.`,
		Example: strings.Trim(`
  game-goat-pp-cli moods show cozy
  game-goat-pp-cli moods show quick-hit --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "local",
			"pp:happy-args":       "mood=cozy",
			"pp:typed-exit-codes": "0,2,6",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "moods show")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <mood>", "a mood name is required (see: game-goat-pp-cli moods list)")
			}
			if len(args) > 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <mood>", "moods show takes exactly one mood name")
			}
			m, ok := moodByName(args[0])
			if !ok {
				return moodNotFoundErr(fmt.Errorf("unknown mood %q; %s", args[0], validMoodsHint()))
			}
			view := moodShowView{
				Meta:    moodShowMeta{Source: "local", Mood: m.Name},
				Results: []moodShowRow{{moodEntry: m, Recipes: moodRecipes(m)}},
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s — %s\n", m.Name, m.Description)
			fmt.Fprintf(w, "  recipe: %s\n", moodRecipeString(m))
			for _, r := range moodRecipes(m) {
				fmt.Fprintf(w, "  %s:\n    %s\n", r.Label, r.Command)
			}
			fmt.Fprintf(w, "browse all: game-goat-pp-cli moods list\n")
			return nil
		},
	}
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		moodsCmd, _, err := root.Find([]string{"moods"})
		if err == nil {
			addNovelCommandIfAbsent(moodsCmd, newNovelMoodsShowCmd(flags))
		}
	})
}
