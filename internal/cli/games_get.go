// games_get.go — hand-written Slice A novel command for games get.
// pp:data-source live — full game detail always comes from the RAWG API.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"

	"github.com/spf13/cobra"
)

// gameStoreLink is one store listing on a game detail record.
type gameStoreLink struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// gameDetailRow extends gameRow with the detail-only RAWG fields.
type gameDetailRow struct {
	gameRow
	Tags       []string        `json:"tags"`
	Stores     []gameStoreLink `json:"stores"`
	Developers []string        `json:"developers"`
	Publishers []string        `json:"publishers"`
	Website    string          `json:"website,omitempty"`
	EsrbRating string          `json:"esrb_rating,omitempty"`
}

func toGameDetailRow(g rawgGame) gameDetailRow {
	row := gameDetailRow{
		gameRow:    toGameRow(g),
		Tags:       refNames(g.Tags),
		Stores:     make([]gameStoreLink, 0, len(g.Stores)),
		Developers: refNames(g.Developers),
		Publishers: refNames(g.Publishers),
		Website:    g.Website,
	}
	if g.EsrbRating != nil {
		row.EsrbRating = g.EsrbRating.Name
	}
	for _, s := range g.Stores {
		if s.Store.Name != "" {
			row.Stores = append(row.Stores, gameStoreLink{ID: s.Store.ID, Name: s.Store.Name, Slug: s.Store.Slug})
		}
	}
	return row
}

type gamesGetMeta struct {
	Source     string               `json:"source"`
	ResolvedBy string               `json:"resolved_by"`
	Ambiguous  []ambiguousCandidate `json:"ambiguous,omitempty"`
}

type gamesGetView struct {
	Meta    gamesGetMeta    `json:"meta"`
	Results []gameDetailRow `json:"results"`
}

// resolveTitleToGameID maps a title to one RAWG id using the top search
// results, applying remake-aware disambiguation: an exact-title match must be
// unique (after any --year pin) or the caller gets an actionable exit-2 error
// listing the candidates (stderr notice for humans, meta.ambiguous in JSON).
func resolveTitleToGameID(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, title, year string) (int, error) {
	exact, _, nearby, err := resolveExactTitleMatches(ctx, cmd, c, flags, title, year)
	if err != nil {
		return 0, err
	}
	if len(exact) == 0 {
		return 0, notFoundErr(fmt.Errorf("no game titled %q in the top RAWG search results; %s", title,
			nearbyMatchHint(nearby, "try 'game-goat-pp-cli games search' or use the RAWG id")))
	}
	if len(exact) > 1 {
		candidates := make([]ambiguousCandidate, 0, len(exact))
		for _, g := range exact {
			candidates = append(candidates, ambiguousCandidate{ID: g.ID, Name: g.Name, Released: g.Released})
		}
		writeAmbiguousNotice(cmd.ErrOrStderr(), candidates)
		msg := fmt.Sprintf("ambiguous title %q matches %d games; pin with --year or use the RAWG id", title, len(exact))
		usage := cmd.CommandPath() + " <title-or-id> --year <yyyy>"
		if flags.asJSON {
			if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"error": msg,
				"usage": usage,
				"meta":  map[string]any{"ambiguous": candidates},
			}, flags); printErr != nil {
				return 0, printErr
			}
		}
		return 0, usageErr(fmt.Errorf("%s\nUsage: %s", msg, usage))
	}
	return exact[0].ID, nil
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func renderGameDetail(cmd *cobra.Command, g gameDetailRow) {
	w := cmd.OutOrStdout()
	metacritic := "n/a"
	if g.Metacritic != nil {
		metacritic = strconv.Itoa(*g.Metacritic)
	}
	fmt.Fprintf(w, "%s (%s) — RAWG id %d\n", orDash(g.Name), orDash(g.Released), g.ID)
	fmt.Fprintf(w, "  rating:     %.1f/5 from %d ratings\n", g.Rating, g.RatingsCount)
	fmt.Fprintf(w, "  metacritic: %s\n", metacritic)
	fmt.Fprintf(w, "  playtime:   ~%dh to beat\n", g.Playtime)
	fmt.Fprintf(w, "  added by:   %d players\n", g.Added)
	fmt.Fprintf(w, "  genres:     %s\n", orDash(strings.Join(g.Genres, ", ")))
	fmt.Fprintf(w, "  platforms:  %s\n", orDash(strings.Join(g.Platforms, ", ")))
	if len(g.Developers) > 0 {
		fmt.Fprintf(w, "  developers: %s\n", strings.Join(g.Developers, ", "))
	}
	if len(g.Publishers) > 0 {
		fmt.Fprintf(w, "  publishers: %s\n", strings.Join(g.Publishers, ", "))
	}
	if len(g.Tags) > 0 {
		fmt.Fprintf(w, "  tags:       %s\n", truncateList(g.Tags, 8))
	}
	if len(g.Stores) > 0 {
		storeNames := make([]string, 0, len(g.Stores))
		for _, s := range g.Stores {
			storeNames = append(storeNames, s.Name)
		}
		fmt.Fprintf(w, "  stores:     %s\n", strings.Join(storeNames, ", "))
	}
	if g.EsrbRating != "" {
		fmt.Fprintf(w, "  esrb:       %s\n", g.EsrbRating)
	}
	fmt.Fprintf(w, "  use --json for the full RAWG record\n")
}

func newGamesGetCmd(flags *rootFlags) *cobra.Command {
	var year string

	cmd := &cobra.Command{
		Use:   "get <title-or-id>",
		Short: "Get full details for one game by title or RAWG id",
		Long: `Resolve a game by title (top RAWG search match) or numeric RAWG id and
print its full detail record: ratings, metacritic, playtime, genres, tags,
platforms, stores, and developers. Titles that match multiple same-named
games (remakes like Doom 1993 vs 2016) are flagged as ambiguous — pin the
release year with --year or use the RAWG id.`,
		Example: strings.Trim(`
  game-goat-pp-cli games get "Hollow Knight"
  game-goat-pp-cli games get "God of War" --year 2018
  game-goat-pp-cli games get 23445 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Hollow Knight",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "games get")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title-or-id>", "a game title or RAWG id is required")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title-or-id> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2018")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			input := strings.Join(args, " ")
			resolvedBy := "id"
			gameID := 0
			if id, ok := parseGameID(input); ok {
				gameID = id
			} else {
				resolvedBy = "title"
				gameID, err = resolveTitleToGameID(cmd.Context(), cmd, c, flags, input, year)
				if err != nil {
					return err
				}
			}
			game, err := fetchGameByID(cmd.Context(), c, gameID)
			if err != nil {
				return err
			}
			view := gamesGetView{
				Meta:    gamesGetMeta{Source: "live", ResolvedBy: resolvedBy},
				Results: []gameDetailRow{toGameDetailRow(game)},
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			renderGameDetail(cmd, view.Results[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin the title to a release year when remakes share the name (e.g. 2018)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		gamesCmd, _, err := root.Find([]string{"games"})
		if err == nil {
			addNovelCommandIfAbsent(gamesCmd, newGamesGetCmd(flags))
		}
	})
}
