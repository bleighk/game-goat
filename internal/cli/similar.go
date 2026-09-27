// similar.go — hand-written Slice D novel command (top-level).
// pp:data-source live — resolve one game, then match candidates by the
// seed's rarest gameplay tags (defining mechanics: roguelite, bullet hell,
// loot...), falling back to a shared-genre join when the seed carries no
// usable tags. RAWG has no "similar" endpoint on the free tier, so the
// join is by design — tag-first keeps it a mechanics match, distinct from
// 'suggested''s broader genre-join fallback.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"game-goat-pp-cli/internal/client"

	"github.com/spf13/cobra"
)

// ----- pure helpers (unit-tested) -----

// genreIDList extracts the RAWG genre ids of a game for /games?genres= joins.
func genreIDList(g rawgGame) []int {
	ids := make([]int, 0, len(g.Genres))
	for _, ref := range g.Genres {
		ids = append(ids, ref.ID)
	}
	return ids
}

// nonGameplayTags are tag names that describe platform metadata or audio
// options rather than what playing the game feels like. They carry no
// similarity signal.
var nonGameplayTags = map[string]bool{
	"singleplayer": true, "multiplayer": false, // multiplayer IS a signal
	"controller": true, "full controller support": true,
	"family sharing": true, "stats": true,
	"stereo sound": true, "surround sound": true,
	"playable without timed input": true, "camera comfort": true,
	"steam timeline": true,
}

// isGameplayTag reports whether a RAWG tag describes gameplay (mechanics,
// presentation, mood) rather than Steam-platform metadata.
func isGameplayTag(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || nonGameplayTags[n] {
		return false
	}
	if strings.HasPrefix(n, "steam ") || strings.HasPrefix(n, "remote play") {
		return false
	}
	return true
}

// tagRef pairs a gameplay tag with RAWG's games_count for the tag.
type tagRef struct {
	ID    int
	Name  string
	Count int
}

// mechanicsTags names RAWG tags that describe HOW a game plays rather than
// its trappings. When the seed carries one of these, it is the identity to
// match: a roguelite's neighbors are roguelites, not "games with loot".
// Same curation spirit as the moods vocabulary.
var mechanicsTags = map[string]bool{
	"roguelike": true, "roguelite": true, "bullet hell": true,
	"hack and slash": true, "metroidvania": true, "platformer": true,
	"souls-like": true, "survival": true, "open world": true,
	"sandbox": true, "tower defense": true, "dungeon crawler": true,
	"city builder": true, "farming simulator": true, "farming sim": true,
	"deckbuilder": true, "deck-building": true, "turn-based tactics": true,
	"turn-based strategy": true, "real-time strategy": true, "grand strategy": true,
	"4x": true, "battle royale": true, "looter shooter": true,
	"immersive sim": true, "stealth": true, "shoot 'em up": true, "shmup": true,
	"beat 'em up": true, "run and gun": true, "twin stick": true,
	"colony sim": true, "base building": true, "automation": true,
	"management": true, "trading card game": true, "action rpg": true,
	"jrpg": true, "crpg": true, "mmorpg": true, "psychological horror": true,
	"survival horror": true, "racing": true, "flight": true, "life sim": true,
}

// isMechanicsTag reports whether a gameplay tag names the game's core loop.
func isMechanicsTag(name string) bool {
	return mechanicsTags[strings.ToLower(strings.TrimSpace(name))]
}

// gameplayTagsByRarity returns the seed's gameplay tags ordered rarest
// first. games_count is RAWG's own census: a tag shared by 2,900 games
// (Loot) describes the game far more specifically than one shared by
// 99,660 (Pixel Graphics) — the rarest tags are the defining mechanics,
// and rarity ordering surfaces them without any hand-curated vocabulary.
func gameplayTagsByRarity(g rawgGame) []tagRef {
	tags := make([]tagRef, 0, len(g.Tags))
	for _, ref := range g.Tags {
		if !isGameplayTag(ref.Name) {
			continue
		}
		tags = append(tags, tagRef{ID: ref.ID, Name: ref.Name, Count: ref.GamesCount})
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].Count < tags[j].Count })
	return tags
}

