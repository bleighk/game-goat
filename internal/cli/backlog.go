// backlog.go — hand-written Slice C novel command group (local store).
// pp:data-source local — the backlog is the user's local SQLite table
// (game_backlog); 'add' resolves titles live, everything else is local.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"game-goat-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// backlogNotDBHint is appended to every not-found error so agents get the
// actionable next step without reading the docs.
const backlogNotDBHint = "see: game-goat-pp-cli backlog list"

// backlogNotFoundErr is the local-store miss for backlog rows: exit code 6
// (declared via pp:typed-exit-codes), distinct from a live RAWG miss (exit 3)
// so agents can tell "not in YOUR data" from "not in RAWG".
func backlogNotFoundErr(err error) error {
	return &cliError{code: 6, err: err}
}

// openBacklogStoreForWrite opens the local store read-write (runs the
// game_backlog migration) at the default per-user path.
func openBacklogStoreForWrite(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("game-goat-pp-cli"))
}

// gameBacklogTableExists reports whether the game_backlog table is present.
// Read paths open the store read-only (no migrations run), so a database
// created before the backlog slice ships simply reads as "no backlog yet".
func gameBacklogTableExists(ctx context.Context, db *store.Store) (bool, error) {
	var count int
	if err := db.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='game_backlog'`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// backlogReadStore opens the local store read-only for backlog reads.
// Returns (nil, nil) when there is no database or no game_backlog table yet
// (empty backlog state, not an error).
func backlogReadStore(ctx context.Context) (*store.Store, error) {
	db, err := openStoreForRead(ctx, "game-goat-pp-cli")
	if err != nil || db == nil {
		return nil, err
	}
	exists, err := gameBacklogTableExists(ctx, db)
	if err != nil || !exists {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// backlogDBMissing reports whether the local store file does not exist yet.
func backlogDBMissing() bool {
	_, err := os.Stat(defaultDBPath("game-goat-pp-cli"))
	return os.IsNotExist(err)
}

// findBacklogRow resolves a title-or-id argument to one backlog row: numeric
// keys match rawg_id directly, anything else matches an exact
// (case-insensitive) title, most recently added first.
func findBacklogRow(ctx context.Context, db *store.Store, key string) (store.GameBacklogRow, bool, error) {
	key = strings.TrimSpace(key)
	if id, ok := parseGameID(key); ok {
		return db.GetGameBacklog(ctx, id)
	}
	matches, err := db.FindGameBacklogByTitle(ctx, key)
	if err != nil {
		return store.GameBacklogRow{}, false, err
	}
	if len(matches) == 0 {
		return store.GameBacklogRow{}, false, nil
	}
	return matches[0], true, nil
}

// backlogHoursCell formats hours played for human tables.
func backlogHoursCell(h float64) string {
	if h == 0 {
		return "-"
	}
	return strconv.FormatFloat(h, 'f', -1, 64)
}

// backlogDateCell shortens an RFC3339 timestamp to YYYY-MM-DD for tables.
func backlogDateCell(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// backlogTableRows renders list/search rows for the shared human table:
// title, status, hours, added_at.
func backlogTableRows(rows []store.GameBacklogRow) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{
			"title":  r.Title,
			"status": r.Status,
			"hours":  backlogHoursCell(r.Hours),
			"added":  backlogDateCell(r.AddedAt),
		})
	}
	return items
}

// slugifyTitle derives a RAWG-style slug for offline --id adds.
var slugNonAlnum = regexp.MustCompile(`[^a-z0-9-]+`)

func slugifyTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugNonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func newNovelBacklogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backlog",
		Short: "Manage your personal game backlog (local)",
		Long: `Manage your personal game backlog in the local store — a per-game
play-tracking table RAWG does not model (status, hours played, finished-at,
user rating, notes). Subcommands:

  add     <title>       resolve against RAWG and add to the backlog
  list                  list entries (--status, --sort, --limit)
  remove  <title-or-id> delete an entry
  show    <title-or-id> full detail for one entry
  search  <query>       substring search across title, slug, notes
  audit                 backlog health report (separate slice)

The backlog lives in the local SQLite store next to the synced RAWG cache;
add is the only subcommand that talks to RAWG (title resolution), and even
that can be skipped with --id.`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog add "Hollow Knight"
  game-goat-pp-cli backlog list --status in-progress
  game-goat-pp-cli backlog remove "Hollow Knight"
  game-goat-pp-cli backlog show 10394
  game-goat-pp-cli backlog search hollow --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBacklogAddCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBacklogListCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBacklogRemoveCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBacklogShowCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBacklogSearchCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBacklogAuditCmd(flags))
	return cmd
}

// ---- backlog add ----

type backlogAddMeta struct {
	Source         string               `json:"source"`
	AlreadyPresent bool                 `json:"already_present"`
	ResolvedBy     string               `json:"resolved_by"`
	Year           string               `json:"year,omitempty"`
	Ambiguous      []ambiguousCandidate `json:"ambiguous,omitempty"`
}

type backlogAddView struct {
	Meta    backlogAddMeta         `json:"meta"`
	Results []store.GameBacklogRow `json:"results"`
}

func newNovelBacklogAddCmd(flags *rootFlags) *cobra.Command {
	var idArg, titleOverride, year, status, notes string
	var hours, rating float64

	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a game to your backlog",
		Long: `Add a game to your local backlog. With a title, the game is resolved
against live RAWG search (exact-title first; remakes and other same-name
years are reported as a notice, pin with --year). With --id the title
search is skipped: --id + --title is a fully offline add, --id alone fetches
the game's name live by id. --rating records your RAWG-style 1-5 score
after finishing (0 = unrated). Re-adding a game already on the backlog with
explicit --status/--hours/--rating/--notes updates those fields in place; a
plain re-add is a no-op notice (already_present), never a duplicate.`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog add "Hollow Knight"
  game-goat-pp-cli backlog add "Yakuza 0" --status in-progress --hours 4
  game-goat-pp-cli backlog add "God of War" --year 2018 --json
  game-goat-pp-cli backlog add --id 10394 --title "Hollow Knight" --notes "weekly pick"
  game-goat-pp-cli backlog add "God of War" --status finished --rating 4.5
`, "\n"),
		Annotations: map[string]string{
			// add resolves titles live, then writes to the local store.
			// It mutates local state, so no mcp:read-only annotation (see
			// phase-11: mutating commands omit the read-only hint).
			"pp:data-source":      "live",
			"pp:happy-args":       "title=Hollow Knight;--dry-run",
			"pp:typed-exit-codes": "0,2,3,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backlog add")
			}

			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" && strings.TrimSpace(idArg) == "" {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--id <rawg-id>]",
					"a game title (or --id for a known RAWG id) is required")
			}
			if strings.TrimSpace(idArg) == "" && strings.TrimSpace(titleOverride) != "" {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title>",
					"--title is only used with --id (offline add); pass the title as the positional argument")
			}
			if strings.TrimSpace(idArg) != "" && strings.TrimSpace(year) != "" {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--id <rawg-id>]",
					"--year only applies to title resolution; use --id to pin the game directly")
			}
			if strings.TrimSpace(year) != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> [--year 2018]",
					"--year must be a four-digit year (YYYY)")
			}
			if status != "" && !store.ValidGameBacklogStatus(status) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --status backlog|in-progress|finished|dropped",
					fmt.Sprintf("unknown --status %q; expected backlog, in-progress, finished, or dropped", status))
			}
			if hours < 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --hours 4.5",
					"--hours cannot be negative")
			}
			if rating < 0 || rating > 5 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --rating 4.5",
					fmt.Sprintf("invalid --rating %.1f: expected 0 (unrated) to 5", rating))
			}
			if status == "" {
				status = store.GameBacklogStatusBacklog
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			meta := backlogAddMeta{Source: "live", ResolvedBy: "title"}
			in := store.GameBacklogInput{Status: status, Hours: hours, UserRating: rating, Notes: notes}

			if strings.TrimSpace(idArg) != "" {
				id, ok := parseGameID(strings.TrimSpace(idArg))
				if !ok {
					return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --id <rawg-id>",
						fmt.Sprintf("invalid --id %q: must be a positive RAWG game id", idArg))
				}
				in.RawgID = id
				// Offline add: --id plus a title (positional or --title)
				// skips every network call.
				offlineTitle := title
				if offlineTitle == "" {
					offlineTitle = strings.TrimSpace(titleOverride)
				}
				if offlineTitle != "" {
					in.Title = offlineTitle
					in.Slug = slugifyTitle(offlineTitle)
					meta.Source, meta.ResolvedBy = "local", "id-offline"
				} else {
					c, err := flags.newClient()
					if err != nil {
						return err
					}
					g, err := fetchGameByID(ctx, c, id)
					if err != nil {
						return err
					}
					in.Title, in.Slug = g.Name, g.Slug
					meta.ResolvedBy = "id"
				}
			} else {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				g, candidates, err := resolveTitleForMultiSource(ctx, cmd, c, flags, title, year)
				if err != nil {
					return err
				}
				in.RawgID, in.Title, in.Slug = g.ID, g.Name, g.Slug
				meta.Year = year
				meta.Ambiguous = candidates
			}

			db, err := openBacklogStoreForWrite(ctx)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			// Idempotent guard: same rawg_id already present -> notice, no write.
			if existing, ok, err := db.GetGameBacklog(ctx, in.RawgID); err != nil {
				return fmt.Errorf("reading backlog: %w", err)
			} else if ok {
				meta.AlreadyPresent = true
				// Plain re-add: no-op notice. Re-adding with explicit
				// --status/--hours/--rating/--notes updates those fields in
				// place (added_at preserved, finished_at follows the flip).
				upd := store.GameBacklogInput{
					RawgID: existing.RawgID, Title: existing.Title, Slug: existing.Slug,
					Status: existing.Status, Hours: existing.Hours,
					UserRating: existing.UserRating, Notes: existing.Notes,
				}
				changed := false
				if cmd.Flags().Changed("status") && status != existing.Status {
					upd.Status = status
					changed = true
				}
				if cmd.Flags().Changed("hours") && hours != existing.Hours {
					upd.Hours = hours
					changed = true
				}
				if cmd.Flags().Changed("rating") && rating != existing.UserRating {
					upd.UserRating = rating
					changed = true
				}
				if cmd.Flags().Changed("notes") && notes != existing.Notes {
					upd.Notes = notes
					changed = true
				}
				if !changed {
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						view := backlogAddView{Meta: meta, Results: []store.GameBacklogRow{existing}}
						return printJSONFiltered(cmd.OutOrStdout(), view, flags)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "already on backlog since %s (no duplicate written)\n",
						backlogDateCell(existing.AddedAt))
					fmt.Fprintf(cmd.OutOrStdout(), "%s is already on your backlog\n", existing.Title)
					return printAutoTable(cmd.OutOrStdout(), backlogTableRows([]store.GameBacklogRow{existing}))
				}
				stored, _, err := db.UpsertGameBacklog(ctx, upd)
				if err != nil {
					return err
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					view := backlogAddView{Meta: meta, Results: []store.GameBacklogRow{stored}}
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "updated existing backlog entry (added %s)\n",
					backlogDateCell(stored.AddedAt))
				fmt.Fprintf(cmd.OutOrStdout(), "updated %s (RAWG id %d, status %s)\n",
					stored.Title, stored.RawgID, stored.Status)
				return printAutoTable(cmd.OutOrStdout(), backlogTableRows([]store.GameBacklogRow{stored}))
			}

			stored, inserted, err := db.UpsertGameBacklog(ctx, in)
			if err != nil {
				return err
			}
			if !inserted {
				// Another writer added the same id between the guard and the
				// upsert; report it the same way as the guard path.
				meta.AlreadyPresent = true
			}

			view := backlogAddView{Meta: meta, Results: []store.GameBacklogRow{stored}}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s to backlog (RAWG id %d, status %s)\n",
				stored.Title, stored.RawgID, stored.Status)
			return printAutoTable(cmd.OutOrStdout(), backlogTableRows([]store.GameBacklogRow{stored}))
		},
	}

	cmd.Flags().StringVar(&idArg, "id", "", "RAWG game id: skip title resolution (--id + --title = fully offline add; --id alone fetches the title live)")
	cmd.Flags().StringVar(&titleOverride, "title", "", "explicit title override for offline adds with --id")
	cmd.Flags().StringVar(&year, "year", "", "four-digit release year to pin remakes during title resolution (YYYY)")
	cmd.Flags().StringVar(&status, "status", store.GameBacklogStatusBacklog, "backlog status: backlog, in-progress, finished, or dropped")
	cmd.Flags().Float64Var(&hours, "hours", 0, "hours already played at add time")
	cmd.Flags().StringVar(&notes, "notes", "", "free-form note stored with the entry")
	cmd.Flags().Float64Var(&rating, "rating", 0, "your RAWG-style 1-5 rating after finishing (0 = unrated)")
	return cmd
}

