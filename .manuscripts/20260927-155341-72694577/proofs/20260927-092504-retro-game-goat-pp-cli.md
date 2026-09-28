# Printing Press Retro: RAWG (game-goat-pp-cli)

## Session Stats
- API: RAWG (api.rawg.io)
- Spec source: public-library RAWG OpenAPI mirror (browser-sniff gate passed; live-probed endpoint cuts)
- Scorecard: 90/100 (A) [uncapped-without-research variant reads 91]
- Verify pass rate: 100% (80/80)
- Fix loops: 3 (build, shipcheck, acceptance/polish)
- Manual code edits: ~14 sanctioned fix-loop edits across internal/cli (Shorts widening, id-example synthesis, dead-helper removal)
- Features built from scratch: 9 novel commands (tonight, trending, versus, studio, backlog, queue, finishline, moods, analytics) + 26 absorbed feature rows mapped to generated endpoints
- Live matrix (keyed, post-run): 219/245 evaluated pass; 26 failures all press-side (see F2/S1); 199 matrix-designed skips

## Findings

### 1. Skip markers omit `source_fingerprint`; promote gate requires it (bug)
- **What happened:** Dogfood `--live` with no credential writes a skip marker that has no `source_fingerprint` field. The Phase 20 promote gate demands the field and hard-fails on its absence, so the sanctioned auth-skip path (skill Phase 18 explicitly permits hand-writing the marker) deterministically blocks promote. game-goat's promote failed exactly this way.
- **Scorer correct?** Partially. The gate is right to want marker integrity, but the marker writer never emits the field — writer/gate schema drift, not a CLI fault.
- **Root cause:** `scorer` — dogfood marker emission vs promote gate validation are out of sync. Skill phase docs tell the agent to hand-write skip markers without a fingerprint recipe.
- **Cross-API check:** Yes — every published marker sampled in the public library (render, tesla, whoop) lacks `source_fingerprint`; game-goat is the fourth confirmed CLI. Any no-credential run on a key-required API hits this.
- **Frequency:** most (every key-required API printed without credentials at Phase 18 — the default)
- **Fallback if the Printing Press doesn't fix it:** Agent hand-computes a source-tree hash and injects it into the marker JSON. Fragile: recipe lives nowhere, each agent invents its own format.
- **Worth a Printing Press fix?** Yes — this is a silent publish blocker on the sanctioned path.
- **Inherent or fixable:** Fixable.
- **Durable fix:** dogfood `--live` emits `source_fingerprint` (source-tree hash) when writing skip markers, mirroring `--write-acceptance`; alternatively promote accepts skip-kind markers by computing the current tree hash itself.
- **Test:** positive — run dogfood `--live` keyless with `--write-acceptance`, confirm marker carries a fingerprint and promote passes; negative — corrupt the fingerprint, promote must fail.
- **Evidence:** This session's promote failure + public-library skip markers for render/tesla/whoop (all missing the field).
- **Related prior retros:** None known.

### 2. Live-matrix bool-flag probe `--json true` false-fails NoArgs commands (bug)
- **What happened:** happy_path/json_fidelity synthesis appends `true` after bool flags (`--json true`, `--dry-run true`). Cobra bool flags do not consume the next token, so `true` leaks into positionals; NoArgs-group commands (backlog audit, finishline, moods list) correctly exit 2 — and the matrix scores the CLI as failing. 6 of game-goat's 26 failures are this shape. Commands with a positional slot absorb the stray token and "pass" by accident.
- **Scorer correct?** No. The CLI is right in all 6; the matrix's arg synthesis is wrong.
- **Root cause:** `scorer` (dogfood matrix arg builder).
- **Cross-API check:** Yes — any generated CLI with NoArgs commands in the matrix. monarch-money and suppco publish NoArgs commands (verified in the public library); game-goat is the third.
- **Frequency:** subclass: cobra-NoArgs CLIs
- **Fallback:** Agent decodes each failure by hand and documents it (done in this session). Reliable but slow and lossy — the marker still reads fail.
- **Worth a fix?** Yes — mis-scores working CLIs on a recurring shape.
- **Inherent or fixable:** Fixable.
- **Durable fix:** probe bool flags in bare form (`--json`) or equals form (`--json=true`), never space-separated.
- **Test:** positive — `backlog audit --json` (no stray token) passes the matrix; negative — `--json true` on a NoArgs command must not be scored as a CLI failure.
- **Evidence:** This session's keyed matrix JSON (args arrays show the stray `true`; exit 2 usage errors on NoArgs commands).
- **Related prior retros:** None known.

