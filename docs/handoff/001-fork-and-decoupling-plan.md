# Handoff 001 — Fork set up, decoupling plan

Date: 2026-11-05
Branch: `master` (origin: `git@github.com:bfalese/civ-of-magic.git`)

## Where we are

- Forked `kazzmir/master-of-magic` → `bfalese/civ-of-magic`. No code changes yet.
- `CLAUDE.md` added (project context, build commands, style, architecture rules).
- Decisions made so far:
  - Base: this Go fork (rejected the C# Scifilization/CivOne route).
  - Split logic from rendering: pure-Go core + swappable frontends
    (Ebitengine now, Three.js via wasm later, headless for tests/AI).
  - Drop LBX assets eventually; standard formats (PNG atlases + YAML data).
  - Visual target: MoM retro look over 3D (low-res, 32-color palette, Bayer dither,
    depth outlines, sprite units). A browser prototype exists ("Realm of Hexes" artifact on claude.ai).

## Codebase survey (measured on upstream, 2026-11-05)

All paths under `game/magic/`.

| Area | Ebitengine coupling | Notes |
|---|---|---|
| `ai`, `player`, `city`, `terrain`, `data`, `hero`, `building`, `pathfinding`, `neural` | none | ~20k lines, ready to move |
| `game/model.go` (~2k lines) | none | game state model |
| `combat/model.go` (~8.8k lines) | 4 refs | already separate from `combat-screen.go`; **template for the split** |
| `combat/remote.go`, `observer.go` | none | networking, useful pattern for command/event API |
| `game/game.go` (~8.6k lines) | 169 refs | main tangle: `Game` struct mixes state + UI (fog image, HUD, draw callbacks). Seam: `Events chan GameEvent` |
| `game/cast.go`, `surveyor.go`, `vault.go`, … | mixed | split after `game.go` |
| `units`, `maplib`, `load` | 1–2 files each | probably sprite refs → asset IDs |

Re-verify before moving anything, e.g.:
```sh
for d in game/magic/*/; do
  n=$(grep -rl 'hajimehoshi/ebiten' "$d" --include=*.go | wc -l)
  echo "$n $d"
done | sort -n
```

## Next steps (in order)

0. **Housekeeping** (small, do first)
   - Rename module `github.com/kazzmir/master-of-magic` → `github.com/bfalese/civ-of-magic`
     in `go.mod` and all imports (one search/replace), then `go build ./... && go test ./lib/... ./game/...`.
   - Optional: `git remote add upstream https://github.com/kazzmir/master-of-magic.git`.
   - Update README badges/links to the fork.
1. **Move clean packages under `core/`** (mechanical, low risk)
   - Target e.g. `core/{ai,player,city,terrain,data,hero,building,pathfinding,neural}`.
   - Use `git mv` + import rewrite only; no logic changes in the same commit.
   - Watch out: some may import LBX-related packages (`lib/lbx`, `load`, `units` sprites) — if so,
     leave them in place for now and note them; don't break the "no LBX in core" rule.
   - Add CI guard to `.github/workflows/build-linux.yml`:
     ```yaml
     - name: Core must not depend on rendering
       run: |
         if go list -deps ./core/... | grep -E 'ebiten|/lbx'; then
           echo "core/ imports rendering or LBX"; exit 1
         fi
     ```
2. **Asset IDs**: replace sprite/image refs in `units`, `maplib`, `load` with string IDs resolved by the frontend.
3. **Split `game/game.go`** into model (→ core) and screens (→ Ebitengine frontend), following `combat`'s pattern.
4. **Command/event API**, then a wasm bridge (JSON commands/events) for the Three.js retro renderer.

## Open questions

- Package name/layout for frontends: `frontend/ebiten/`, `frontend/web/`, `frontend/headless/`?
- How long to keep LBX loading working (needed to run the game until replacement assets exist)?
- Which Civ concepts come first once the split is done (tech tree, culture, diplomacy…)?
