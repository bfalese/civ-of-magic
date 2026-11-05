# Handoff 006 — the game model is in core/game

Date: 2026-11-05
Previous: [005](005-combat-in-core-game-model-next.md)

## What changed

Open, stacked (merge in order; see 003 for the no-force-push procedure):

| PR | Branch | Base | Content |
|---|---|---|---|
| #7 | `refactor-scripts` | `master` | refactor helpers |
| #8 | `maplib-split` | `refactor-scripts` | `mapview`; maplib, city, setup, player, ai → core |
| #9 | `combat-split` | `maplib-split` | combat → `core/combat`, screens → `combatview` |
| #10 | `game-model-core` | `combat-split` | game model → `core/game` |

Decisions taken (user, 2026-11-05): package name `core/game`; split `GameEvent` into core (model
output) and UI events.

### core/game
- `model.go` (`GameModel`), `events.go` (random events), `GameModel` contact methods, with
  `contact_test.go` and `earthquake_test.go`.
- `ChangeCityOwner` (+ `ChangeCityEnchantments`), `CanUseArtifact`, `ComputeEngineerCount`.
- **Events:** `GameEvent` (empty interface) and the events the model emits: `GameEventNotice`,
  `GameEventRefreshUI`, `GameEventScroll`, `GameEventShowRandomEvent`, `GameEventVault`,
  `GameEventDiplomacy`. The UI package has `type GameEvent = gamecore.GameEvent` and keeps its other
  ~36 events (screen requests like `GameEventMagicView`, UI commands like `GameEventRunUI`) on the
  same channel. **This is the start of the command/event API**: core events are what a non-Ebitengine
  frontend has to handle.
- Exported for the UI: `DistributeEquipment`, `DoAiMoveUnit`, `DoCallTheVoid`,
  `DoMoveFleeingDefender`, fields `HeroNames`, `AllSpells` (loading a save builds the model in
  `game/magic/game/serialize.go`).
- `ReadEventData` → `lbxdata`.

**Import alias:** `gamecore "github.com/bfalese/civ-of-magic/core/game"`. `gamelib` is already used
by 11 files for the UI package `game/magic/game`.

`core/` now: `ai`, `artifact`, `building`, `city`, `combat`, `data`, `game`, `hero`, `maplib`,
`neural`, `pathfinding`, `player`, `setup`, `spellbook`, `terrain`, `units`.

## What's left in game/magic/game

| File | Lines | UI refs | Notes |
|---|---|---|---|
| `game.go` | 8565 | 404 | the tangle: `Game` struct (state + UI), turn loop, input, drawing |
| `cast.go` | 2779 | 156 | spell casting: mixes targeting UI with spell effects |
| `damage.go` | 55 | 0 | candidate for core |
| `treasure.go` | 426 | 1 | candidate (treasure generation) |
| `serialize.go` | 236 | 0 | candidate if `game/magic/serialize` and `settings` are clean |
| `vault.go`, `surveyor.go`, `hero.go`, `road.go`, `mercenaries.go`, `merchant.go`, `undead.go`, `learn-spell.go`, `cast-select-wizard.go` | 130–470 | 11–67 | mostly screens; check each for logic |

## Next steps

1. Merge #7 → #10; smoke-test the game when MoM data is available (overworld, minimap, roads,
   cartographer, map editor, combat incl. chaos nodes/projectiles, loading a save, random events,
   meeting a wizard).
2. Quick wins: `damage.go`, `treasure.go`, `serialize.go` → core (check their deps first).
3. `cast.go`: split spell effects (model) from targeting/animation (UI).
4. `game.go`: pull logic into `GameModel` piece by piece; the UI keeps input/drawing. Grow the core
   event set as model code moves (anything the model needs to tell a frontend becomes a core event).
5. Then a wasm bridge: serialize core events to JSON for the Three.js renderer.

## Tools

`scripts/refactor/` got fixes this round (see its README): `/* */` blocks and single-line rune
literals in the qualifier, aliased imports in `fixcallers.py`, and notes on the remaining gotchas
(map-literal keys, multi-line `case` lists, parameters named like an exported field).