// mechanicsClusterNeighborhood probes the seed's mechanics-vocabulary tags
// (independent -added top-20 per tag) and returns the neighborhood of the
// tag whose results co-occur most with the other tags' neighborhoods — the
// seed's identity cluster (megabonk: roguelike+roguelite+bullet-hell beat a
// lone hack-and-slash trapping). Probe sets are independent because a game
// appearing in several tags' neighborhoods IS the signal; RAWG's comma-
// separated tags= is a union, so single-tag queries are the tightest match
// the API offers. fill is the caller's list size; a tag that fills it
// outranks higher co-occurrence on a smaller set. Callers own dedupe and
// ordering. Shared by 'similar' and 'suggested'.
func mechanicsClusterNeighborhood(ctx context.Context, cmd *cobra.Command, c *client.Client, flags *rootFlags, seed rawgGame, fill int) ([]rawgGame, tagRef, bool, error) {
	seedTags := gameplayTagsByRarity(seed)
	var mechTags []tagRef
	for _, t := range seedTags {
		if isMechanicsTag(t.Name) {
			mechTags = append(mechTags, t)
		}
	}
	if len(mechTags) > 4 {
		mechTags = mechTags[:4] // rarity-ascending; enough to find the cluster
	}
	if len(mechTags) == 0 {
		return nil, tagRef{}, false, nil
	}
	filterSeed := func(gs []rawgGame) []rawgGame {
		out := make([]rawgGame, 0, len(gs))
		for _, g := range gs {
			if g.ID != 0 && g.ID != seed.ID && !(g.Slug != "" && g.Slug == seed.Slug) {
				out = append(out, g)
			}
		}
		return out
	}
	sets := make([][]rawgGame, len(mechTags))
	idSets := make([]map[int]bool, len(mechTags))
	for i, t := range mechTags {
		gs, _, err := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
			"tags":      strconv.Itoa(t.ID),
			"ordering":  "-added",
			"page_size": "20",
		})
		if err != nil {
			return nil, tagRef{}, false, err
		}
		sets[i] = filterSeed(gs)
		idSets[i] = make(map[int]bool, len(sets[i]))
		for _, g := range sets[i] {
			idSets[i][g.ID] = true
		}
	}
	bestIdx, bestScore := -1, -1
	for i := range mechTags {
		score := 0
		for j := range mechTags {
			if i == j {
				continue
			}
			for _, g := range sets[i] {
				if idSets[j][g.ID] {
					score++
				}
			}
		}
		if len(sets[i]) >= fill && score > bestScore {
			bestIdx, bestScore = i, score
		}
	}
	// No cluster tag filled the list: keep the largest usable set.
	if bestIdx == -1 {
		for i := range mechTags {
			if len(sets[i]) > 0 && (bestIdx == -1 || len(sets[i]) > len(sets[bestIdx])) {
				bestIdx = i
			}
		}
	}
	if bestIdx == -1 || len(sets[bestIdx]) == 0 {
		return nil, tagRef{}, false, nil
	}
	return sets[bestIdx], mechTags[bestIdx], true, nil
}

// scoreSimilarity ranks a candidate against a source game: shared-genre
// count first, then metacritic (nil loses to any score), then RAWG rating.
// Pure: unit-tested.
type similarityScore struct {
	Shared     int
	Metacritic float64
	Rating     float64
}

func scoreSimilarity(candidate rawgGame, sourceGenres map[string]bool) similarityScore {
	shared := 0
	for _, ref := range candidate.Genres {
		if sourceGenres[strings.ToLower(ref.Name)] {
			shared++
		}
	}
	meta := 0.0
	if candidate.Metacritic != nil {
		meta = float64(*candidate.Metacritic)
	}
	return similarityScore{Shared: shared, Metacritic: meta, Rating: candidate.Rating}
}

func lessSimilarity(a, b similarityScore) bool {
	if a.Shared != b.Shared {
		return a.Shared > b.Shared
	}
	if a.Metacritic != b.Metacritic {
		return a.Metacritic > b.Metacritic
	}
	return a.Rating > b.Rating
}

