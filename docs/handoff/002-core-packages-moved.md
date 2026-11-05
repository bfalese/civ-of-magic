# Handoff 002 — Module renamed, first packages in core/

Date: 2026-11-05
Previous: [001](001-fork-and-decoupling-plan.md)

## What changed

Three stacked PRs. Merge them in order; each is based on the one before.

| PR | Branch | Content |
|---|---|---|
| #1 | `rename-module` | CLAUDE.md + handoff 001 committed; module path → `github.com/bfalese/civ-of-magic`; README badges |
| #2 | `core-move-clean` | `data`, `pathfinding`, `neural` → `core/`; CI tests `./core/...` and fails if `core/` depends on `ebiten` or `/lbx` |
| #3 | `core-terrain-building` | New `game/magic/lbxdata` holds the LBX parsers; `terrain`, `building` → `core/`; `building/testinfo` → `test/building-info` |

All verified locally: `go build ./...`, `go vet ./...`, `xvfb-run go test ./core/... ./lib/... ./game/...`,
the `util/combat-simulator/server` module, and the core guard.

Local setup note: the machine needed `libasound2-dev xorg-dev xvfb` (now installed).

## Correction to 001

001 listed `ai`, `player`, `city`, `terrain`, `hero`, `building` as rendering-free. That only counted
*direct* imports. Transitively they all pulled in Ebitengine:
- `lib/lbx` itself imports Ebitengine, so any package parsing LBX is out of core.
- Always check with `go list -deps ./pkg | grep -E 'ebiten|/lbx'`, not grep on the package.

Pattern that worked for `terrain`/`building`: move the `Read*(cache *lbx.LbxCache)` functions
unchanged into `game/magic/lbxdata`, then the package is clean and can be `git mv`'d.

## Still blocked from core/

| Package | Blocked by |
|---|---|
| `city` | `ReadCityNames` (lbx) + imports `units`, `maplib`, `spellbook` |
| `hero` | `names.go` (lbx) + imports `units`, `artifact`, `spellbook` |
| `ai`, `player` | the above |

What ties the blockers to Ebitengine (measured 2026-11-05):

| Package | Coupling | Likely fix |
|---|---|---|
| `units` (7.3k lines) | only `graphics.go`: `GetUnitBackgroundImage` returns `*ebiten.Image`; imports `artifact`, `spellbook`, `util` | move that function to a view package; then depends on `artifact`/`spellbook` being clean |
| `spellbook` | `data.go` = spell model + LBX read (10 refs); `spell.go` = spellbook page UI (58 refs) | split: model → core, LBX read → `lbxdata`, UI → `spellbookview` |
| `artifact` | `item.go` (24 refs), `create-artifact.go` UI (74), `view.go` (3) | same split as `spellbook` |
| `maplib` | `map.go` mixes map model and rendering (39 refs); imports `camera`, `shaders`, `scale` | the biggest one; do last (combine with asset IDs, step 2 of 001) |
| `game/magic/util` | `animation.go`, `graphics.go`, `image-cache.go` use Ebitengine | everything imports `util`; split the non-graphics helpers into a clean package if a model package needs them |

## Next steps (in order)

1. Merge PRs #1 → #2 → #3 (squash). After #1 merges, retarget #2 to `master` (GitHub may do it
   automatically when the branch is deleted), then the same for #3.
2. `spellbook`: split model / LBX reader / UI. Then `artifact` the same way.
3. `units`: move `GetUnitBackgroundImage` out, then move `units` to core.
4. `hero`, then `city` (names readers → `lbxdata`); `city` still needs `maplib` decoupled.
5. `maplib` + asset IDs; then `player`, `ai`; then 001 steps 3–4 (`game.go` split, command/event API).

## Open questions (carried over from 001)

- Frontend layout: `frontend/ebiten/`, `frontend/web/`, `frontend/headless/`?
- How long to keep LBX loading working? (`lbxdata` makes it easy to swap for YAML/PNG loaders later.)
- Which Civ concepts come first after the split?