### 3. Generator emits two known-thin Shorts in every CLI (enhancement)
- **What happened:** tools-audit flags the same two generated command Shorts as thin on every audit — platform_client "List client profiles" and teach "List recorded learnings" — and every polish run re-accepts them with pre-decision rationale. exa, ynab, and game-goat (public library + this session) carry byte-identical findings for the same two leaves.
- **Scorer correct?** Yes — the leaves genuinely say almost nothing; the audit is right, the template is the defect.
- **Root cause:** `generator` (platform_client/teach leaf templates in internal/generator/).
- **Cross-API check:** Yes — universal: the leaves are emitted for every print; three CLIs confirmed identical.
- **Frequency:** every API
- **Fallback:** pre-decision accept each polish (current practice). Reliable but recurring manual noise on every run.
- **Worth a fix?** Yes — the press's own gate fires on the press's own template, forever.
- **Inherent or fixable:** Fixable.
- **Durable fix:** generator emits verb-led, object-specific Shorts for those two leaves (or drops the leaves if they carry no value).
- **Test:** positive — fresh print's tools-audit reports 0 thin findings for those leaves; negative — no other Shorts regress.
- **Evidence:** tools-polish ledgers in exa/ynab (public library) + this session's ledger: same two findings, same wording.
- **Related prior retros:** None known.

### 4. PII audit flags vendor-domain contact emails every run (enhancement)
- **What happened:** support/contact emails published by the API vendor on its own domain (attribution/ToS/README text the generator copies) are flagged as PII `email` and must be manually accepted as `api_provider_data` each polish. Five published CLIs (nepra, ars-sicilia, flight-goat, copper — public-library ledgers — plus game-goat) carry accepted vendor-domain email findings; game-goat had two (RAWG contact addresses in attribution text).
- **Scorer correct?** Partially. Flagging is defensible (email shape), but the vendor's own published contact address is definitionally not customer PII, and the same manual accept recurs every run.
- **Root cause:** `scorer` (pii-audit) — no domain-based classification.
- **Cross-API check:** Yes — 5 CLIs confirmed with accepted vendor-domain email findings; recurs wherever attribution quotes vendor contact info.
- **Frequency:** most
- **Fallback:** manual accept with rationale (current practice; works, repeats every run).
- **Worth a fix?** Yes — a deterministic five-CLI recurrence of the same manual decision.
- **Inherent or fixable:** Fixable.
- **Durable fix:** pii-audit auto-classifies emails whose domain matches the API's registered vendor domain / spec contact metadata as `api_provider_data`. Guard: only domains present in spec metadata or the API host — customer emails stay flagged.
- **Test:** positive — RAWG attribution email categorizes as api_provider_data without manual accept; negative — a customer email from live data still hard-flags.
- **Evidence:** five public-library pii-polish ledgers + this session's two accepts.
- **Related prior retros:** None known.

