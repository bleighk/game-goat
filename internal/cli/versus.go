// versus.go — hand-written Slice B novel command (top-level).
// pp:data-source live — two resolved RAWG detail records + keyless Steam
// enrichment per side, with a defensive local backlog join.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strings"

	"game-goat-pp-cli/internal/source/steam"

	"github.com/spf13/cobra"
)

type versusSide struct {
	Name         string             `json:"name"`
	Released     string             `json:"released"`
	RawgID       int                `json:"rawg_id"`
	Rating       float64            `json:"rating"`
	RatingsCount int                `json:"ratings_count"`
	Metacritic   *int               `json:"metacritic"`
	Playtime     int                `json:"playtime"`
	TopGenre     string             `json:"top_genre"`
	OnBacklog    bool               `json:"on_backlog"`
	Steam        *steam.SteamReview `json:"steam,omitempty"`
}

type versusDimension struct {
	Dimension string `json:"dimension"`
	Leader    string `json:"leader"`
	Detail    string `json:"detail,omitempty"`
}

type versusComparison struct {
	A       versusSide        `json:"a"`
	B       versusSide        `json:"b"`
	Verdict []versusDimension `json:"verdict"`
}

type versusMeta struct {
	Source           string               `json:"source"`
	Ambiguous        []ambiguousCandidate `json:"ambiguous,omitempty"`
	SourcesMissing   []string             `json:"sources_missing,omitempty"`
	BacklogAvailable bool                 `json:"backlog_available"`
}

type versusView struct {
	Meta    versusMeta         `json:"meta"`
	Results []versusComparison `json:"results"`
}

func toVersusSide(g rawgGame, review *steam.SteamReview, backlogIDs map[int]bool, backlogOK bool) versusSide {
	side := versusSide{
		Name:         g.Name,
		Released:     g.Released,
		RawgID:       g.ID,
		Rating:       g.Rating,
		RatingsCount: g.RatingsCount,
		Metacritic:   g.Metacritic,
		Playtime:     g.Playtime,
		OnBacklog:    backlogOK && backlogIDs != nil && backlogIDs[g.ID],
		Steam:        review,
	}
	if len(g.Genres) > 0 {
		side.TopGenre = g.Genres[0].Name
	}
	return side
}

func steamScore(side versusSide) (float64, bool) {
	if side.Steam == nil || side.Steam.Total <= 0 {
		return 0, false
	}
	return side.Steam.Score, true
}

// buildVersusVerdict names the leader per comparable dimension. Dimensions
// where either side lacks data are skipped (or single-sided, when only one
// side has a score); playtime leads toward the SHORTER game.
func buildVersusVerdict(a, b versusSide) []versusDimension {
	verdict := make([]versusDimension, 0, 5)
	leader := func(dim string, av, bv float64, higherWins bool, detail string) versusDimension {
		name := "tie"
		switch {
		case av > bv:
			name = a.Name
			if !higherWins {
				name = b.Name
			}
		case bv > av:
			name = b.Name
			if !higherWins {
				name = a.Name
			}
		}
		return versusDimension{Dimension: dim, Leader: name, Detail: detail}
	}
	verdict = append(verdict, leader("rating", a.Rating, b.Rating, true,
		fmt.Sprintf("%.1f vs %.1f (RAWG community, 0-5)", a.Rating, b.Rating)))
	verdict = append(verdict, leader("ratings_count", float64(a.RatingsCount), float64(b.RatingsCount), true,
		fmt.Sprintf("%d vs %d ratings", a.RatingsCount, b.RatingsCount)))
	if a.Metacritic != nil && b.Metacritic != nil {
		verdict = append(verdict, leader("metacritic", float64(*a.Metacritic), float64(*b.Metacritic), true,
			fmt.Sprintf("%d vs %d (0-100)", *a.Metacritic, *b.Metacritic)))
	} else {
		missing := "neither game"
		if a.Metacritic != nil {
			missing = b.Name
		} else if b.Metacritic != nil {
			missing = a.Name
		}
		verdict = append(verdict, versusDimension{Dimension: "metacritic", Leader: "", Detail: "no metacritic score for " + missing})
	}
	if a.Playtime > 0 && b.Playtime > 0 {
		verdict = append(verdict, leader("playtime", float64(a.Playtime), float64(b.Playtime), false,
			fmt.Sprintf("~%dh vs ~%dh (shorter leads)", a.Playtime, b.Playtime)))
	}
	sa, okA := steamScore(a)
	sb, okB := steamScore(b)
	switch {
	case okA && okB:
		verdict = append(verdict, leader("steam_score", sa, sb, true,
			fmt.Sprintf("%.0f%% vs %.0f%% (%s vs %s)", sa, sb, a.Steam.Desc, b.Steam.Desc)))
	case okA || okB:
		verdict = append(verdict, versusDimension{Dimension: "steam_score", Leader: "", Detail: "only one game has steam reviews"})
	default:
		verdict = append(verdict, versusDimension{Dimension: "steam_score", Leader: "", Detail: "no steam review data for either game"})
	}
	return verdict
}

