# Handoff 040: soft fog edges

Date: 2026-11-09
Previous: [039](039-start-screen.md). Start-here doc: [034](034-start-here.md) (this finishes its
step 5).

## State

- #45 (start screen; `docs/core-review.md` treaty item sharpened) is merged. This round: branch
  `fog-edges`, PR against `master`.
- **Re-export the assets**: 5060 sprites expected (fog edges added).

## What changed

1. **Soft fog edges**: `mapback.lbx` 0-13 exported as `fog/N` (`assets.FogEdge`). `map.js`
   `drawFogEdges` follows the desktop `Overworld.DrawFog`: on an explored tile, black edge pieces
   toward unexplored neighbours (sides, two-side corners, diagonal corners); on a visible tile, the
   same toward explored-but-not-visible neighbours at the dimming strength (0.45), so the dimmed
   area fades in. The map wraps east-west; the north/south map edge counts as no fog.
2. **`fraction.NormalString`** printed whole numbers equal to the denominator as "1/1" (the tax
   dropdown showed "1/1 (20%)"); now "1". Test added; the only other user is the desktop
   surveyor's food bonus, which gets the same fix.
3. Smoke checks: the fog sprites are in the assets, and the tax dropdown reads "1 (...)".

## Notes

- `drawFogEdges` scans the whole map each redraw (a few thousand tiles); fine at current map
  sizes. Cache it per view if the map grows or redraws become frequent.

## Next

034 step 6: battle spell animations (the replay only glows targets) and a `castable` that knows
whether a spell has a valid target. Then step 7: the 3D/retro look, Ebitengine xdotool smoke
tests, small open items from 027. Also open: `docs/core-review.md` (diplomacy decisions).
