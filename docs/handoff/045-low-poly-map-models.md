# Handoff 045: low-poly models on the overland map (first step into 3D assets)

Date: 2026-11-10
Previous: [044](044-small-features-done.md). Start-here doc: [034](034-start-here.md).

A first, low-risk step towards real 3D assets (034 step 7, the 3D/retro look): replace the
billboards of the static things on the strategic map with low-poly models. Nothing here is
animated, so it is a way to get the asset pipeline, loading, lighting and instancing right before
units (which will come later with vertex animation textures, see "Later" below).

## Where things stand

In `frontend/web/map3d.js`:

- `buildBillboards()` puts a `THREE.Sprite` on every tile with a standing thing: encounters
  (`encounter/<slug>`, see `core/assets.Encounter`), nodes (tiles whose terrain ends in "Node"),
  cities and stacks. Sprites stand at `heightAt()` of the tile centre, one group per map copy
  (the map wraps east-west: three copies).
- Encounter types (`core/maplib/map.go`): Monster Lair, Mysterious Cave, Tower, Ancient Temple,
  Fallen Temple, Ruins, Abandoned Keep, Dungeon; nodes: Chaos, Nature, Sorcery.
- Forests are not objects: they are painted into the ground texture.
- The ground is unlit: slope shading is baked into vertex colours (`lightDirection`,
  `shadeAmbient = 0.55`).
- Picking raycasts the billboard groups, then the ground (`groundHit`).

## Plan, in order (one PR each)

### 1. Model registry with billboard fallback

- New `frontend/web/models.js`: asset ID → `{ url, scale, rotation, offset }`.
- Add `"three/addons/": ".../three@0.160.0/examples/jsm/"` to the import map in `index.html`
  and load with `GLTFLoader`. Cache one loaded scene per URL; place clones.
- In `buildBillboards()`: if the asset ID has a model, add a mesh at the same spot; otherwise keep
  the billboard. Things can then be replaced one by one without breaking anything.
- Models are normalised to tiles (world units = tiles): a footprint of about 0.8 tile.
- A query flag (`?models=0`) to compare with the billboards.

### 2. The eight encounters

- One model per encounter type.
- A fixed random rotation around Y per tile from `hash(x, y)`, so repeated types don't look stamped.
- Dimming (explored but not seen): per-instance colour (or a dimmed material variant), at
  `dimColor`.
- Picking: add the model groups to the raycast in `pick` (or pick by tile under the hit).

### 3. The three nodes

- A model per node realm, with simple code-driven motion (a rotating crystal, a pulsing emissive
  colour) in `afterDraw`, replacing the four-frame sprite animation. No baked animation needed.

### 4. Trees on forest tiles

- `InstancedMesh` per tree variant (2-3 variants), about 3-5 trees per Forest tile, positions and
  scale jittered with `hash()`, placed at `heightAt()`. One draw call per variant per map copy.
- Keep the painted forest under them (or tone it down to a ground colour).
- This is the instancing practice run for units later. Rocks on hills/mountains could follow the
  same way.

## Lighting and style

- Models must match the unlit, baked-shaded ground: flat shading with a `DirectionalLight` along
  the same `lightDirection` plus ambient around `shadeAmbient`, or bake the same shading into the
  models' vertex colours and draw them unlit (`MeshBasicMaterial` with `vertexColors`).
- Few colours per model, no textures (or tiny palette textures), so the later retro pass
  (low-res render, ~32-colour palette, Bayer dither, depth outlines) applies cleanly on top.

## Assets

- Start with CC0 low-poly packs (Kenney, Quaternius, KayKit: nature and fantasy sets). CC0 means
  they can ship with the game, unlike anything derived from the original MoM data.
- Keep models as GLB under a new `frontend/web/models/` (small files) with a `CREDITS.md` listing
  pack and licence per file. Original models can replace them later.

## Later: animated units with vertex animation textures (VAT)

Inspired by *Diplomacy is Not an Option*: bake each unit's animation into a texture of vertex
positions per frame, and play it in a vertex shader on an `InstancedMesh` (one draw call per unit
type, per-instance clip / time offset / speed / faction colour). Thousands of animated figures
cost almost nothing on the CPU.

- Animation sources: Mixamo (humanoids, auto-rig + mocap library), CC0 animated packs
  (Quaternius, KayKit), Cascadeur; Blender only for unusual creatures (hydras, elementals...).
- Baking: OpenVAT (Blender addon), or our own baker in JS: load the `SkinnedMesh` GLB, step it
  with `AnimationMixer`, write vertex positions into a `DataTexture` / binary at build time.
- Trade-offs: no fine animation blending or runtime attachments (weapon swaps) unless handled
  separately.
- Visual style considered for units and battles: flat-shaded, Another World-like polygons, with
  clear silhouettes per role and the faction colour on a large area.

## Open questions

- Unlit with baked vertex colours vs. a real light: decide in step 1 with one model side by side.
- Do cities move to models too (size by population), or stay sprites until the retro pass?
- Trees on forests only, or also sparse on grasslands for variety?