func renderVersus(cmd *cobra.Command, cmp versusComparison, meta versusMeta) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s (%s", cmp.A.Name, orDash(cmp.A.Released))
	if cmp.A.TopGenre != "" {
		fmt.Fprintf(w, ", %s", cmp.A.TopGenre)
	}
	fmt.Fprintf(w, ")\n")
	fmt.Fprintf(w, "vs\n")
	fmt.Fprintf(w, "%s (%s", cmp.B.Name, orDash(cmp.B.Released))
	if cmp.B.TopGenre != "" {
		fmt.Fprintf(w, ", %s", cmp.B.TopGenre)
	}
	fmt.Fprintf(w, ")\n\n")
	rows := make([]map[string]any, 0, len(cmp.Verdict))
	for _, v := range cmp.Verdict {
		leader := v.Leader
		if leader == "" || leader == "tie" {
			leader = orDash(leader)
		}
		rows = append(rows, map[string]any{
			"dimension": v.Dimension,
			"detail":    v.Detail,
			"leader":    leader,
		})
	}
	if err := printAutoTable(w, rows); err != nil {
		return err
	}
	aLeads, bLeads, ties := 0, 0, 0
	for _, v := range cmp.Verdict {
		switch v.Leader {
		case cmp.A.Name:
			aLeads++
		case cmp.B.Name:
			bLeads++
		case "tie":
			ties++
		}
	}
	fmt.Fprintf(w, "verdict: %s leads %d, %s leads %d, %d tie\n", cmp.A.Name, aLeads, cmp.B.Name, bLeads, ties)
	backlogA, backlogB := "not on backlog", "not on backlog"
	if cmp.A.OnBacklog {
		backlogA = "on backlog"
	}
	if cmp.B.OnBacklog {
		backlogB = "on backlog"
	}
	fmt.Fprintf(w, "backlog: %s: %s; %s: %s\n", cmp.A.Name, backlogA, cmp.B.Name, backlogB)
	if len(meta.SourcesMissing) > 0 {
		fmt.Fprintf(w, "missing sources: %s\n", strings.Join(meta.SourcesMissing, ", "))
	}
	return nil
}

func newVersusCmd(flags *rootFlags) *cobra.Command {
	var year string

	cmd := &cobra.Command{
		Use:   "versus <titleA> <titleB>",
		Short: "Head-to-head comparison of two games",
		Long: `Resolve two games by title and compare them side by side: RAWG rating,
ratings count, metacritic, playtime, release date, top genre, keyless Steam
review summary, and a backlog flag from your local store. A verdict line
names which game leads each dimension (or a tie); playtime leads toward the
shorter game. A failed Steam lookup degrades that side and lists "steam" in
sources_missing. Titles shared by remakes are flagged as ambiguous; pin
with --year.`,
		Example: strings.Trim(`
  game-goat-pp-cli versus "Elden Ring" "Dark Souls III"
  game-goat-pp-cli versus "God of War" "God of War Ragnarok" --year 2018
  game-goat-pp-cli versus "Elden Ring" "Dark Souls III" --json --select results.verdict
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "titleA=Elden Ring;titleB=Dark Souls III;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "versus")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) < 2 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <titleA> <titleB> [--year <yyyy>]", "two game titles are required for a head-to-head comparison")
			}
			if len(args) > 2 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <titleA> <titleB>", "versus takes exactly two quoted titles, e.g. versus \"Elden Ring\" \"Dark Souls III\"")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <titleA> <titleB> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2018")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			gameA, candA, err := fetchResolvedGameDetail(ctx, cmd, c, flags, args[0], year)
			if err != nil {
				return err
			}
			gameB, candB, err := fetchResolvedGameDetail(ctx, cmd, c, flags, args[1], year)
			if err != nil {
				return err
			}
			if gameA.ID == gameB.ID {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <titleA> <titleB> [--year <yyyy>]",
					fmt.Sprintf("both titles resolved to the same game (%q, RAWG id %d) — nothing to compare; pass two different titles", gameA.Name, gameA.ID))
			}
			backlogIDs, backlogAvailable, backlogErr := backlogGameIDs(ctx)
			if backlogErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read local backlog: %v\n", backlogErr)
				backlogAvailable = false
			}
			meta := versusMeta{
				Source:           "live",
				BacklogAvailable: backlogAvailable && backlogErr == nil,
			}
			// Per-title Steam enrichment; each side degrades independently.
			steamA, serrA := steamReviewGraceful(ctx, steamLookupName(args[0], gameA))
			if serrA != nil {
				meta.SourcesMissing = append(meta.SourcesMissing, "steam")
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam enrichment unavailable for %q: %v\n", args[0], serrA)
			}
			steamB, serrB := steamReviewGraceful(ctx, steamLookupName(args[1], gameB))
			if serrB != nil {
				meta.SourcesMissing = append(meta.SourcesMissing, "steam")
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam enrichment unavailable for %q: %v\n", args[1], serrB)
			}
			if candidates := append(candA, candB...); candidates != nil {
				meta.Ambiguous = candidates
			}
			sideA := toVersusSide(gameA, steamA, backlogIDs, backlogAvailable)
			sideB := toVersusSide(gameB, steamB, backlogIDs, backlogAvailable)
			cmp := versusComparison{
				A:       sideA,
				B:       sideB,
				Verdict: buildVersusVerdict(sideA, sideB),
			}
			view := versusView{Meta: meta, Results: []versusComparison{cmp}}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderVersus(cmd, cmp, meta)
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin both titles to a release year when remakes share a name (e.g. 2018)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newVersusCmd(flags))
	})
}