// ----- view -----

type similarResult struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Released   string   `json:"released,omitempty"`
	Rating     float64  `json:"rating"`
	Metacritic *int     `json:"metacritic,omitempty"`
	Genres     []string `json:"genres"`
	OnBacklog  bool     `json:"on_backlog"`
	Reason     string   `json:"reason,omitempty"`
}

type similarMeta struct {
	Source     string               `json:"source"`
	DataOrigin string               `json:"data_origin"` // shared-genre-join
	Seed       string               `json:"seed"`
	Note       string               `json:"note,omitempty"`
	Ambiguous  []ambiguousCandidate `json:"ambiguous,omitempty"`
	Count      int                  `json:"count"`
	Limit      int                  `json:"limit"`
}

type similarView struct {
	Meta    similarMeta     `json:"meta"`
	Results []similarResult `json:"results"`
}

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var year string
	var limit int

	cmd := &cobra.Command{
		Use:   "similar <title>",
		Short: "Games like <title>, matched by defining gameplay tags",
		Long: `Find games similar to one you name. The seed title is resolved
against RAWG (remake collisions are flagged as ambiguous; pin with --year),
then candidates are matched by the seed's rarest gameplay tags — RAWG's own
games_count per tag surfaces the defining mechanics (Loot, Action Roguelike)
over broad descriptors (Action, Pixel Graphics) — ordered by community
rating. Seeds with no usable tags fall back to a shared-genre join. Every
result carries a reason and an on_backlog flag from your local store.

RAWG has no free-tier "similar games" endpoint, so the join is by design:
'similar' matches mechanics via tags; 'suggested' takes the broader genre
join when its business-tier endpoint denies a free key.`,
		Example: strings.Trim(`
  game-goat-pp-cli similar "Hollow Knight"
  game-goat-pp-cli similar "God of War" --year 2018
  game-goat-pp-cli similar "Hollow Knight" --limit 5 --json --select results.name,results.reason
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "title=Hollow Knight;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "similar")
			}
			if len(args) != 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--year <yyyy>]", "similar takes exactly one quoted game title, e.g. similar \"Hollow Knight\"")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2018")
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

			sourceGenres := map[string]bool{}
			for _, ref := range seed.Genres {
				sourceGenres[strings.ToLower(ref.Name)] = true
			}

			backlogIDs, _, bidErr := backlogGameIDs(ctx)
			if bidErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog for on-backlog flags: %v\n", bidErr)
			}
			// Mechanics match first: query /games by the seed's rarest
			// gameplay tags (its defining mechanics). A genre join calls
			// The Last of Us "similar" to an indie roguelite because both
			// are Action; the tag join finds the roguelites. Ladder from
			// tight (3 rarest tags intersected) to broad (1 tag), keeping
			// the first intersection that fills the list.
			seedTags := gameplayTagsByRarity(seed)
			dataOrigin := "tag-mechanics-join"
			note := "RAWG has no free-tier similar endpoint; results match the seed's defining gameplay tags (rarest first), ordered by community rating"
			var usedTags []tagRef
			pool := make([]rawgGame, 0, limit+4)
			seen := map[int]bool{seed.ID: true}
			addUnseen := func(gs []rawgGame) []rawgGame {
				out := make([]rawgGame, 0, len(gs))
				for _, g := range gs {
					if g.ID != 0 && !seen[g.ID] && !(g.Slug != "" && g.Slug == seed.Slug) {
						seen[g.ID] = true
						out = append(out, g)
					}
				}
				return out
			}
			// Mechanics cluster: the seed's identity tags, resolved by
			// the shared neighborhood probe (see mechanicsClusterNeighborhood
			// for the co-occurrence scoring rationale).
			if nb, tag, ok, nerr := mechanicsClusterNeighborhood(ctx, cmd, c, flags, seed, limit); nerr != nil {
				return nerr
			} else if ok {
				usedTags = []tagRef{tag}
				pool = append(pool, addUnseen(nb)...)
			}
			// No mechanics signal at all: fall back to the rarest generic
			// gameplay tag, single-tag query, -added canon.
			if len(pool) == 0 {
				for _, t := range seedTags {
					gs, _, terr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
						"tags":      strconv.Itoa(t.ID),
						"ordering":  "-added",
						"page_size": "20",
					})
					if terr != nil {
						return terr
					}
					gs = addUnseen(gs)
					if len(gs) > 0 {
						usedTags = []tagRef{t}
						pool = append(pool, gs...)
						break
					}
				}
			}
			// Genre join: broader neighbors. Fills the list when tag
			// intersections run narrow, and replaces them entirely when
			// the seed has no gameplay tags.
			if len(pool) < limit && len(genreIDList(seed)) > 0 {
				if len(pool) == 0 {
					dataOrigin = "shared-genre-join"
					note = "RAWG has no free-tier similar endpoint and this seed has no usable gameplay tags; results are a shared-genre join (metacritic + rating tiebreak)"
				}
				gs, _, gerr := fetchGamesResults(ctx, cmd, c, flags, "live", map[string]string{
					"genres":    idCSV(genreIDList(seed)),
					"ordering":  "-rating",
					"page_size": "20",
				})
				if gerr != nil {
					return gerr
				}
				// No seed tags and nothing from the tag tiers: the genre
				// join is the only signal. Hide single-broad-genre rows —
				// the popularity canon, not seed-specific matches (F-U16).
				if len(pool) == 0 && len(seedTags) == 0 {
					filtered := make([]rawgGame, 0, len(gs))
					for _, g := range gs {
						if !sharesOnlyBroadGenre(g, sourceGenres) {
							filtered = append(filtered, g)
						}
					}
					if len(filtered) != len(gs) {
						note += "; weak single-broad-genre matches are hidden"
					}
					gs = filtered
				}
				pool = append(pool, addUnseen(gs)...)
			}
			if len(pool) > limit {
				pool = pool[:limit]
			}

			results := make([]similarResult, 0, len(pool))
			for _, g := range pool {
				genres := refNames(g.Genres)
				reason := ""
				if dataOrigin == "tag-mechanics-join" && len(usedTags) > 0 {
					names := make([]string, 0, len(usedTags))
					for _, t := range usedTags {
						names = append(names, t.Name)
					}
					reason = fmt.Sprintf("matches %s's defining tags (%s)", seed.Name, strings.Join(names, ", "))
				} else {
					shared := make([]string, 0, len(genres))
					for _, name := range genres {
						if sourceGenres[strings.ToLower(name)] {
							shared = append(shared, name)
						}
					}
					if len(shared) > 0 {
						noun := "genres"
						if len(shared) == 1 {
							noun = "genre"
						}
						reason = fmt.Sprintf("shares %d %s with %s (%s)", len(shared), noun, seed.Name, strings.Join(shared, ", "))
					}
				}
				results = append(results, similarResult{
					ID: g.ID, Name: g.Name, Released: g.Released,
					Rating: g.Rating, Metacritic: g.Metacritic, Genres: genres,
					OnBacklog: backlogIDs[g.ID], Reason: reason,
				})
			}
			view := similarView{
				Meta: similarMeta{
					Source: "live", DataOrigin: dataOrigin, Seed: seed.Name,
					Note:      note,
					Ambiguous: candidates, Count: len(results), Limit: limit,
				},
				Results: results,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(w, "No similar games found for %q. Browse by genre: game-goat-pp-cli discover\n", seed.Name)
				return nil
			}
			items := make([]map[string]any, 0, len(results))
			for _, r := range results {
				rating := ""
				if r.Rating > 0 {
					rating = fmt.Sprintf("%.1f", r.Rating)
				}
				items = append(items, map[string]any{
					"similar": r.Name,
					"rating":  rating,
					"genres":  strings.Join(r.Genres, ","),
					"reason":  r.Reason,
				})
			}
			return printAutoTable(w, items)
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin the title to a release year when remakes share a name (e.g. 2018)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum similar games to return (1-20)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSimilarCmd(flags))
	})
}
