# Printing Press Retro: game-goat

## Session Stats
- API: game-goat (RAWG)
- Spec source: docs (RAWG API documentation, research brief + absorbed manifest)
- Scorecard: 90/100 (grade A; live-check capped — flagship RAWG calls 401 without credential)
- Verify pass rate: 100% (80/80)
- Fix loops: 3 documented (build fix, shipcheck, post-UAT acceptance re-run)
- Manual code edits: heavy — full local-store novel-command suite, Steam cross-source bridge, G104 fixes, UAT-driven fixes
- Features built from scratch: 10+ hand-authored novel commands (backlog, moods, finishline, deliver, export, retention, sync-hint, plus learn/playbook framework wiring and the Steam source bridge)

## Findings

### 1. Bare boolean `--flag` tokens in `pp:happy-args` are emitted space-separated and false-fail the live matrix (scorer bug)
- **What happened:** Three hand-authored novel commands (`backlog audit`, `finishline`, `moods list`) annotated with `pp:happy-args: "...;--json"` (bare bool flag, exactly the form the skill documents) produced 6 false failures in the keyed live matrix — 3 happy_path + 3 json_fidelity — because the probe actually ran `--json true`: cobra leaves a stray `true` positional, and these NoArgs commands exit 2 on it. The run burned a decode cycle diagnosing a defect that did not exist in the CLI.
- **Scorer correct?** No. The dogfood harness miscompiled its own documented input grammar.
- **Root cause:** Scorer — `parseHappyArgsAnnotation` (internal/pipeline runtime_commands.go:159-162) maps a bare `--flag` token to the pair `["--flag", "true"]`, and `overlayLiveDogfoodFlags` (live_dogfood.go:2719-2720, 2729) emits that pair as two space-separated argv tokens. The skill documents the contract as "bare `--flag` tokens are treated as boolean `--flag=true`" (printing-press/phases/11-build-the-goat.md:102; references/spec-format.md:170) — but the implementation emits `--flag true`, which cobra never parses as `--flag=true` for a bool flag. No input exists for which the bare-form emission is correct.
- **Cross-API check:** Yes — any CLI whose authoring agent writes the documented bare form. A 40-file sample of the 205 library files combining happy-args with `--json`/`--dry-run` found only `=`-forms in the wild, so library exposure is currently latent; the trigger is the press's own documented grammar, which the game-goat agent followed verbatim. Confirmed present in current press main source (read directly).
- **Frequency:** every CLI whose author uses the documented bare form; silent positional-corruption variant on positional-taking commands.
- **Fallback if the Printing Press doesn't fix it:** Write `--json=true` in every hand-authored annotation (reliable but undocumented contract — every future run must rediscover it).
- **Worth a Printing Press fix?** Yes — the parser produces args cobra can never interpret as intended, then scores the resulting failure against the CLI.
- **Inherent or fixable:** Fixable — emit a single `--flag=true` token (matching the documented contract), or reject the bare form loudly at parse time.
- **Durable fix:** In happy-args parsing/overlay, emit bare bool flags as one `--flag=true` argv token. Verify against cobra bool-flag semantics; add a regression test with a NoArgs command annotated with a bare `--json`.
- **Test:** positive: NoArgs command with `pp:happy-args: "--json"` passes its keyed live-matrix happy_path and json_fidelity legs; negative: `--flag=value` tokens unchanged, `--flag=-12.3` negative-numeric path unchanged.
- **Evidence:** game-goat keyed live matrix (run 20260927-155341-72694577), 6 false failures traced to `--json true` in probe argv; fixed CLI-side by rewriting annotations to `--json=true`, after which acceptance matrix passed 209/209 (163 skips).
- **Related prior retros:**
  - `game-goat` prior-run retro (20260927-092504, same day) — `extends`. Not surfaced there because that run had no live key; the keyed matrix is what exercises hand-authored happy-args.

