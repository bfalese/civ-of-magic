# Handoff 003 — spellbook, artifact, units, hero in core/

Date: 2026-11-05
Previous: [002](002-core-packages-moved.md)

## What changed

- PRs #1–#3 (from 002) are **merged** into `master`.
- Open, stacked: merge in order.

| PR | Branch | Base | Content |
|---|---|---|---|
| #4 | `spellbook-split` | `master` | spell model → `core/spellbook`, screen → `game/magic/spellbookview`, readers → `lbxdata` |
| #5 | `artifact-split` | `spellbook-split` | artifact model → `core/artifact`, screens → `game/magic/artifactview`, readers → `lbxdata`; import tidy |
| #6 | `units-hero-core` | `artifact-split` | `units/graphics.go` → `unitview`; `units` → core; hero name readers → `lbxdata`; `hero` → core |

`core/` now: `data`, `pathfinding`, `neural`, `terrain`, `building`, `spellbook`, `artifact`, `units`, `hero`.
Each step verified with build, vet, `xvfb-run go test ./core/... ./lib/... ./game/...`, the
combat-simulator server module and the core guard.

Only real behavior-adjacent change so far: `artifact.MakeRandomArtifact` now takes
`(costs, compatibilities)` instead of the LBX cache; treasure generation reads the power table.

## How to merge stacked PRs (force-push is blocked in this setup)

Squash-merging a PR leaves the next branch with conflicts against `master`. Without force-push:
1. `gh pr merge N --squash`
2. on the next branch: `git merge -s ours origin/master` (safe because the branch already contains
   everything in the squashed commit; check `git diff <prev-branch> origin/master` is empty first), push.
3. retarget with `gh api -X PATCH repos/bfalese/civ-of-magic/pulls/M -f base=master`
   (`gh pr edit --base` fails with a "Projects (classic)" GraphQL error).

## Recipe that works for each package

1. Same-package moves first: pull model logic out of UI files (e.g. `ComputeSpellCost`).
2. Move the UI files to `game/magic/<pkg>view` (methods on model types can't move; check first).
   Import the model as `<pkg>lib` if code uses `<pkg>` as a variable name.
3. Move `Read*(cache)` functions to `game/magic/lbxdata`. UI must move first if it calls them
   unqualified, else you get an import cycle (`lbxdata` imports the model).
4. `git mv` to `core/`, rewrite import paths.
5. After each step: verify the moved code is identical apart from qualifiers (diff with
   qualifiers stripped).

## Next: `maplib` (needs a design decision)

`maplib` (3.4k lines; `map.go`, `node.go`, `serialize.go`) imports `camera`, `scale`, `shaders`,
`util`, Ebitengine. The coupling is structural, not just a few functions:
- Tile-extra types (`ExtraRoad`, `ExtraBonus`, `ExtraEncounter`, `ExtraOpenTower`, `ExtraMagicNode`,
  `ExtraVolcano`, `ExtraCorruption`) have `DrawLayer1`/`DrawLayer2(screen *ebiten.Image, ...)`
  methods, presumably part of the extra-tile interface.
- `Map` has `GetTileImage`, `DrawMinimap`, `DrawMinimap2`, `DrawLayer1`, `DrawLayer1_y`, `DrawLayer2`.

Proposed approach: drop `Draw*` from the model interface; add `game/magic/mapview` with
`DrawExtra(extra, screen, ...)` (type switch) and `DrawMap(m *maplib.Map, ...)` functions; callers
change from `m.DrawLayer1(...)` to `mapview.DrawLayer1(m, ...)`. Methods can't move packages, so this
is more than a `git mv`. After that, `city` (`ReadCityNames` → `lbxdata`), then `player`, `ai`
(`player` also imports `game/magic/setup`; check it).

## Notes

- CI: macOS M1 jobs sometimes fail with "job was not acquired by Runner" (GitHub capacity);
  arm64 tests had one xvfb flake ("Failed to open display :99"). Neither is a code problem.
- The automated moves used small Python scripts (move decls, requalify, fix callers). They aren't
  in the repo; ask if you want them under `scripts/refactor/`.

## Open questions

- `maplib` split approach above: OK?
- Frontend layout: `frontend/ebiten/`, `frontend/web/`, `frontend/headless/`? (The `*view`
  packages are effectively the Ebitengine frontend already.)
- How long to keep LBX loading working?
- Which Civ concepts come first after the split?
