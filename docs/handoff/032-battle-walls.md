# Handoff 032: walls on the battlefield, siege scenario

Date: 2026-11-08
Previous: [031](031-faster-smoke-test.md). Start-here doc: [027](027-start-here.md) (this finishes its step 3, except spell visuals).

## State

- PR #38 (faster smoke test, fast battle view) is merged.
- This round: branch `battle-walls`, PR against `master`. **Don't merge without approval.**
- **Re-export the assets** (new sprites): `go run ./util/export-assets -data "<MoM dir>"`. That
  gives 4930 sprites.

## What changed

1. **`combat.Tile.WallPieces`** (`core/combat/walls.go`): the pieces of a tile's city wall, wall
   of fire and wall of darkness (citywall.lbx index, offset, animated or destroyed, order), with
   the rules from the Ebitengine `DrawWall` (corners, variants by position, fire/darkness over wall
   corners). `TestWallPieces`.
2. **Ebitengine `DrawWall`** now draws those pieces (the rising animation from wallrise.lbx is
   kept), so both frontends share one set of rules. *Not checked by eye in the desktop game;* the
   rules are the old ones moved, and the web page shows them right.
3. **Battle view**: per tile `walls` (`asset` `battle-wall/<index>`, `dx`, `dy`, `animated`,
   `frame`) and `walls-in-front` (south/east walls stand in front of a unit on the tile, as the
   original sorts them ±0.1 tile). Also `fortress` + `fortress-tile` for a town with the wizard's
   fortress (cmbtcity 17), which the page didn't draw before. `TestSessionBattleWalls`.
4. **Assets**: citywall.lbx 0-63 (stone/dark/ivy walls, fire, darkness; all frames, not cropped)
   and the fortress top.
5. **Page**: walls drawn like a top in the middle of the tile plus each piece's offset, fire and
   darkness animate, sorted with units and tops; the fortress as a bottom-aligned top.
6. **`siege` scenario**: an AI wizard's capital gets city walls, a wall of fire, a wall of
   darkness and a spearman, with four human swordsmen next to it. `TestHostSiegeScenario` (10
   games). The smoke test attacks with it, checks walls, fire/darkness and the fortress are on the
   field, takes a screenshot (`8-siege`) and flees.
7. **Fix in the tests**: the map wraps east-west. The army can stand at x=0 next to a capital at
   x=width-1. Both the Go test and the smoke test's finder now measure x distance with the wrap
   (150 siege games passed after that; the battle scenario's 150 too).

## Notes

- The smoke test takes about 8.5 s now, with three page loads (first turn, battle, siege).
- The headless hang from 031: 400 isolated runs of `TestHeadlessGame` didn't hang. The one hang
  was in a combined batch right after another test had failed, so it may come from tests
  interacting (shared state between hosts in one process) rather than from the game. Keep an eye
  on CI.

## Next

027 step 4, web frontend gaps:
- first-turn research (the core asks only from turn 2);
- magic screen (power distribution, alchemy) and the spellbook for overland spells;
- armies and items (hero equipment, vault, disband, split stacks);
- rename city, city enchantments, tax rate, diplomacy;
- soft fog edges; a start screen instead of URL parameters.
Spell visuals in battles (the original's animations) are still open from step 3.