### 2. Dogfood skip markers lack the source fingerprint promote recomputes — 4th recurrence, recovery is manual (scorer bug)
- **What happened:** The mid-pipeline Phase 18 sanctioned skip (`auth_required_no_credential`, RAWG 401s with no live key) was recorded without the acceptance fingerprint. When the keyed acceptance re-run recomputed the fingerprint, promote rejected the marker as stale and the fingerprint had to be hand-injected to clear the gate — the documented recovery, again.
- **Scorer correct?** Partially. The fingerprint guard correctly caught a stale marker (it should fire); the marker writer just never records the value the guard wants.
- **Root cause:** Scorer — the dogfood/skip-marker writer and the promote/acceptance fingerprint computation are not parity paths; skip markers carry no fingerprint, so every sanctioned skip goes stale the moment any source edit lands between dogfood and promote.
- **Cross-API check:** Yes — companies-house (retro #4457), plus two earlier recurrences cited there; game-goat is the 4th CLI to hit it. Confirmed unfixed on current main.
- **Frequency:** every run that records a sanctioned skip and then edits source (i.e., most nontrivial runs).
- **Fallback if the Printing Press doesn't fix it:** Hand-inject the recomputed fingerprint into the archived marker (reliable, documented, and repeatedly exercised — but it is manual surgery on a proof artifact every time).
- **Worth a Printing Press fix?** Yes — the parity gap is a mechanical writer/reader mismatch, not a per-CLI quirk.
- **Inherent or fixable:** Fixable — have the skip-marker writer emit the same source fingerprint promote recomputes (or give promote a sanctioned `--write-skip` re-record path).
- **Durable fix:** Fingerprint at marker-write time; parity test that a skip marker written before an edit is either still valid or has an explicit re-record path.
- **Test:** positive: record a sanctioned skip, edit a source file, run promote — marker re-records or validates without hand-editing; negative: an unsanctioned stale marker still fails promote.
- **Evidence:** game-goat Phase 18 skip marker + hand-injected fingerprint in the archived marker (this run's proofs); acceptance re-run then passed 209/209.
- **Related prior retros:**
  - `companies-house` retro #4457 — `aligned`. Same family: documented hand-authored marker shape vs. gate expectations. This retro adds the 4th-CLI recurrence and the still-unfixed-on-main status. Related: #4166 (marker location).

### 3. Generator emits byte-identical thin platform framework Shorts in every print (generator enhancement)
- **What happened:** tools-audit flagged 2 thin Shorts — `platform_client.go:517` "List client profiles" and `teach.go:868" "List recorded learnings" — both in DO-NOT-EDIT generated files. Both are byte-identical to the same lines in ynab and exa (verified this session), and the templates on current press main still contain them. Every polish run must accept-and-annotate them by hand.
- **Scorer correct?** Yes — tools-audit correctly flags genuinely thin Shorts; the defect is that the generator keeps emitting them.
- **Root cause:** Generator — framework templates (`platform_cli.go.tmpl`, `teach.go.tmpl`) hardcode one-line Shorts with no per-API or command-specific synthesis.
- **Cross-API check:** Yes — game-goat, ynab, exa byte-identical; templates unfixed on main; recurs on every print of every CLI with the platform framework.
- **Frequency:** every API.
- **Fallback if the Printing Press doesn't fix it:** Accept-and-annotate in the tools ledger every polish (reliable, scripted into the polish flow, but pure recurring tax).
- **Worth a Printing Press fix?** Yes — one template edit removes a per-run manual acceptance.
- **Inherent or fixable:** Fixable — synthesize a specific Short (name the profile store / learning journal per CLI) or at minimum a fuller generic description that passes the depth heuristic.
- **Durable fix:** Template-level Short for platform_client `list` and teach log list commands.
- **Test:** positive: fresh generate → tools-audit reports no thin-short findings for these two framework commands; negative: hand-authored command Shorts unaffected.
- **Evidence:** tools-audit pending findings in the phase19 polish proof; byte-identical grep across three library CLIs this session; template sources still on press main.
- **Related prior retros:**
  - `game-goat` prior-run retro (20260927-092504) — `aligned`. Same thin-shorts accepted there; recurrence confirmed across APIs and against current templates.

### 4. pii-audit does not auto-classify vendor-domain emails as api_provider_data (scorer enhancement)
- **What happened:** pii-audit flagged 2 copies of the RAWG published support address (verbatim attribution text in SKILL.md, quoted from RAWG's API terms). Both required manual accept-and-annotate as `api_provider_data` — the vendor's own published support address, not customer PII.
- **Scorer correct?** Partially. Flagging is defensible (an email is an email); the defect is that vendor-owned support addresses are a recurring, mechanically-recognizable class that the classifier should already know.
- **Root cause:** Scorer — internal pii.go on current main classifies emails with only GitHub-noreply, RFC-2606-reserved, and URL-placeholder exemptions (plus the vendor-spec-file exemption); there is no vendor-domain rule (email whose domain is the spec/API host domain or a subdomain, or the vendor's published support address in docs-derived attribution text).
- **Cross-API check:** Yes — library pii-polish ledgers record the same manual accept for nepra, ars-sicilia, flight-goat, and copper; code search finds `api_provider_data` accept annotations across 9+ library files (edgar, tenderned, pushover, scrape-creators, techtwitter among them). Direct evidence exceeds the three-API bar.
- **Frequency:** most APIs (any vendor that publishes a support contact in its docs, which the skill's attribution step quotes verbatim).
- **Fallback if the Printing Press doesn't fix it:** Manual accept per polish (reliable; already routine).
- **Worth a Printing Press fix?** Yes — the pattern is deterministic (domain == spec host / vendor domain) and currently costs a manual decision on most runs.
- **Inherent or fixable:** Fixable — add a vendor-domain email classification to the PII classifier, same accept class as the existing vendor-spec-file exemption.
- **Durable fix:** Classifier rule: email on the spec's host domain (or vendor-published support address inside verbatim attribution text) → `api_provider_data`, never pending.
- **Test:** positive: pii-audit on SKILL.md quoting the RAWG terms attribution → 0 pending; negative: a customer's email address still flags as PII.
- **Evidence:** game-goat phase19 PII ledger (2 accepts, `--strict` exit 0 after acceptance); 4 named library ledgers + 9+ files with the same accept class.
- **Related prior retros:**
  - `game-goat` prior-run retro (20260927-092504) — `extends`. Same accepts recorded; this retro adds the cross-library count and the classifier-rule shape.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Bare bool happy-args flag emitted as `--flag true`, false-failing NoArgs commands | scorer | documented-grammar users | high (write `=`-form) | small | keyed-matrix regression test |
| 2 | Skip markers lack the fingerprint promote recomputes | scorer | most runs w/ sanctioned skips | high (hand-inject) | small | marker/promote parity test |
| 3 | Byte-identical thin platform framework Shorts | generator | every API | high (accept-and-annotate) | small | fresh-generate tools-audit check |
| 4 | Vendor-domain emails not auto-classified | scorer | most APIs | high (manual accept) | small | pii-audit fixture test |

## Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| S1 | help "missing Examples" + live scorer synthesizing bare invocations that exit 2 on required positionals (20/173 keyed-matrix failures on id-required RAWG spec mirrors, blocks the publish marker) | Step B: only RAWG proven example-id-less among the API groups (xai, fred passed); single-API evidence. Revisit if a second id-required API reproduces. |
| S2 | polish `--research-dir` Phase-3 residual | Fixed upstream (verified: skill main passes RESEARCH_ARGS in the Phase-1 scorecard); residual is one human-output line + one ambiguous doc sentence — below bar. |
| S3 | Staged-binary staleness in shipcheck loop-1 | Carried skip: single-CLI ordering evidence; press auto-rebuilds for the scorecard leg. |
| S4 | Mid-run regeneration friction (scope cut required clean generate to /tmp + hand-spliced README/SKILL.md) | Step B: single-CLI; reprint/amend machinery covers post-publish regeneration. |

## Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| err*.txt probe leftovers in CLI tree | UAT ambiguity-probe output files landed in the source tree | iteration-noise (cleaned CLI-side) |
| `--select "results.rating"` no-match warning | Ratings card puts selected fields in a different JSON path | printed-CLI (output shape) |
| research.json novel_features empty name/rationale | Empty metadata fields on a JSON blob with no proven consumer | unproven-one-off |
| Acceptance-marker staleness after ~30 UAT edits | Fingerprint guard fired and a clean re-run resolved it — the guard worked | unproven-one-off (guard behaved as designed) |
| herdr pane bash syntax errors on parens | Harness noise in the operator's terminal multiplexer | iteration-noise (not press) |
| Flagship 401s without live key | Environmental — sanctioned skip flow recorded it cleanly | API-quirk (credential absence) |

## Work Units

### WU-1: Emit documented boolean happy-args flags as `--flag=true` (from F1)
- **Stable ID:** WU-1
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** A bare `--flag` token in `pp:happy-args` probes as a single `--flag=true` argv token, matching the documented grammar, so NoArgs commands stop false-failing.
- **Target:** internal/pipeline happy-args parsing (runtime_commands.go parseHappyArgsAnnotation) and live-dogfood flag overlay (live_dogfood.go overlayLiveDogfoodFlags).
- **Acceptance criteria:**
  - positive test: NoArgs command annotated `pp:happy-args: "--json"` passes its keyed live-matrix happy_path and json_fidelity legs.
  - negative test: `--flag=value` and `--flag=-12.3` forms emit exactly as before; a hand-authored stale marker still fails promote.
- **Scope boundary:** Does not touch the annotation grammar docs, positional-token handling, or Example synthesis (S1 stays separate).
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Skip-marker/promote fingerprint parity — new evidence on #4457 (from F2)
- **Stable ID:** WU-2
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** Fold the 4th-CLI recurrence and unfixed-on-main status into the existing #4457 evidence trail (comment, not duplicate issue).
- **Target:** same as #4457 — skip-marker writer / promote fingerprint parity.
- **Acceptance criteria:**
  - positive test: comment posted on #4457 citing game-goat evidence and the still-current code path.
  - negative test: no new duplicate issue filed.
- **Scope boundary:** No code change in this retro; the fix itself stays on #4457.
- **Dependencies:** None.
- **Complexity:** small

### WU-3: Synthesize non-thin Shorts for platform framework commands (from F3)
- **Stable ID:** WU-3
- **Priority:** P2
- **Type:** enhancement
- **Component:** generator
- **Goal:** Fresh-generated CLIs pass tools-audit without manual accept-and-annotate for the platform_client `list` and teach-log `list` Shorts.
- **Target:** internal/generator/templates (platform_cli.go.tmpl, teach.go.tmpl).
- **Acceptance criteria:**
  - positive test: fresh generate → tools-audit reports no thin-short findings for those framework commands.
  - negative test: hand-authored command Shorts unaffected; no other template output changes.
- **Scope boundary:** Does not cover per-API profile naming beyond the two known framework commands.
- **Dependencies:** None.
- **Complexity:** small

### WU-4: Auto-classify vendor-domain emails as api_provider_data (from F4)
- **Stable ID:** WU-4
- **Priority:** P2
- **Type:** enhancement
- **Component:** scorer
- **Goal:** pii-audit stops pending on emails whose domain is the spec/API vendor's own domain or its published support address in verbatim attribution text.
- **Target:** internal pii classifier (pii.go) — add vendor-domain rule alongside the existing GitHub-noreply / RFC-reserved / URL-placeholder exemptions.
- **Acceptance criteria:**
  - positive test: pii-audit on a SKILL.md quoting the RAWG terms attribution reports 0 pending.
  - negative test: a customer's email address in output still flags as PII.
- **Scope boundary:** Does not loosen real-person PII detection; only vendor-owned domains and published support addresses.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns
- Hand-authored `pp:happy-args` following the skill's documented bare-bool grammar deterministically false-fails the live matrix — the documented contract and the probe's emission disagree (F1).
- The keyed live matrix is the only leg that exercises hand-authored annotations, so a harness-side compile bug presents as a CLI defect and burns diagnosis time.
- Publish markers gate on help-quality dims while generated leaves lack Example synthesis for id-required endpoints — 20 matrix failures with no CLI-side fix short of spec edits (S1).
- Mid-run regeneration is effectively forbidden, forcing scope cuts to be executed as clean generates into /tmp plus hand-spliced README/SKILL.md (S4).

## What the Printing Press Got Right
- The fingerprint guard caught a stale acceptance marker after ~30 UAT hand-edits and forced a clean, fully-passing re-run instead of shipping a stale proof — the guard fired correctly; only the marker-writer parity lags (F2).
- The Phase-18 sanctioned-skip flow recorded environmental 401s cleanly, and next-steps correctly instructed re-running the live matrix once a key was provided — which then passed 209/209.
- The prior retro's polish `--research-dir` fix is verified live on main: the Phase-1 scorecard now passes RESEARCH_ARGS with a comment describing exactly that finding — retro feedback loop demonstrably closing.
- All polish gates (verify 100%, gosec 0 hand-authored, tools-audit 0 pending after acceptance, pii-audit strict 0 pending) reached green with clear accept semantics.
- The hollow-coverage publish gate pushed real exit-0 happy paths onto the local-store novel commands instead of letting declared-skip hollow features through.
- Ambiguity dogfood caught a genuine zelda/DOOM title-ambiguity in UAT and drove a CLI-side fix before ship.
