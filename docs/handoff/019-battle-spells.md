# Handoff 019: combat spells through commands; combat events applied without a screen

Date: 2026-11-06
Previous: [018](018-tactical-combat-sessions.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#22 are merged into `master`. #22 was merged with a fix: `CombatModel.FinalState()` has
  side effects once a battle ends (it re-runs the flee rolls and `FinishCombat`), and the battle
  session called it twice after a flee, so fleeing units rolled twice. `BattleSession.State()` now
  caches the outcome.
- This round: branch `battle-spells`, PR #23 against `master`. **Don't merge without explicit approval.**
- Next big item: the wasm bridge.

## What changed

1. **`battle-cast {spell, caster?, target?}`** (`core/game/battle.go`), with the battle screen's
   rules: the wizard casts once per turn, within `min(army mana pool, wizard mana / range)`, from
   the combat spells (city-defence filter); a unit (`caster` = the selected unit's id) casts its
   spells within its casting skill, or uses item charges, then is done (`DoUnitCast`). The combat
   model asks for targets with events (`CombatEventSelectUnit`, `CombatEventSelectTile`,
   `CombatSelectTargets` for Raise/Animate Dead); the session answers them from the command's
   `target` (`{unit}` or `{x, y}`), checking side and `CanTarget`. The cost is paid in the model's
   callback, so a refused target costs nothing (as in the screen).
2. **`CombatModel.ApplyEvent`** (`core/combat/events.go`): the model-side effects of combat events
   for code without a battle screen: next unit, Lightning Bolt projectiles, Wall of Fire / Darkness
   tiles, AI/auto armies picking a target from a list. **`combat.Run` now uses it** (it discarded all
   events before, so AI-vs-AI and headless battles lost those effects; inherited from upstream)
   and resolves projectiles after. The battle session uses it too.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass;
  CI guard empty; `TestHeadlessGame` with data passes; session tests 300× without failures and
  under `-race`.
- New test: the wizard casts Fire Bolt at the enemy (refused without a target and nothing paid;
  then paid, and a second wizard spell in the turn refused).
- Headless 300 years (36 battles) and `-watch` 3 min (89 years): no errors.
- Ebitengine screens unchanged; the real game uses `combat.Run` for battles between two AI players,
  so those now get the event effects too.

## Next

1. **Wasm bridge** (`GOOS=js`): a `CommandSession` per human player plus the turn loop (AI turns
   via `DoAiTurn`, as in the headless game), behind `execute(json)`, `events()`, `view()`,
   `map(plane)`, `battle()`; data from `core/gamedata`.
2. Diplomacy initiated by other wizards (proposals to the player) has no events/answers yet.
