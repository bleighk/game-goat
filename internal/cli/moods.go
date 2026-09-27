// moods.go — hand-written Slice D novel command group: the curated mood
// vocabulary that powers tonight --mood, plus the "moods" parent.
// pp:data-source local — the vocabulary is static and ships with the CLI;
// recipes only resolve against RAWG when tonight/discover use them.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// moodRecipe is the RAWG /games query a mood expands to: genre/tag slugs
// plus the ordering the mood's picks should come back in.
type moodRecipe struct {
	Genres   []string `json:"genres,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Ordering string   `json:"ordering"`
}

// moodEntry is one curated mood in the static vocabulary. PlaytimeMin/
// PlaytimeMax bound the RAWG average playtime (minutes) of games the mood
// fits, so tonight can time-box a session against a mood band.
type moodEntry struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Recipe      moodRecipe `json:"recipe"`
	PlaytimeMin int        `json:"playtime_min_minutes"`
	PlaytimeMax int        `json:"playtime_max_minutes"`
}

// gameMoods is the curated mood vocabulary. Order is stable: the surprise-me
// picker indexes into it by --seed, so appending is safe but reordering
// changes which mood a given seed maps to.
var gameMoods = []moodEntry{
	{
		Name:        "cozy",
		Description: "Warm, low-stakes games to sink into on the couch",
		Recipe:      moodRecipe{Genres: []string{"casual"}, Tags: []string{"cozy"}, Ordering: "-rating"},
		PlaytimeMax: 900,
	},
	{
		Name:        "story-heavy",
		Description: "Narrative-first games where the plot is the point",
		Recipe:      moodRecipe{Genres: []string{"adventure", "role-playing-games-rpg"}, Tags: []string{"story-rich"}, Ordering: "-rating"},
		PlaytimeMin: 480, PlaytimeMax: 2400,
	},
	{
		Name:        "challenging",
		Description: "Games that fight back — soulslikes and hard-as-nails action",
		Recipe:      moodRecipe{Genres: []string{"action"}, Tags: []string{"difficult", "souls-like"}, Ordering: "-rating"},
		PlaytimeMin: 600, PlaytimeMax: 3000,
	},
	{
		Name:        "quick-hit",
		Description: "Short-session games you can finish in one sitting",
		Recipe:      moodRecipe{Genres: []string{"casual", "arcade"}, Ordering: "-rating"},
		PlaytimeMax: 120,
	},
	{
		Name:        "co-op",
		Description: "Couch and online co-op / party games for two or more",
		Recipe:      moodRecipe{Tags: []string{"co-op", "multiplayer"}, Ordering: "-rating"},
		PlaytimeMax: 1800,
	},
	{
		Name:        "retro",
		Description: "Arcade classics and pixel-art throwbacks",
		Recipe:      moodRecipe{Genres: []string{"arcade"}, Tags: []string{"retro"}, Ordering: "-rating"},
		PlaytimeMax: 600,
	},
	{
		Name:        "open-world",
		Description: "Big maps to get lost in for an evening (or fifty)",
		Recipe:      moodRecipe{Genres: []string{"action", "adventure"}, Tags: []string{"open-world"}, Ordering: "-rating"},
		PlaytimeMin: 900, PlaytimeMax: 3600,
	},
	{
		Name:        "puzzle",
		Description: "Brain teasers and logic games with zero twitch reflexes",
		Recipe:      moodRecipe{Genres: []string{"puzzle"}, Ordering: "-rating"},
		PlaytimeMax: 600,
	},
	{
		Name:        "horror",
		Description: "Scary games best played with the lights off",
		Recipe:      moodRecipe{Tags: []string{"horror"}, Ordering: "-rating"},
		PlaytimeMax: 1200,
	},
	{
		Name:        "power-fantasy",
		Description: "Games that make you feel gloriously overpowered",
		Recipe:      moodRecipe{Genres: []string{"action", "shooter"}, Tags: []string{"power-fantasy"}, Ordering: "-rating"},
		PlaytimeMin: 300, PlaytimeMax: 1800,
	},
}

// allMoods returns the curated vocabulary in stable order. Callers must
// not mutate the returned slice.
func allMoods() []moodEntry {
	return gameMoods
}

// moodNames lists the vocabulary in stable order.
func moodNames() []string {
	names := make([]string, 0, len(gameMoods))
	for _, m := range gameMoods {
		names = append(names, m.Name)
	}
	return names
}

// moodByName resolves a mood case-insensitively.
func moodByName(name string) (moodEntry, bool) {
	name = strings.TrimSpace(name)
	for _, m := range gameMoods {
		if strings.EqualFold(name, m.Name) {
			return m, true
		}
	}
	return moodEntry{}, false
}

// validMoodsHint is the bounded list appended to every unknown-mood error
// so callers can fix the flag without a second round trip.
func validMoodsHint() string {
	return fmt.Sprintf("valid moods: %s (see: game-goat-pp-cli moods list)", strings.Join(moodNames(), ", "))
}

// moodNotFoundErr is the typed exit-6 miss for an unknown mood name.
func moodNotFoundErr(err error) error {
	return &cliError{code: 6, err: err}
}

// moodBandString renders the playtime band for human tables.
func moodBandString(m moodEntry) string {
	switch {
	case m.PlaytimeMin <= 0 && m.PlaytimeMax <= 0:
		return ""
	case m.PlaytimeMin <= 0:
		return fmt.Sprintf("~0-%dmin", m.PlaytimeMax)
	default:
		return fmt.Sprintf("~%d-%dmin", m.PlaytimeMin, m.PlaytimeMax)
	}
}

// moodRecipeString renders the recipe (and band) as a one-line hint.
func moodRecipeString(m moodEntry) string {
	parts := make([]string, 0, 4)
	if len(m.Recipe.Genres) > 0 {
		parts = append(parts, "genres="+strings.Join(m.Recipe.Genres, ","))
	}
	if len(m.Recipe.Tags) > 0 {
		parts = append(parts, "tags="+strings.Join(m.Recipe.Tags, ","))
	}
	parts = append(parts, "ordering="+m.Recipe.Ordering)
	if band := moodBandString(m); band != "" {
		parts = append(parts, band)
	}
	return strings.Join(parts, " + ")
}

// moodRecipeParams expands a mood into RAWG /games query params.
func moodRecipeParams(m moodEntry, pageSize int) map[string]string {
	params := map[string]string{"ordering": m.Recipe.Ordering}
	if len(m.Recipe.Genres) > 0 {
		params["genres"] = strings.Join(m.Recipe.Genres, ",")
	}
	if len(m.Recipe.Tags) > 0 {
		params["tags"] = strings.Join(m.Recipe.Tags, ",")
	}
	if pageSize > 0 {
		params["page_size"] = strconv.Itoa(pageSize)
	}
	return params
}

// pickSurpriseMood deterministically maps a seed to a vocabulary entry —
// the surprise-me default for tonight --mood.
func pickSurpriseMood(seed int64) moodEntry {
	n := int64(len(gameMoods))
	if n == 0 {
		return moodEntry{}
	}
	idx := seed % n
	if idx < 0 {
		idx += n
	}
	return gameMoods[idx]
}

func newNovelMoodsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "moods",
		Short: "The curated mood vocabulary tonight --mood accepts",
		Long: `Browse the curated mood vocabulary that powers tonight --mood. Each mood
is a named RAWG recipe: genre/tag filters, an ordering, and a typical
playtime band, so 'tonight --mood cozy --time 90' resolves to real /games
query parameters.

Subcommands:

  list            every mood with its description and recipe
  show <mood>     one mood in detail, with copy-paste commands

The vocabulary is static and local: no RAWG call is needed to browse it.`,
		Example: strings.Trim(`
  game-goat-pp-cli moods list
  game-goat-pp-cli moods list --json
  game-goat-pp-cli moods show cozy
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelMoodsListCmd(flags))
	return cmd
}
