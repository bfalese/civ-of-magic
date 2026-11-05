# Handoff 007 — spell logic from cast.go is in core/game

Date: 2026-11-05
Previous: [006](006-game-model-in-core.md)

## What changed

Open, stacked (merge in order; see 003 for the no-force-push procedure):
#7 `refactor-scripts` → #8 `maplib-split` → #9 `combat-split` → #10 `game-model-core` →
#11 `game-quickwins` → #12 `cast-split-2` (this round).

All game logic in `game/magic/game/cast.go` now lives in `GameModel` methods; `cast.go` keeps
target selection, animations, dialogs, sounds and the `doCastSpell` dispatcher (routing).

| core/game file | What |
|---|---|
| `spells_map.go` | Corruption, Raise Volcano, Enchant Road, Change Terrain, Transmute, Warp Node, Floating Island |
| `spells.go` | instant fizzle (+ `FizzleReason`), summon-hero choice, global enchantment effects, Drain Power, Cruel Unminding, Spell Blast, unit summoning, Disenchant Area, Spell Ward choices, Subversion, Spell Binding, Disjunction, Resurrection |
| `spells_units.go` | Death Wish, Great Unsummoning, Black Wind, Stasis, Ice Storm, Fire Storm |
| `spells_city.go` | city enchantment/building added, Consecration, Summoning Circle / Fortress moved |
| `planes.go` | `PlaneShift`, `IsGlobalEnchantmentActive`, Lycanthropy, Spell of Return |
| `stack.go` | `ResolveStackAt`, `RelocateUnit`, `GetPlayerByBanner` |
| `targeting.go` | `LocationType` + `CheckSpellTarget` (the per-target-type rules from the selection screen) |

Each has headless tests (`*_test.go` next to it); they run without xvfb.

**Pattern used everywhere:** the UI keeps the animation/dialog and calls the model at the same
moment the effect used to happen (often mid-animation, at a given frame), so behavior and timing are
unchanged. Where the UI needs a message, the model returns the data (amount drained, refusal reason)
and the UI formats the same text as before. Game-level wrappers that other UI code calls
(`Game.PlaneShift`, `Game.IsGlobalEnchantmentActive`, `Game.GetPlayerByBanner`) now delegate.

Small semantic note: `ApplyDisenchantArea` uses `model.AllSpells` instead of re-reading spells from
the LBX cache (`Game.AllSpells()`); same data, both model constructors set it.

Inherited quirk kept: `ApplyCorruption`'s row check is always true (`||` instead of `&&`), marked FIXME.

## What's left

`game/magic/game` is the only package that still mixes logic and UI:
- `game.go` (~8.4k lines): turn loop, AI moves, combat setup, movement, city updates, input, drawing.
  Same approach: find state changes inside `Game` methods, move them to `GameModel`, keep UI flow.
  Good first targets: methods with few UI refs (survey them like 005/006 did for `cast.go`).
- `vault`, `surveyor`, `hero`, `road`, `mercenaries`, `merchant`, `undead`, `learn-spell`,
  `cast-select-wizard`: mostly screens; check each for embedded logic.
- Possibly logic in `cityview/city-screen.go`.

## Smoke test needed

Not launched yet (no MoM data on the dev machine). When data is available, play through: overworld
map/minimap, roads, cartographer, map editor, a combat (chaos node, fireball), loading a save,
random events, meeting a wizard, and casting a range of overworld spells (map spells, summons,
curses, Spell Ward, Disenchant, Plane Shift, Spell Binding/Disjunction, Resurrection).
