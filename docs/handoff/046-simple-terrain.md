# Handoff 046: the simple-colour 3D terrain

Date: 2026-11-12
Previous: [045](045-low-poly-map-models.md). Start-here doc: [045](045-start-here.md).

A second look for the 3D map, next to the textured one (the original's tile art over heights):
plain colours per terrain with real shapes. Compare with `?terrain=simple` or the V key (T is the
plane-shift order). `?scenario=reveal` explores the whole map on both planes, to look at it.

## What's there

- `frontend/web/terrain-simple.js` (`SimpleGround`):
  - one patch of vertices per tile, finer where there's shape (water 2, flat land 4, hills 8,
    mountains and rivers 16 per edge); a finer patch follows a coarser neighbour's vertices along
    their shared edge (no cracks). Patches are cached by their 5x5 neighbourhood (terrain + sites).
  - heights and colours blend between tiles over a band at their edges (`blendBand`).
  - water: one plane at height 0, water tiles' ground sinks below it; darker away from the coast
    (a texture of each water tile's distance to explored land, `deepFrom`).
  - rivers: channels carved from a winding course through each river tile to its river/water
    neighbours.
  - hills: mound + bumps; mountains: a cone, a side peak and ridged noise, snow on top (volcano:
    lava, chaos node: red rock). Neighbouring mountains (and hills) are joined by ridges (`range`).
  - sites: lairs, cities and minerals get a flat ledge on hills and mountains (`siteRadius`).
  - shades: several per terrain in patches (`shaded`), each tile a little lighter or darker
    (`tileTint`).
  - one shader for ground and water: flat (facet) shading from screen-space derivatives, the 2D
    map's canvas (roads, fog, dimming, grid, paths) laid over it. `MapRenderer.drawsTerrain` off
    paints only those, on a transparent canvas.
- `frontend/web/vegetation.js` (`Vegetation`): procedural low-poly trees and shrubs (Three.js
  primitives merged; canopies tinted per plant), one `InstancedMesh` per kind per map copy. Density
  per terrain in `plantings`; placements fixed by hash and cached per tile; kept off water, steep
  slopes and billboards' feet; dimmed on explored-not-visible land.
- `map3d.js`: `setTerrainLook` / `toggleTerrainLook`, `sites()`, minerals as billboards in the
  simple look (`standsUp`), node tiles without their tile-art billboard there.
- `core/game/scenario.go`: `reveal`.
- Smoke test: `simple_terrain` (simple ground, mineral billboards, trees, V back).

## Follow-ups done (same day)

- View rotation: Q/E quarter turns (`rotateView`); billboards and ledges follow when a turn ends.
- Foothills: a mountain next to hills sends a ridge down to them (`ridgeProfiles`).
- Billboards stand at the highest ground right around their foot (`standingHeight`): stacks on
  slopes don't sink in.
- Nodes: `frontend/web/nodes3d.js`, a crystal per realm (sorcery: floating octahedron with orbiting
  shards; nature: a crystal cluster; chaos: a jagged crystal over its peak) that turns, bobs and
  pulses, a glow, and motes rising in a shader. Trees keep off the node's middle.
- Rebuild speed: river courses and range segments cached per rebuild, typed per-tile arrays for
  terrain codes, rivers and sites, no allocation per sample; sites count only on hills and
  mountains in the 3x3. Full rebuild ~135 ms, a quarter turn ~45 ms, an exploration step ~11 ms.
- The hint bar shows Q/E, V and H with the 3D map.
- Per-plane colours (`palettes` in terrain-simple.js, `planeShades` in vegetation.js): Myrror has
  brown-purple ground, ashen rock with lavender snow, dark violet water and dark purple woods;
  nodes keep their colours. Nodes glow brighter (a wider pulsing halo) for now.

## Next / open

- The smoke test's pact check failed once (passed on rerun): maybe flaky.
- Decide which look is the default.
- 045's model registry: low-poly models for encounters and cities.
