# Handoff 005 — combat in core/; the game model needs an event-API decision

Date: 2026-11-05
Previous: [004](004-maplib-city-player-ai-in-core.md)

## What changed

Open, stacked (merge in order; see 003 for the no-force-push procedure):

| PR | Branch | Base | Content |
|---|---|---|---|
| #7 | `refactor-scripts` | `master` | refactor helpers |
| #8 | `maplib-split` | `refactor-scripts` | `mapview`; maplib, city, setup, player, ai → core |
| #9 | `combat-split` | `maplib-split` | combat model → `core/combat`, screens → `game/magic/combatview` |

`core/` now: `ai`, `artifact`, `building`, `city`, `combat`, `data`, `hero`, `maplib`, `neural`,
`pathfinding`, `player`, `setup`, `spellbook`, `terrain`, `units`. Guard clean, build/vet/tests pass.

### The combat split (#9), commit by commit
1. `TileTop.Drawer` (a drawing closure in the model, for the chaos-node river) → `TileTop.Special`
   enum; the screen draws it in `drawTileTopSpecial` with the same code.
2. `Projectile`/`MagicVortex` hold `combat.Animation` (`interface { Next() bool }`) instead of
   `*util.Animation`. The screen still creates `*util.Animation`; frames via `animationFrame()`.
3. `CombatState`, `CombatEvent*`, `Targeting`, `computeTileDistance`, `abs` moved from the screen
   file into model files (the model uses them).
4. Screens → `combatview` (model imported as `combatlib`). Exported the 13 model internals the
   screen used (`MeleeAttack`, `RangeAttack`, `DoUnitCast`, `ComputeTileDistance`, ...).
   `unitview.CombatPoints` → `units.CombatPoints` (unchanged).
5. `git mv` → `core/combat`; import tidy.

### Game-logic quirks found (kept as-is, worth knowing for headless/AI work)
- **Projectile effects fire when the explosion animation ends** (`UpdateProjectiles`), so combat
  timing depends on how many frames the explosion sprite has. A headless frontend must supply an
  `Animation` with the same frame counts, or effects fire at different times.
- **The headless runner (`combat/run.go`) applies one ranged hit per `units.CombatPoints` point.**
  For more than 8 figures the grid can have fewer points than figures (10 figures → 9 points).
- `combatview` keeps its own `abs()`; `Army.units` is read via `GetUnits()`.

## Next: `game/model.go` — needs a decision on the event API

`model.go` (2k lines) has no rendering imports, but it can't move alone. From the rest of package
`game` it needs:
- `events.go` (random events; `ReadEventData` is an LBX reader → `lbxdata`),
- `road.go`: `ComputeEngineerCount`,
- `game.go`: `ChangeCityOwner` (+ `ChangeCityEnchantments` enum), `canUseArtifact`, and the
  `GameEvent` types it sends: `GameEventNotice`, `GameEventRefreshUI`, `GameEventScroll`,
  `GameEventShowRandomEvent`, `GameEventVault`.

`GameEvent` (game.go:100) is the model→UI channel, but it mixes two kinds of messages:
- 3 carry UI objects: `GameEventInvokeRoutine` (coroutine), `GameEventRunUI` (`*uilib.UIElementGroup`),
  `GameEventSelectLocationForSpell` (coroutine callback). Plus a `musiclib.Song` field somewhere.
- 39 are plain data, but many are really "open screen X" requests (`GameEventMagicView`,
  `GameEventArmyView`, ...), not game-state events.

**Proposal:** new package `core/game` (or `core/gamemodel`) with `GameModel`, the random events,
`ChangeCityOwner`, `canUseArtifact`, `ComputeEngineerCount`, and an `Event` interface plus only the
events the model emits (the five above). The UI package keeps its own events (screen requests, UI
commands) on the same channel, typed as the core interface. This starts the command/event API
(001 step 4) with the model's real outputs, instead of moving the whole mixed set.

Decisions needed:
1. Package name: `core/game` (clashes in readers' heads with `game/magic/game`) or `core/gamemodel`?
2. OK to split `GameEvent` into core events (model output) vs UI events (screen requests)?

## Other next steps
- After the model: the rest of `game.go` (8.6k lines). Same recipe, lots of small steps.
- Smoke-test the game when MoM data is available: overworld, minimap, roads, cartographer,
  map editor (from #8) and a full combat incl. chaos nodes and fireball/lightning (from #9).
