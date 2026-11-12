# Handoff 047: generated 3D models (ComfyUI → Blender)

Date: 2026-11-12
Previous: [046](046-simple-terrain.md).

Towards the polygon look's models (CLAUDE.md "Visual style"): text → image → mesh on the user's
ComfyUI server (a laptop: one job at a time), then a Blender script makes a map model of it.

## What's there (uncommitted at the end of the session)

- `tools/comfyui/text-to-image.json`, `image-to-3d.json`: the user's workflows (API format):
  Anima preview 3 (Qwen 0.6B text encoder), Hunyuan3D 2.1 (shape only).
- `tools/comfyui/comfy.py`: the client (standard library). `image "<prompt>" out.png`,
  `model in.png out.glb`. Server from `COMFYUI_HOSTS` (host:port list, first that answers);
  waits for an empty queue before each job.
  - image prompt wrapped with "low poly, flat shading, flat colors ... no shadows, even
    lighting, plain white background": colours that are materials, not light (what the
    projection needs). Adding "no ground, no base" avoids a ground slab.
  - model defaults: `--octree 64` (the workflow's 256 gives ~500k triangles), `--threshold 0.15`
    (0.6 leaves thin walls full of holes at 64), `--mesher basic` (the workflow's "surface net"
    is tangled: a third of its edges have 3-4 faces, so Blender can neither decimate nor
    remesh it; "basic" is closed voxel faces).
  - same image + seed: ComfyUI reuses the sampled shape, a decode-only retry takes seconds.
- `tools/blender/normalize.py` (`blender --background --python ... -- in.glb out.glb`): weld,
  voxel remesh, drop crumbs, cut the bottom (`--cut`), smooth, triangulate, decimate to
  `--faces`, centre/stand/scale to height 1. With `--image` (the source picture): project its
  colours along the model's front (`--front auto` scores each axis by how much of the
  silhouette lands on the object), k-means to `--colours` (farthest-apart start), `--settle`
  passes of neighbour voting; exported as COLOR_0.

## Findings

- Timing on the laptop: image ~2 min, shape ~3-7 min, decode at octree 64 seconds; Blender
  seconds.
- The flat-lit tower (blue pyramid roof, grey walls) painted well at 300-600 triangles.
- Anima can't draw a turnaround: "reference sheet, multiple views" repeats the same angle.
- Smoothing rounds a pyramid roof into a cone; less smoothing shows voxel steps.

## Next / open

- The user's 4-view generate-and-texture workflow failed on Mac MPS (a float32/float64
  incompatibility); look at it, then project per face from the view it faces most.
- A face's colour snapped to its side's shade (the roof's two shades meet in a sawtooth).
- Load a model on the map (polygon look) in place of a billboard.
- Licences: Anima preview 3 and Hunyuan3D 2.1 (territory, commercial use) before generated
  models go in the repo.