// ---- backlog list ----

type backlogListMeta struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
	Status string `json:"status,omitempty"`
	Sort   string `json:"sort"`
}

type backlogListView struct {
	Meta    backlogListMeta        `json:"meta"`
	Results []store.GameBacklogRow `json:"results"`
}

const backlogListDefaultLimit = 50
const backlogListMaxLimit = 500

func newNovelBacklogListCmd(flags *rootFlags) *cobra.Command {
	var status, sortBy string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List games on your backlog",
		Long: `List backlog entries from the local store, newest first by default.
--status filters to one backlog status; --sort reorders (added = newest
first, title = alphabetical, hours = most played first); --limit bounds the
output (default 50, capped at 500). An empty backlog is a normal result,
not an error.`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog list
  game-goat-pp-cli backlog list --status in-progress
  game-goat-pp-cli backlog list --sort hours --limit 10 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--limit=10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backlog list")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath(), "backlog list takes no positional arguments")
			}
			if status != "" && !store.ValidGameBacklogStatus(status) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --status backlog|in-progress|finished|dropped",
					fmt.Sprintf("unknown --status %q; expected backlog, in-progress, finished, or dropped", status))
			}
			switch sortBy {
			case "", "added", "title", "hours":
			default:
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --sort added|title|hours",
					fmt.Sprintf("unknown --sort %q; expected added, title, or hours", sortBy))
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "--limit must be at least 1")
			}
			if limit > backlogListMaxLimit {
				limit = backlogListMaxLimit
				fmt.Fprintf(cmd.ErrOrStderr(), "clamping --limit to %d\n", backlogListMaxLimit)
			}
			if sortBy == "" {
				sortBy = "added"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := []store.GameBacklogRow{}
			if db, err := backlogReadStore(ctx); err != nil {
				return fmt.Errorf("opening local store: %w", err)
			} else if db != nil {
				defer db.Close()
				rows, err = db.ListGameBacklog(ctx, store.ListGameBacklogFilter{
					Status: status, Sort: sortBy, Limit: limit,
				})
				if err != nil {
					return err
				}
			}

			view := backlogListView{
				Meta:    backlogListMeta{Source: "local", Count: len(rows), Status: status, Sort: sortBy},
				Results: rows,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				if status != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No %s games on your backlog. Add one: game-goat-pp-cli backlog add 'Hollow Knight'\n", status)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Your backlog is empty. Add one: game-goat-pp-cli backlog add 'Hollow Knight'")
				}
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), backlogTableRows(rows))
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "filter by backlog status: backlog, in-progress, finished, or dropped")
	cmd.Flags().StringVar(&sortBy, "sort", "added", "row order: added (newest first), title (alphabetical), or hours (most played first)")
	cmd.Flags().IntVar(&limit, "limit", backlogListDefaultLimit, fmt.Sprintf("maximum rows to print (default %d, capped at %d)", backlogListDefaultLimit, backlogListMaxLimit))
	return cmd
}

