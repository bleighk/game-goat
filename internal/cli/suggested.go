// suggested.go — hand-written Slice D novel command (top-level).
// pp:data-source live — resolve one game, then RAWG's /games/{id}/suggested
// with a silent tier-gate fallback join for free keys.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"game-goat-pp-cli/internal/client"

	"github.com/spf13/cobra"
)

// ----- pure helpers (unit-tested) -----

// isSuggestedTierGate reports whether the /games/{id}/suggested failure is
// the RAWG business-tier gate (401/403) — the signal to fall back to a
// shared-genre join instead of failing the command.
func isSuggestedTierGate(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == 401 || apiErr.StatusCode == 403)
}

// idCSV renders genre ids as a RAWG comma-separated param value.
func idCSV(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

// minSuggestedRatingSample is the smallest RAWG ratings_count treated as
// a confident community signal. Below it a 4.7 is a handful of fans, not
// a recommendation — RAWG's -rating ordering surfaces exactly those rows
// first (UAT F-U13: megabonk's fallback led with 4.7s backed by 6-7
// ratings while a real roguelite pick sat in the tail).
const minSuggestedRatingSample = 20

// namedRefIDs extracts ids from a named-ref slice (developers, publishers).
func namedRefIDs(refs []rawgNamedRef) []int {
	ids := make([]int, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	return ids
}

// broadGenres are RAWG genres carried by a majority of games; matching on
// one of them alone is not a similarity signal (UAT F-U16: guildrun).
var broadGenres = map[string]bool{
	"indie": true, "action": true, "adventure": true,
}

// sharesOnlyBroadGenre reports whether g's only shared genre with the seed
// is a broad one (Indie, Action, Adventure). Such a row matches millions of
// games and carries nothing seed-specific.
func sharesOnlyBroadGenre(g rawgGame, seedGenres map[string]bool) bool {
	shared := 0
	sharedName := ""
	for _, ref := range g.Genres {
		if seedGenres[strings.ToLower(ref.Name)] {
			shared++
			sharedName = strings.ToLower(ref.Name)
		}
	}
	return shared == 1 && broadGenres[sharedName]
}

// ----- view -----

type suggestedResult struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Released     string   `json:"released,omitempty"`
	Rating       float64  `json:"rating"`
	RatingsCount int      `json:"ratings_count"`
	Genres       []string `json:"genres"`
	OnBacklog    bool     `json:"on_backlog"`
	Reason       string   `json:"reason,omitempty"`
}

type suggestedMeta struct {
	Source     string               `json:"source"`
	DataOrigin string               `json:"data_origin"` // suggested-endpoint | fallback-join
	Seed       string               `json:"seed"`
	SeedID     int                  `json:"seed_id"`
	Note       string               `json:"note,omitempty"`
	Ambiguous  []ambiguousCandidate `json:"ambiguous,omitempty"`
	Count      int                  `json:"count"`
	Limit      int                  `json:"limit"`
}

type suggestedView struct {
	Meta    suggestedMeta     `json:"meta"`
	Results []suggestedResult `json:"results"`
}

func newSuggestedCmd(flags *rootFlags) *cobra.Command {
	var year string
	var limit int

	cmd := &cobra.Command{
		Use:   "suggested <title>",
		Short: "Games RAWG suggests next, with a genre-join fallback",
		Long: `Resolve a title and fetch RAWG's own suggestions for it via
/games/{id}/suggested. That endpoint lives behind RAWG's business tier:
when a free key gets HTTP 401/403 the command falls back to what the free
tier can honestly say, in tiers: same-studio games first (the developer of
the seed, from its RAWG detail), then games matching the seed's defining
gameplay tag (roguelite, metroidvania — its mechanics identity), and only
then a shared-genre join. Every tier is ranked by genre overlap, metacritic,
rating sample size, and rating; tiny rating samples (a 4.7 from six
ratings) are filtered. The seed itself is excluded and rows already on
your local backlog are flagged.

meta.data_origin always reports which path produced the results:
"suggested-endpoint" or "fallback-join" (with a note when the fallback
fired). Remake collisions on the seed title are flagged ambiguous; pin with
--year.`,
		Example: strings.Trim(`
  game-goat-pp-cli suggested "Dark Souls III"
  game-goat-pp-cli suggested "God of War" --year 2018
  game-goat-pp-cli suggested "Dark Souls III" --limit 5 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Dark Souls III;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "suggested")
			}
			if len(args) != 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--year <yyyy>]", "suggested takes exactly one quoted game title, e.g. suggested \"Dark Souls III\"")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2016")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > 20 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --limit <1-20>", "--limit must be between 1 and 20")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			seed, candidates, err := fetchResolvedGameDetail(ctx, cmd, c, flags, args[0], year)
			if err != nil {
				return err
			}
			seedGenres := map[string]bool{}
			for _, ref := range seed.Genres {
				seedGenres[strings.ToLower(ref.Name)] = true
			}

			var games []rawgGame
			var studioOf map[int]string
			var tagRow map[int]bool
			var mechTagName string
			dataOrigin := "suggested-endpoint"
			note := ""

			raw, serr := c.Get(ctx, fmt.Sprintf("/games/%d/suggested", seed.ID), map[string]string{
				"page_size": strconv.Itoa(limit),
			})
			if serr == nil {
				var page struct {
					Count   int        `json:"count"`
					Results []rawgGame `json:"results"`
				}
				if jerr := json.Unmarshal(raw, &page); jerr != nil {
					return fmt.Errorf("parsing /games/%d/suggested response: %w", seed.ID, jerr)
				}
				games = page.Results
			} else if isSuggestedTierGate(serr) {
				// Business-tier gate: fall back to what the free tier can
				// honestly say. Tier 1 is same-studio games — the studio's
				// other work is the strongest free signal for "players of
				// this also tried". Tier 2 is a mechanics-tag match: the
				// seed's defining gameplay tag (roguelite, metroidvania...)
				// via the identity-cluster probe shared with 'similar',
				// confidence-ranked. Tier 3 is the shared-genre join with a
				// rating-confidence floor, ONLY when the earlier tiers ran
				// short — a broad genre ("Action") recommends The Last of Us
				// for an indie survivor-like, exactly the irrelevant output
				// UAT rejected (F-U15).
				dataOrigin = "fallback-join"
				fallbackParts := make([]string, 0, 3)
				pool := make([]rawgGame, 0, limit+4)
				studioOf = map[int]string{}
				if devIDs := namedRefIDs(seed.Developers); len(devIDs) > 0 {
					gs, _, derr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
						"developers": idCSV(devIDs),
						"ordering":   "-rating",
						"page_size":  strconv.Itoa(limit * 2),
					})
					if derr != nil {
						return derr
					}
					for _, g := range gs {
						if g.ID == seed.ID || (g.Slug != "" && g.Slug == seed.Slug) {
							continue
						}
						if _, dup := studioOf[g.ID]; dup {
							continue
						}
						studioOf[g.ID] = seed.Developers[0].Name
						pool = append(pool, g)
					}
				}
				if len(pool) > 0 {
					fallbackParts = append(fallbackParts, "same-studio games")
				}
				// Tier 2: mechanics-tag match, confidence-ranked.
				if len(pool) < limit {
					if nb, tag, ok, nerr := mechanicsClusterNeighborhood(ctx, cmd, c, flags, seed, limit); nerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: mechanics-tag tier failed: %v\n", nerr)
					} else if ok {
						confident := make([]rawgGame, 0, len(nb))
						for _, g := range nb {
							if g.RatingsCount >= minSuggestedRatingSample {
								confident = append(confident, g)
							}
						}
						if len(confident) > 0 {
							nb = confident
						}
						sort.SliceStable(nb, func(i, j int) bool {
							si, sj := scoreSimilarity(nb[i], seedGenres), scoreSimilarity(nb[j], seedGenres)
							if si.Shared != sj.Shared {
								return si.Shared > sj.Shared
							}
							if si.Metacritic != sj.Metacritic {
								return si.Metacritic > sj.Metacritic
							}
							if nb[i].RatingsCount != nb[j].RatingsCount {
								return nb[i].RatingsCount > nb[j].RatingsCount
							}
							return si.Rating > sj.Rating
						})
						tagAdded := 0
						for _, g := range nb {
							if len(pool) >= limit {
								break
							}
							if _, dup := studioOf[g.ID]; dup {
								continue
							}
							studioOf[g.ID] = ""
							if tagRow == nil {
								tagRow = map[int]bool{}
							}
							tagRow[g.ID] = true
							pool = append(pool, g)
							tagAdded++
						}
						if tagAdded > 0 {
							mechTagName = strings.ToLower(tag.Name)
							fallbackParts = append(fallbackParts, fmt.Sprintf("mechanics-tag matches (%s)", mechTagName))
						}
					}
				}
				genreIDs := genreIDList(seed)
				if len(genreIDs) == 0 && len(pool) == 0 {
					return notFoundErr(fmt.Errorf("%q has no genre data on RAWG, so no fallback join is possible", seed.Name))
				}
				if len(pool) < limit && len(genreIDs) > 0 {
					gs, _, gerr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
						"genres":    idCSV(genreIDs),
						"ordering":  "-rating",
						"page_size": strconv.Itoa(limit * 3),
					})
					if gerr != nil {
						return gerr
					}
					// Confidence floor: keep only rows with a real rating
					// sample. A shorter confident list beats a longer one
					// padded with 4.7s from six ratings (UAT F-U13); relax
					// only when the floor yields nothing at all.
					confident := make([]rawgGame, 0, len(gs))
					for _, g := range gs {
						if g.RatingsCount >= minSuggestedRatingSample {
							confident = append(confident, g)
						}
					}
					if len(confident) > 0 {
						gs = confident
					}
					sort.SliceStable(gs, func(i, j int) bool {
						si, sj := scoreSimilarity(gs[i], seedGenres), scoreSimilarity(gs[j], seedGenres)
						if si.Shared != sj.Shared {
							return si.Shared > sj.Shared
						}
						if si.Metacritic != sj.Metacritic {
							return si.Metacritic > sj.Metacritic
						}
						if gs[i].RatingsCount != gs[j].RatingsCount {
							return gs[i].RatingsCount > gs[j].RatingsCount
						}
						return si.Rating > sj.Rating
					})
					for _, g := range gs {
						if len(pool) >= limit {
							break
						}
						if g.ID == seed.ID || (g.Slug != "" && g.Slug == seed.Slug) {
							continue
						}
						if _, dup := studioOf[g.ID]; dup {
							continue
						}
						studioOf[g.ID] = ""
						pool = append(pool, g)
					}
					fallbackParts = append(fallbackParts, "shared-genre join")
				}
				if len(fallbackParts) == 0 {
					fallbackParts = append(fallbackParts, "no fallback signal available")
				}
				note = "RAWG suggested requires the business tier (HTTP 401/403); fallback: " + strings.Join(fallbackParts, ", ")
				// Data-scarcity handling: when the genre join was the only
				// usable tier AND the seed carries no gameplay tags and no
				// studio, rows sharing ONLY a broad genre (Indie, Action,
				// Adventure) carry no seed-specific signal — the popularity
				// canon wearing a suggestion label, the exact output UAT
				// rejected twice (F-U15/F-U16). Hide those rows and say why;
				// anything that survives shares a specific genre and earns
				// its place.
				if len(pool) > 0 && len(fallbackParts) == 1 && fallbackParts[0] == "shared-genre join" &&
					len(gameplayTagsByRarity(seed)) == 0 && len(namedRefIDs(seed.Developers)) == 0 {
					kept := make([]rawgGame, 0, len(pool))
					for _, g := range pool {
						if !sharesOnlyBroadGenre(g, seedGenres) {
							kept = append(kept, g)
						}
					}
					hidden := len(pool) - len(kept)
					pool = kept
					note += fmt.Sprintf("; RAWG has minimal data on this game (no gameplay tags, no studio) — weak single-broad-genre matches are hidden (%d hidden)", hidden)
					fmt.Fprintf(cmd.ErrOrStderr(), "note: RAWG has minimal data on %q (no gameplay tags, no studio) — weak single-broad-genre matches are hidden; anything shown shares a specific genre\n", seed.Name)
				}
				games = pool
			} else {
				return classifyAPIErrorOnly(serr)
			}

			backlogIDs, _, bidErr := backlogGameIDs(ctx)
			if bidErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog for on-backlog flags: %v\n", bidErr)
			}
			results := make([]suggestedResult, 0, len(games))
			for _, g := range games {
				if g.ID == seed.ID || (g.Slug != "" && g.Slug == seed.Slug) {
					continue
				}
				if len(results) >= limit {
					break
				}
				genres := refNames(g.Genres)
				p := suggestedResult{
					ID: g.ID, Name: g.Name, Released: g.Released, Rating: g.Rating,
					RatingsCount: g.RatingsCount,
					Genres:       genres,
					OnBacklog:    backlogIDs[g.ID],
				}
				if studio, ok := studioOf[g.ID]; ok && studio != "" && dataOrigin == "fallback-join" {
					p.Reason = fmt.Sprintf("same studio as %s (%s)", seed.Name, studio)
				} else if tagRow != nil && tagRow[g.ID] && mechTagName != "" {
					p.Reason = fmt.Sprintf("plays like %s — %s", seed.Name, mechTagName)
				} else if dataOrigin == "fallback-join" {
					shared := 0
					sharedNames := make([]string, 0, len(g.Genres))
					for _, ref := range g.Genres {
						if seedGenres[strings.ToLower(ref.Name)] {
							shared++
							sharedNames = append(sharedNames, ref.Name)
						}
					}
					noun := "genres"
					if shared == 1 {
						noun = "genre"
					}
					p.Reason = fmt.Sprintf("shares %d %s with %s (%s)", shared, noun, seed.Name, strings.Join(sharedNames, ", "))
				} else {
					p.Reason = fmt.Sprintf("RAWG suggests for %s", seed.Name)
				}
				results = append(results, p)
			}

			view := suggestedView{
				Meta: suggestedMeta{
					Source: "live", DataOrigin: dataOrigin, Seed: seed.Name, SeedID: seed.ID,
					Note: note, Ambiguous: candidates, Count: len(results), Limit: limit,
				},
				Results: results,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(w, "No suggestions found for %q. Browse by genre: game-goat-pp-cli discover\n", seed.Name)
				return nil
			}
			items := make([]map[string]any, 0, len(results))
			for _, r := range results {
				rating := ""
				if r.Rating > 0 {
					rating = fmt.Sprintf("%.1f", r.Rating)
					if r.RatingsCount > 0 {
						rating = fmt.Sprintf("%s (%d)", rating, r.RatingsCount)
					}
				}
				backlogCell := ""
				if r.OnBacklog {
					backlogCell = "on backlog"
				}
				items = append(items, map[string]any{
					"suggested": r.Name,
					"rating":    rating,
					"genres":    strings.Join(r.Genres, ","),
					"backlog":   backlogCell,
					"reason":    r.Reason,
				})
			}
			return printAutoTable(w, items)
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin the title to a release year when remakes share a name (e.g. 2016)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum suggested games to return (1-20)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSuggestedCmd(flags))
	})
}
