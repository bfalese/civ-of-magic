#!/usr/bin/env python3
"""Run this folder's ComfyUI workflows (API format) against a ComfyUI server, one job at a time.

    python3 tools/comfyui/comfy.py image "a stone wizard tower" out/tower.png [--seed N] [--size 1024]
        [--width W --height H] [--views]
    python3 tools/comfyui/comfy.py model out/tower.png out/tower.glb [--seed N] [--octree 64] [--latent 4096] [--threshold 0.15] [--mesher basic]

--octree: the voxel grid the shape is decoded on (triangles grow with its square; 64 by default,
~25k triangles, the workflow's 256 gives ~500k); --latent: how many shape tokens are generated (4096, what Hunyuan3D 2.1 was
trained on; fewer: a simpler but rougher shape); --threshold: where the surface is cut from the
decoded field (lower: thicker, fewer holes in thin walls on a coarse grid; 0.15 by default, the
workflow's 0.6 leaves walls full of holes at octree 64); --mesher: VoxelToMesh's algorithm,
"basic" (the voxels' faces: a closed blocky surface tools/blender/normalize.py can remesh) by
default, or the workflow's "surface net" (smoother, but tangled: edges shared by 3-4 faces).

The server: COMFYUI_HOSTS, a comma-separated list of host:port tried in order (the first that
answers is used); localhost:8188 by default. A job waits until the server's queue is empty
before it goes in, so it never runs beside another one (the server is a laptop).

Workflows: text-to-image.json (Anima, a Qwen text encoder) and image-to-3d.json (Hunyuan3D 2.1:
shape only, no texture). Only the standard library.
"""

import json
import os
import random
import sys
import time
import urllib.parse
import urllib.request
import uuid

HERE = os.path.dirname(os.path.abspath(__file__))
# what goes around the prompt: one object, whole and plain, lit evenly (its colours are its
# materials, for tools/blender/normalize.py to project), for the image-to-3D step
IMAGE_PREFIX = "masterpiece, best quality, score_7, safe, low poly, flat shading, flat colors, "
IMAGE_SUFFIX = (", single object, centered, whole object in view, three-quarter view, plain white background,"
                " no shadows, even lighting, simple shapes, no text")
# --views: the object from several sides on one sheet (to cut up)
VIEWS_SUFFIX = (", reference sheet, multiple views, turnaround, front view, side view, back view, the same object"
                " repeated in a row, plain white background, no shadows, even lighting, simple shapes, no text")


def server():
    hosts = os.environ.get("COMFYUI_HOSTS", "localhost:8188").split(",")
    for host in hosts:
        url = f"http://{host.strip()}"
        try:
            with urllib.request.urlopen(f"{url}/system_stats", timeout=4):
                return url
        except OSError:
            continue
    sys.exit(f"no ComfyUI server answers at {', '.join(hosts)}")


def get_json(url):
    with urllib.request.urlopen(url, timeout=30) as response:
        return json.load(response)


def wait_idle(url):
    while True:
        queue = get_json(f"{url}/queue")
        if not queue.get("queue_running") and not queue.get("queue_pending"):
            return
        print("the server is busy: waiting", flush=True)
        time.sleep(10)


def node(workflow, class_type, title=None):
    for key, value in workflow.items():
        if value["class_type"] == class_type and (title is None or title in value.get("_meta", {}).get("title", "")):
            return value
    sys.exit(f"the workflow has no {class_type} node" + (f" titled {title!r}" if title else ""))


def upload(url, path):
    """Upload an input image; returns the name the server gave it."""
    boundary = uuid.uuid4().hex
    with open(path, "rb") as file:
        data = file.read()
    name = os.path.basename(path)
    body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"image\"; filename=\"{name}\"\r\n"
            f"Content-Type: image/png\r\n\r\n").encode() + data + \
           f"\r\n--{boundary}\r\nContent-Disposition: form-data; name=\"overwrite\"\r\n\r\ntrue\r\n--{boundary}--\r\n".encode()
    request = urllib.request.Request(f"{url}/upload/image", data=body, headers={"Content-Type": f"multipart/form-data; boundary={boundary}"})
    with urllib.request.urlopen(request, timeout=60) as response:
        result = json.load(response)
    return result["name"] if not result.get("subfolder") else f"{result['subfolder']}/{result['name']}"