// ---- backlog remove ----

type backlogRemoveView struct {
	Removed bool                 `json:"removed"`
	RawgID  int                  `json:"rawg_id"`
	Title   string               `json:"title"`
	Row     store.GameBacklogRow `json:"row,omitempty"`
}

func newNovelBacklogRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <title-or-id>",
		Short: "Remove a game from your backlog",
		Long: `Remove one backlog entry. The argument is a RAWG game id or an exact
title (case-insensitive; duplicates of one title remove the most recently
added). No prompts: removal is immediate. There is no built-in undo —
re-add the game (backlog add, or add --id <id> --title <title> for a
fully offline restore) to get it back.

Not on the backlog (or no local store yet): typed error exit 6 with a hint
to list what IS there (see: game-goat-pp-cli backlog list).`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog remove "Hollow Knight"
  game-goat-pp-cli backlog remove 10394 --json
  # undo: game-goat-pp-cli backlog add --id 10394 --title "Hollow Knight"
`, "\n"),
		Annotations: map[string]string{
			// Mutates the local store: no mcp:read-only annotation.
			"pp:data-source":      "local",
			"pp:typed-exit-codes": "0,2,6",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backlog remove")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title-or-id>",
					"a RAWG game id or exact backlog title is required")
			}
			if len(args) > 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title-or-id>",
					"backlog remove takes exactly one title or id; quote titles with spaces")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			key := strings.Join(args, " ")
			if backlogDBMissing() {
				return backlogNotFoundErr(fmt.Errorf("no local backlog: %q is not on it; %s", key, backlogNotDBHint))
			}

			db, err := openBacklogStoreForWrite(ctx)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			row, ok, err := findBacklogRow(ctx, db, key)
			if err != nil {
				return err
			}
			if !ok {
				return backlogNotFoundErr(fmt.Errorf("%q is not on your backlog; %s", key, backlogNotDBHint))
			}

			removed, ok, err := db.RemoveGameBacklog(ctx, row.RawgID)
			if err != nil {
				return err
			}
			if !ok {
				return backlogNotFoundErr(fmt.Errorf("%q is not on your backlog; %s", key, backlogNotDBHint))
			}

			view := backlogRemoveView{Removed: true, RawgID: removed.RawgID, Title: removed.Title, Row: removed}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s from backlog (RAWG id %d; re-add with: %s add --id %d --title %q)\n",
				removed.Title, removed.RawgID, cmd.Root().Name(), removed.RawgID, removed.Title)
			return nil
		},
	}
	return cmd
}

// ---- backlog show ----

func newNovelBacklogShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <title-or-id>",
		Short: "Show full detail for one backlog entry",
		Long: `Show every stored column for one backlog entry: title, RAWG id, slug,
