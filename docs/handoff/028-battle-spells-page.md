# Handoff 028: combat spells in the web battle screen

Date: 2026-11-08
Previous: [027](027-start-here.md) (still the start-here doc; this round did its step 1).

## State

- This round: branch `battle-spells-page`, stacked on `handoff-027` (PR #34). **Don't merge without
  approval.** Once #34 is squash-merged, merge `master` into this branch and retarget the PR with
  `gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master`.
- No asset re-export needed.
- On this machine port 8080 is taken by another service: run `go run ./util/serve-web -addr localhost:8099`
  and `python3 frontend/web/smoke_test.py http://localhost:8099`.

## What changed

1. **Two-step casting** (`core/game/battle.go`): a `battle-cast` without `target` for a spell that
   needs one no longer fails. The model's question (select unit / tile / one of a list) stays
   pending in the session. New commands: `battle-target` (`unit` or `x`,`y`) answers it (a refused
   target leaves it waiting) and `battle-cancel` drops it (nothing paid). Any other battle command
   drops it too. A `battle-cast` with `target` still works in one step.
2. **Battle view**: `magic` (the wizard's `mana-pool`, `mana`, `range`, `available`, `casted`,
   `spells`), `unit-magic` (the selected unit's spells and item charges, with `caster` and
   `casting-skill`), `casting` (the waiting spell, `kind` and the targets the model accepts:
   `units`, `tiles` or `choices` with name/asset/banner). Each spell has `castable`, using the same
   checks `battle-cast` makes. The target lists come from the model's own `CanTarget`, so the page
   never guesses.
3. **Fix**: picking from Raise/Animate Dead's choices matched ids through `GetUnitById`, which
   doesn't find killed units, so it always failed. It now matches within the offered units.
4. **Page** (`frontend/web/battle.js`): Spells button (S), with mana and skill in the bar. The list
   overlays the field, grouped wizard / unit, coloured by realm, with uncastable spells disabled.
   Picking a spell that needs a target enters target mode: accepted units/tiles pulse purple, a
   click casts, Esc cancels. Choices show as buttons with the unit's sprite. Errors and "X cast"
   show in the hint line, since the log is hidden behind the battle screen. Field clicks go
   through the same path, so refused moves show too.
5. **Tests**: `TestSessionBattleCast` covers the waiting spell, a refused target, cancel and the
   one-step cast. `TestSessionBattleCastChoice` covers Animate Dead offering two killed units and
   reviving the picked one. The smoke test banks mana (ends turns until 30) before marching, opens
   the list with S, casts the first castable wizard spell, cancels once with Esc if it waits for a
   target, then picks the target on the canvas (or the first choice).
6. `docs/frontend-protocol.md`: the new commands and view fields.

## Not yet / notes

- Variable-cost spells are cast at their base cost (as the core did before); the original lets
  you add power.
- Fizzled spells (`CheckDispel`) report only through the model's message event, which the session
  consumes. The page says "X cast" then. A combat event log (027 step 3) would carry it.
- Invisible enemies are listed as targets if the spell's `CanTarget` allows them. The Ebitengine
  screen behaves the same.

## Next

027's steps 2 onward: AI-started battles against the human through the host coroutine, battle
polish (walls, animations, a log), web frontend gaps.
