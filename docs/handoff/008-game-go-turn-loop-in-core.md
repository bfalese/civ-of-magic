# Handoff 008 — game.go: rules and the turn loop are in core/game

Date: 2026-11-05
Previous: [007](007-cast-logic-in-core.md)

## What changed

PRs #7–#12 are merged into `master`. This round: PR #13 (`game-go-split-1`, base `master`).

`game/magic/game/game.go`: 8.5k → 6.8k lines.

| core/game file | What moved |
|---|---|
| `world.go` | 37 rule functions: city placement (`FindValidCityLocation*`, `MakeCityValidArea`, `CityValidArea`), end-of-turn global effects (Meteor Storm, Armageddon, Great Wasting, Chaos Rift, Nature's Wrath, volcanoes), enchantment upkeep, disbanding, melding, road building, wizard power, combat landscape, bonuses, owner lookups, `UnitBuildPowers`, `ChooseUniqueWizard`; random offers (mercenaries, merchant, heroes), Chancellor, hero experience, fizzle notice |
| `turn.go` | `StartPlayerTurn(player, castSpell)`, `EndOfTurn(randomEvents, logSummary)`, `DoNextTurn(randomEvents, logSummary, startPlayerTurn)` |
| `gameevents.go` | more model-output events: `HireMercenaries`, `Merchant`, `HireHero`, `HeroLevelUp`, `LearnedSpell`, `ResearchSpell`, `NewBuilding` |

**Frontend hooks** (functions the UI passes in, called at the same point as before):
- `StartPlayerTurn(player, castSpell)`: `castSpell` runs when a spell finishes casting (the UI
  passes `Game.doCastSpell`, which starts targeting/animations).
- `DoNextTurn(..., startPlayerTurn)`: the UI passes `Game.StartPlayerTurn`, which calls the model
  and then does Awareness, wizard discovery, next-unit selection and the refresh.
- UI preferences become parameters (`randomEvents` from settings, `logSummary` = watch mode).

`Game` keeps one-line delegating wrappers for exported methods (32 of them), so callers and
interfaces didn't change. They can be inlined later.

**Headless tests:** `core/game/turn_test.go` runs a player's turn and full rounds of the turn loop
without the UI. Players need `GlobalEnchantmentsProvider = model` (the real game does this in
`AddPlayer`).

`Game.Events` and `GameModel.Events` are the same channel (the model is created with it), so moved
code sends on `model.Events`.

Kept in the UI on purpose: `ActiveConjunctionName` (returns a HUD color), `StartingCityEvent`
(`GameEventCityName` has a screen position).

## Tools

New `scripts/refactor/promote-methods.py`: moves `Game` methods/functions onto `GameModel`, exports
them, rewrites `game.Model.`/`game.Events`/`game.X(` inside, leaves wrappers, updates UI callers.
Verify with a normalized diff (receiver/qualifier/export renames stripped), as done each step here.

## Next (game.go)

What's left is mostly screens (HUD 891 lines, watch UI, advisors, drawing, input). Logic still sits
inside these flows:
1. `doCombat` (396 ln): combat setup and applying the result (casualties, experience, city capture)
   to the overworld; screens/music stay.
2. `doMoveSelectedUnit` (203 ln): movement rules, encounters, attacking.
3. `doAiUpdate` (176 ln): driving the AI's actions; needs a model-side AI turn so AI can run headless.
4. `doEncounter`, `DoBuildAction`, `defeatCity`, `ApplyTreasure`, `createTreasure`.
Then the nine smaller UI files and `cityview/city-screen.go`.

## Smoke test

Still not launched (no MoM data). Besides 007's list: play several turns (income, research
completing, a spell finishing casting, city growth/new buildings, hero for hire / merchant offers),
with random events on and off, and end-of-turn global spells (Armageddon, Meteor Storm).