status, hours, added-at, finished-at, user rating, and notes. The argument
is a RAWG game id or an exact title (case-insensitive). Unknown entries
are a typed exit-6 miss (see: game-goat-pp-cli backlog list).`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog show "Hollow Knight"
  game-goat-pp-cli backlog show 10394 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "local",
			"pp:typed-exit-codes": "0,2,6",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backlog show")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title-or-id>",
					"a RAWG game id or exact backlog title is required")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			key := strings.Join(args, " ")
			db, err := backlogReadStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db != nil {
				defer db.Close()
			}

			var row store.GameBacklogRow
			var ok bool
			if db != nil {
				row, ok, err = findBacklogRow(ctx, db, key)
				if err != nil {
					return err
				}
			}
			if !ok {
				return backlogNotFoundErr(fmt.Errorf("%q is not on your backlog; %s", key, backlogNotDBHint))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), row, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (RAWG id %d)\n", row.Title, row.RawgID)
			fmt.Fprintf(w, "  status:     %s\n", row.Status)
			fmt.Fprintf(w, "  hours:      %s\n", backlogHoursCell(row.Hours))
			fmt.Fprintf(w, "  added:      %s\n", row.AddedAt)
			if row.FinishedAt != "" {
				fmt.Fprintf(w, "  finished:   %s\n", row.FinishedAt)
			} else {
				fmt.Fprintln(w, "  finished:   -")
			}
			if row.UserRating > 0 {
				fmt.Fprintf(w, "  rating:     %.1f / 5\n", row.UserRating)
			} else {
				fmt.Fprintln(w, "  rating:     - (unrated)")
			}
			if row.Notes != "" {
				fmt.Fprintf(w, "  notes:      %s\n", row.Notes)
			}
			if row.Slug != "" {
				fmt.Fprintf(w, "  slug:       %s\n", row.Slug)
			}
			return nil
		},
	}
	return cmd
}

// ---- backlog search ----

type backlogSearchMeta struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
	Query  string `json:"query"`
	Status string `json:"status,omitempty"`
}

type backlogSearchView struct {
	Meta    backlogSearchMeta      `json:"meta"`
	Results []store.GameBacklogRow `json:"results"`
}

func newNovelBacklogSearchCmd(flags *rootFlags) *cobra.Command {
	var status string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search your backlog by substring",
		Long: `Search backlog titles, slugs, and notes for a case-insensitive substring,