### 5. Polish skill omits `--research-dir` from scorecard invocations (enhancement)
- **What happened:** the polish skill's Phase 1 baseline and Phase 3 before/after scorecard commands pass `RESEARCH_ARGS` to dogfood but not scorecard. Scorecard reads the run's research directory via `--research-dir`; without it the live-matrix qualifier reads "unavailable" while the data sits in the run dir. This session's before/after both degraded this way; with the flag, the qualifier resolves to real pass/fail counts.
- **Scorer correct?** Yes — scorecard behaved as documented; the skill's command block is the defect.
- **Root cause:** `skill` (printing-press-polish phase-doc command blocks).
- **Cross-API check:** Structural — applies to every polish run on any API with a research dir; verified against the skill's own Phase 1/3 command blocks and this session's before/after output.
- **Frequency:** every API
- **Fallback:** agent adds the flag by hand (as done here). Reliable once noticed, but the skill text keeps teaching the degraded form.
- **Worth a fix?** Yes — evidence-quality regression in the skill's own gate comparison.
- **Inherent or fixable:** Fixable.
- **Durable fix:** add the research-dir passthrough to both scorecard invocations in the polish skill phases (two one-line edits), or scorecard auto-discovers research.json adjacent to the CLI dir.
- **Test:** positive — polish Phase 1 output shows a real live-matrix qualifier; negative — without a research dir present, scorecard still degrades gracefully.
- **Evidence:** this session's /tmp/polish-scorecard-after.json (status available but qualifier unavailable without the flag) vs the run-dir research.json that holds the matrix.
- **Related prior retros:** None known.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Skip-marker `source_fingerprint` parity | scorer | most | fragile hand-hash | small | skip-kind markers only; corrupt fingerprint must still fail |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | Bool-flag probe `--json true` false-fails NoArgs | scorer | cobra-NoArgs CLIs | manual decode | small | bare or `=` probe only |
| 3 | Known-thin platform_client/teach Shorts | generator | every API | pre-decision accept | small | no other Shorts regress |
| 4 | Vendor-domain email auto-classification | scorer | most | manual accept | medium | spec-metadata/host domains only |
| 5 | Scorecard `--research-dir` passthrough | skill | every API | hand-added flag | small | graceful when research absent |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| S1 | help-kind failures: "missing Examples section" for id-required endpoints | Step B: only RAWG's spec proven to lack example ids (xai/fred matrices passed); single-spec evidence. Real gate impact but unproven to generalize. |
| S2 | Depth-heuristic leaf-name collision ("moods list" mismatch) | Step B: game-goat only; exa's depth mismatch is a different shape (registration grouping), not the same false positive. |
| S3 | Dead helpers in generated template | Step B: 2 of 4 sampled CLIs carry them; below the 3-API bar. |
| S4 | Staged-binary staleness in shipcheck validate-narrative leg | Press already auto-rebuilds for the scorecard leg (skill phase doc documents it); single-CLI evidence of the ordering gap. |
| S5 | Typed secondary-client scaffolding (Steam enrichment source) | Step B: 2 APIs with evidence (game-goat steam, flight-goat flightaware). |
| S6 | mcp_surface_strategy spec fields unused by generator | Spec-edit territory per the polish skill's own guidance; not a machine defect without 3-API evidence. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|------------|
| `--skip-if-no-credential` flag typo | dogfood rejected unknown flag | iteration-noise |
| phase-receipt `--notes`/`--status` misuse | wrong args on enter | iteration-noise |
| fabric_exec type/argument errors, ask_user_question timeouts | harness friction mid-run | iteration-noise |
| RAWG `--rating` flag choice | per-CLI semantic decision | printed-CLI |
| verify pass-rate confusion (UI-anchor rows) | scorer correctly excludes disabled anchors | iteration-noise |
| publish blocked awaiting passing marker | downstream of F1/F2, not separate | unproven-one-off |

## Work Units

### WU-1: Emit `source_fingerprint` in dogfood skip markers (from F1)
- **Stable ID:** WU-1
- **Priority:** P1
- **Type:** bug
- **Component:** scorer
- **Goal:** Dogfood-written skip markers satisfy the promote gate's fingerprint requirement without agent hand-hashing.
- **Target:** scorer — dogfood marker emission (skip path), verify against promote gate validation.
- **Acceptance criteria:**
  - positive test: keyless `dogfood --live --write-acceptance` produces a marker that passes promote
  - negative test: tampered/corrupt fingerprint still fails promote