def run(url, workflow, out_path):
    """Queue a workflow, wait for it, and save its (first) output file to out_path."""
    wait_idle(url)
    request = urllib.request.Request(f"{url}/prompt", data=json.dumps({"prompt": workflow, "client_id": uuid.uuid4().hex}).encode(),
                                     headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            prompt_id = json.load(response)["prompt_id"]
    except urllib.error.HTTPError as error:
        sys.exit(f"the server refused the workflow: {error.read().decode()[:2000]}")
    started = time.time()
    while True:
        history = get_json(f"{url}/history/{prompt_id}").get(prompt_id)
        if history:
            break
        time.sleep(3)
    status = history.get("status", {})
    if status.get("status_str") == "error":
        sys.exit(f"the job failed: {json.dumps(status.get('messages'))[:2000]}")
    for output in history.get("outputs", {}).values():
        for files in output.values():
            if not isinstance(files, list):
                continue
            for entry in files:
                if isinstance(entry, dict) and entry.get("filename"):
                    query = urllib.parse.urlencode({"filename": entry["filename"], "subfolder": entry.get("subfolder", ""), "type": entry.get("type", "output")})
                    os.makedirs(os.path.dirname(os.path.abspath(out_path)), exist_ok=True)
                    with urllib.request.urlopen(f"{url}/view?{query}", timeout=120) as response, open(out_path, "wb") as file:
                        file.write(response.read())
                    print(f"saved {out_path} ({time.time() - started:.0f}s)")
                    return
    sys.exit(f"the job saved nothing: {json.dumps(history.get('outputs'))[:1000]}")


def option(arguments, name, default):
    if name in arguments:
        index = arguments.index(name)
        value = arguments[index + 1]
        del arguments[index:index + 2]
        return value
    return default


def main():
    arguments = sys.argv[1:]
    seed = int(option(arguments, "--seed", random.randrange(2 ** 48)))
    size = int(option(arguments, "--size", 1024))
    width = int(option(arguments, "--width", size))
    height = int(option(arguments, "--height", size))
    views = "--views" in arguments
    if views:
        arguments.remove("--views")
    octree = option(arguments, "--octree", "64")
    latent = option(arguments, "--latent", None)
    threshold = option(arguments, "--threshold", "0.15")
    mesher = option(arguments, "--mesher", "basic")
    if len(arguments) != 3 or arguments[0] not in ("image", "model"):
        sys.exit(__doc__)
    command, source, out_path = arguments
    url = server()
    if command == "image":
        with open(os.path.join(HERE, "text-to-image.json")) as file:
            workflow = json.load(file)
        node(workflow, "CLIPTextEncode", "Positive")["inputs"]["text"] = IMAGE_PREFIX + source + (VIEWS_SUFFIX if views else IMAGE_SUFFIX)
        node(workflow, "KSampler")["inputs"]["seed"] = seed
        latent = node(workflow, "EmptyLatentImage")["inputs"]
        latent["width"] = width
        latent["height"] = height
    else:
        with open(os.path.join(HERE, "image-to-3d.json")) as file:
            workflow = json.load(file)
        node(workflow, "LoadImage")["inputs"]["image"] = upload(url, source)
        node(workflow, "KSampler")["inputs"]["seed"] = seed
        if octree:
            node(workflow, "VAEDecodeHunyuan3D")["inputs"]["octree_resolution"] = int(octree)
        if threshold:
            node(workflow, "VoxelToMesh")["inputs"]["threshold"] = float(threshold)
        if mesher:
            node(workflow, "VoxelToMesh")["inputs"]["algorithm"] = mesher
        if latent:
            node(workflow, "EmptyLatentHunyuan3Dv2")["inputs"]["resolution"] = int(latent)
    print(f"{command} (seed {seed}) on {url}", flush=True)
    run(url, workflow, out_path)


if __name__ == "__main__":
    main()