ranked title matches first, then slug, then notes, newest added first
within a tier. --status filters before searching. LIKE wildcards in the
query (% and _) match literally. Empty results are a normal state.`,
		Example: strings.Trim(`
  game-goat-pp-cli backlog search hollow
  game-goat-pp-cli backlog search "sekiro" --status backlog --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "query=Hollow Knight",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backlog search")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <query>",
					"a search substring is required")
			}
			if status != "" && !store.ValidGameBacklogStatus(status) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --status backlog|in-progress|finished|dropped",
					fmt.Sprintf("unknown --status %q; expected backlog, in-progress, finished, or dropped", status))
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "--limit must be at least 1")
			}
			if limit > backlogListMaxLimit {
				limit = backlogListMaxLimit
				fmt.Fprintf(cmd.ErrOrStderr(), "clamping --limit to %d\n", backlogListMaxLimit)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			query := strings.Join(args, " ")
			rows := []store.GameBacklogRow{}
			if db, err := backlogReadStore(ctx); err != nil {
				return fmt.Errorf("opening local store: %w", err)
			} else if db != nil {
				defer db.Close()
				rows, err = db.SearchGameBacklog(ctx, query, status, limit)
				if err != nil {
					return err
				}
			}

			view := backlogSearchView{
				Meta:    backlogSearchMeta{Source: "local", Count: len(rows), Query: query, Status: status},
				Results: rows,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No backlog entries matched %q. See everything: game-goat-pp-cli backlog list\n", query)
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), backlogTableRows(rows))
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "filter results by backlog status: backlog, in-progress, finished, or dropped")
	cmd.Flags().IntVar(&limit, "limit", backlogListDefaultLimit, fmt.Sprintf("maximum rows to print (default %d, capped at %d)", backlogListDefaultLimit, backlogListMaxLimit))
	return cmd
}