- **Scope boundary:** Does not change acceptance-marker semantics or gate policy for pass markers.
- **Dependencies:** None
- **Complexity:** small

### WU-2: Fix bool-flag probe form in live-matrix arg synthesis (from F2)
- **Stable ID:** WU-2
- **Priority:** P2
- **Type:** bug
- **Component:** scorer
- **Goal:** NoArgs commands stop failing the matrix when a bool flag is probed.
- **Target:** scorer — dogfood matrix arg builder (happy_path/json_fidelity probe synthesis).
- **Acceptance criteria:**
  - positive test: `backlog audit --json` (bare probe) passes json_fidelity on a NoArgs command
  - negative test: `--json true` never appears in synthesized args; stray positional tokens never counted as CLI failure
- **Scope boundary:** No change to command registration or cobra behavior — probe shape only.
- **Dependencies:** None
- **Complexity:** small

### WU-3: Fix the two known-thin generator Shorts (from F3)
- **Stable ID:** WU-3
- **Priority:** P2
- **Type:** enhancement
- **Component:** generator
- **Goal:** A fresh print's tools-audit no longer flags platform_client/teach leaves as thin.
- **Target:** generator — platform_client/teach leaf Short templates.
- **Acceptance criteria:**
  - positive test: regenerated CLI passes tools-audit with 0 thin findings on those leaves
  - negative test: no other Shorts regress (audit comparison before/after)
- **Scope boundary:** Those two leaves only; no audit allowlisting.
- **Dependencies:** None
- **Complexity:** small

### WU-4: Auto-classify vendor-domain emails in pii-audit (from F4)
- **Stable ID:** WU-4
- **Priority:** P2
- **Type:** enhancement
- **Component:** scorer
- **Goal:** Vendor-published contact emails categorize as api_provider_data without a manual accept.
- **Target:** scorer — pii-audit classification (domain match against spec contact metadata / API host).
- **Acceptance criteria:**
  - positive test: RAWG attribution email auto-categorized, no pending finding
  - negative test: a customer email from captured live data still flags and requires review
- **Scope boundary:** Classification only — detector scope and leak rules unchanged.
- **Dependencies:** None
- **Complexity:** medium

### WU-5: Add `--research-dir` passthrough to polish skill scorecard calls (from F5)
- **Stable ID:** WU-5
- **Priority:** P2
- **Type:** enhancement
- **Component:** skill
- **Goal:** Polish before/after scorecard output shows the real live-matrix qualifier every run.
- **Target:** skill — printing-press-polish Phase 1 and Phase 3 scorecard command blocks.
- **Acceptance criteria:**
  - positive test: polish Phase 1 on a run with research.json prints a resolved qualifier
  - negative test: absent research dir, scorecard still degrades gracefully (no crash)
- **Scope boundary:** Skill-doc edits only; no scorer changes.
- **Dependencies:** None
- **Complexity:** small

## Anti-patterns
- Ran the full live matrix keyless and wrote a skip marker before decoding what the marker would later gate — decode failure classes before writing gates.
- Flag typos discovered by rejection instead of checking command help first (`--skip-if-no-credential`).
- Phase-receipt ordering errors (entering phases out of sequence) — read the receipt contract before invoking.
- Let 26 matrix failures sit un-decoded overnight; decoding took minutes once attempted.

## What the Printing Press Got Right
- The absorb-manifest research loop (movie-goat → game-goat, 26 rows mapped with live-probe-verified cuts) — the strongest part of the pipeline.
- Doctor/auth UX: keyless failures print remediation (export var, apidocs link, doctor) instead of bare 401s.
- Exit-code taxonomy (2 usage, 4 auth, 5 not-found) made the matrix failures decodable by hand in minutes.
- Scorecard `--live-check` auto-rebuilds stale staged binaries and reports the refresh — caught a real staleness issue mid-shipcheck.
- Scrub discipline (Layer 0) kept vendor emails and any secret shapes out of this retro doc automatically.