// ---- shared helpers for the queue command (queue.go) ----

// backlogRowsForQueue loads every non-finished backlog row in pick-priority
// order for the next-play queue: in-progress (least played first), then
// backlog-status (longest waiting first), then dropped (longest waiting
// first). Finished rows are never queued.
func backlogRowsForQueue(ctx context.Context) ([]store.GameBacklogRow, error) {
	db, err := backlogReadStore(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return []store.GameBacklogRow{}, nil
	}
	defer db.Close()

	rows, err := db.ListGameBacklog(ctx, store.ListGameBacklogFilter{
		Sort: "added", Limit: 500,
	})
	if err != nil {
		return nil, err
	}

	var inProgress, waiting, dropped []store.GameBacklogRow
	for _, r := range rows {
		switch r.Status {
		case store.GameBacklogStatusInProgress:
			inProgress = append(inProgress, r)
		case store.GameBacklogStatusBacklog:
			waiting = append(waiting, r)
		case store.GameBacklogStatusDropped:
			dropped = append(dropped, r)
		} // finished: never queued
	}
	sort.SliceStable(inProgress, func(i, j int) bool {
		if inProgress[i].Hours != inProgress[j].Hours {
			return inProgress[i].Hours < inProgress[j].Hours
		}
		return inProgress[i].AddedAt < inProgress[j].AddedAt
	})
	// waiting and dropped come back added_at DESC from the store;
	// waiting-longest-first is the ascending order.
	reverseRows(waiting)
	reverseRows(dropped)

	out := make([]store.GameBacklogRow, 0, len(inProgress)+len(waiting)+len(dropped))
	out = append(out, inProgress...)
	out = append(out, waiting...)
	out = append(out, dropped...)
	return out, nil
}

func reverseRows(rows []store.GameBacklogRow) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}
