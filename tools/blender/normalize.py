"""Normalise a generated GLB (tools/comfyui, Hunyuan3D) into a map model: one mesh, few faces.

    blender --background --python tools/blender/normalize.py -- in.glb out.glb [--faces 1200]
        [--cut 0.03] [--smooth 6] [--keep 0.02] [--voxel 0.015]
        [--image source.png [--colours 8] [--front auto] [--mirror 0] [--settle 2]]

Steps: join the meshes; rebuild the surface with a voxel remesh (cells of --voxel of the height:
the voxel-to-mesh output has edges shared by more than two faces, which the decimator won't
collapse; this also closes small holes); drop loose pieces with fewer than --keep of
the faces (crumbs of a base slab); cut away the bottom --cut of the height (a thin ground slab
the image showed); smooth the voxel staircase (--smooth iterations); decimate to about --faces
triangles; centre it on x/z, stand it on y = 0 and scale it to a height of 1; flat shading.
Prints the face counts along the way.

Colours (--image, the picture the model was made from): Hunyuan3D takes the picture as the
model's front view, so it's projected straight back along --front (the axis the camera looks
from, in Blender's z-up axes: x, -x, y, -y; --mirror 1 flips it; "auto" picks the way the
model's silhouette best covers the object in the picture): the object's box in the picture (what isn't
white background) is fitted to the model's box seen from the front. Each face takes the colour
behind its middle (the background next to the object is filled from the nearest object pixel),
then the faces' colours are reduced to --colours clusters (k-means): flat colours from a few, as
the polygon look wants; then --settle passes where each face takes the colour most of it and its
neighbours have (the projection is a little off here and there: stray specks go). Stored per face corner (exported as COLOR_0).
"""

import sys

import bmesh
import bpy


TEXT_OPTIONS = ("image", "front")


def options():
    arguments = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    settings = {"faces": 1200, "cut": 0.03, "smooth": 6, "keep": 0.02, "voxel": 0.015,
                "image": None, "colours": 8, "front": "auto", "mirror": 0, "settle": 2}
    paths = []
    index = 0
    while index < len(arguments):
        argument = arguments[index]
        if argument.startswith("--"):
            name = argument[2:]
            settings[name] = arguments[index + 1] if name in TEXT_OPTIONS else float(arguments[index + 1])
            index += 2
        else:
            paths.append(argument)
            index += 1
    if len(paths) != 2:
        sys.exit(__doc__)
    return paths[0], paths[1], settings


def apply_modifier(target, kind, **properties):
    modifier = target.modifiers.new(kind.lower(), kind)
    for name, value in properties.items():
        setattr(modifier, name, value)
    with bpy.context.temp_override(object=target, active_object=target):
        bpy.ops.object.modifier_apply(modifier=modifier.name)


# what a point looks like from the front: (across, up)
FRONTS = {
    "-y": lambda co: (co.x, co.z),
    "y": lambda co: (-co.x, co.z),
    "x": lambda co: (co.y, co.z),
    "-x": lambda co: (-co.y, co.z),
}


class Picture:
    """The source picture, small, with the background filled from the nearest object pixel."""

    size = 256

    def __init__(self, path):
        image = bpy.data.images.load(path)
        image.scale(self.size, self.size)
        pixels = image.pixels[:]
        size = self.size
        colours = [None] * (size * size)
        # the background: light, grey pixels joined to the picture's border (a faint halo around
        # the object is background too; light parts inside it aren't)
        def light(index):
            red, green, blue = pixels[index * 4:index * 4 + 3]
            return (red + green + blue) / 3 > 0.7 and max(red, green, blue) - min(red, green, blue) < 0.1
        background = bytearray(size * size)
        stack = [index for index in range(size * size)
                 if (index % size in (0, size - 1) or index // size in (0, size - 1)) and light(index)]
        for index in stack:
            background[index] = 1
        while stack:
            index = stack.pop()
            column, row = index % size, index // size
            for other_column, other_row in ((column + 1, row), (column - 1, row), (column, row + 1), (column, row - 1)):
                if 0 <= other_column < size and 0 <= other_row < size:
                    other = other_row * size + other_column
                    if not background[other] and light(other):
                        background[other] = 1
                        stack.append(other)
        frontier = []
        for index in range(size * size):
            if not background[index]:
                colours[index] = tuple(pixels[index * 4:index * 4 + 3])
                frontier.append(index)
        if not frontier:
            sys.exit(f"{path}: nothing but background")
        columns = [index % size for index in frontier]
        rows = [index // size for index in frontier]
        # the object's box, in pixels (rows count up from the bottom, as Blender's)
        self.box = (min(columns), max(columns) + 1, min(rows), max(rows) + 1)
        while frontier:
            following = []
            for index in frontier:
                column, row = index % size, index // size
                for other_column, other_row in ((column + 1, row), (column - 1, row), (column, row + 1), (column, row - 1)):
                    if 0 <= other_column < size and 0 <= other_row < size:
                        other = other_row * size + other_column
                        if colours[other] is None:
                            colours[other] = colours[index]
                            following.append(other)
            frontier = following
        self.colours = colours
        self.background = background

    def covers(self, across, up):
        """Whether a point of the object's box (0..1 each way) is on the object."""
        left, right, bottom, top = self.box
        column = min(self.size - 1, max(0, int(left + across * (right - left))))
        row = min(self.size - 1, max(0, int(bottom + up * (top - bottom))))
        return not self.background[row * self.size + column]

    def at(self, across, up):
        """The colour at a point of the object's box (0..1 each way)."""
        left, right, bottom, top = self.box
        column = min(self.size - 1, max(0, int(left + across * (right - left))))
        row = min(self.size - 1, max(0, int(bottom + up * (top - bottom))))
        return self.colours[row * self.size + column]


def kmeans(colours, count):
    """colours reduced to `count` clusters: each colour's cluster centre."""
    distance = lambda a, b: sum((a[axis] - b[axis]) ** 2 for axis in range(3))
    distinct = sorted(set(colours))
    count = min(int(count), len(distinct))
    # start from the colours furthest apart (a hue few faces have, a teal roof, keeps its own)
    centres = [max(distinct, key=colours.count)]
    while len(centres) < count:
        centres.append(max(distinct, key=lambda colour: min(distance(colour, centre) for centre in centres)))
    for _ in range(12):
        groups = [[] for _ in centres]
        for colour in colours:
            nearest = min(range(len(centres)), key=lambda index: sum((colour[axis] - centres[index][axis]) ** 2 for axis in range(3)))
            groups[nearest].append(colour)
        centres = [tuple(sum(colour[axis] for colour in group) / len(group) for axis in range(3)) if group else centres[index]
                   for index, group in enumerate(groups)]
    return [min(centres, key=lambda centre: sum((colour[axis] - centre[axis]) ** 2 for axis in range(3))) for colour in colours]


def paint(target, picture, box, settings):
    """Each face the picture's colour behind its middle, reduced to a few colours."""
    front = FRONTS[settings["front"]]
    low_across, high_across, low_up, high_up = box
    colours = []
    for polygon in target.data.polygons:
        across, up = front(polygon.center)
        across = (across - low_across) / (high_across - low_across)
        if settings["mirror"]:
            across = 1 - across
        colours.append(picture.at(across, (up - low_up) / (high_up - low_up)))
    colours = kmeans(colours, settings["colours"])
    # neighbours: faces sharing an edge
    by_edge = {}
    for polygon in target.data.polygons:
        for key in polygon.edge_keys:
            by_edge.setdefault(key, []).append(polygon.index)
    neighbours = [set() for _ in target.data.polygons]
    for faces in by_edge.values():
        for face in faces:
            neighbours[face].update(other for other in faces if other != face)
    for _ in range(int(settings["settle"])):
        settled = []
        for index, colour in enumerate(colours):
            votes = {colour: 1.5}
            for other in neighbours[index]:
                votes[colours[other]] = votes.get(colours[other], 0) + 1
            settled.append(max(votes, key=votes.get))
        colours = settled
    attribute = target.data.color_attributes.new(name="Color", type="BYTE_COLOR", domain="CORNER")
    for polygon, colour in zip(target.data.polygons, colours):
        for loop in polygon.loop_indices:
            attribute.data[loop].color_srgb = (*colour, 1.0)
    target.data.color_attributes.active_color = attribute
    print(f"painted: {len(set(colours))} colours")


def front_box(points, front):
    seen = [front(point) for point in points]
    return (min(point[0] for point in seen), max(point[0] for point in seen), min(point[1] for point in seen), max(point[1] for point in seen))


def orient(points, picture, settings):
    """The front (and mirror) whose silhouette lands most on the picture's object."""
    fronts = [settings["front"]] if settings["front"] != "auto" else list(FRONTS)
    sample = points[::max(1, len(points) // 4000)]
    best = None
    for name in fronts:
        low_across, high_across, low_up, high_up = front_box(sample, FRONTS[name])
        for mirror in (0, 1):
            score = 0
            for point in sample:
                across, up = FRONTS[name](point)
                across = (across - low_across) / (high_across - low_across)
                score += picture.covers(1 - across if mirror else across, (up - low_up) / (high_up - low_up))
            if best is None or score > best[0]:
                best = (score, name, mirror)
            print(f"front {name}{' mirrored' if mirror else ''}: {score / len(sample):.2f} on the object")
    settings["front"], settings["mirror"] = best[1], best[2]
    print(f"front: {best[1]}{' mirrored' if best[2] else ''}")


def main():
    source, destination, settings = options()
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.ops.import_scene.gltf(filepath=source)
    meshes = [item for item in bpy.context.scene.objects if item.type == "MESH"]
    if not meshes:
        sys.exit(f"no mesh in {source}")
    for item in bpy.context.scene.objects:
        item.select_set(item in meshes)
    bpy.context.view_layer.objects.active = meshes[0]
    if len(meshes) > 1:
        bpy.ops.object.join()
    target = bpy.context.view_layer.objects.active
    # transforms into the vertices (the importer may leave a rotation on the node)
    bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
    print(f"imported: {len(target.data.polygons)} faces")
    # the model seen from the front, before anything is cut: the picture's object fits this box
    picture = None
    box = None
    if settings["image"]:
        picture = Picture(settings["image"])
        points = [vertex.co.copy() for vertex in target.data.vertices]
        orient(points, picture, settings)
        box = front_box(points, FRONTS[settings["front"]])
    # weld first: the voxel faces come with their own corners each
    mesh = bmesh.new()
    mesh.from_mesh(target.data)
    bmesh.ops.remove_doubles(mesh, verts=mesh.verts, dist=1e-5)
    mesh.to_mesh(target.data)
    mesh.free()
    apply_modifier(target, "REMESH", mode="VOXEL", voxel_size=max(target.dimensions) * settings["voxel"], adaptivity=0, use_smooth_shade=False)
    print(f"remeshed: {len(target.data.polygons)} faces")

    mesh = bmesh.new()
    mesh.from_mesh(target.data)
    # loose pieces: keep those with at least `keep` of the faces
    mesh.faces.ensure_lookup_table()
    seen = set()
    pieces = []
    for face in mesh.faces:
        if face.index in seen:
            continue
        piece = []
        stack = [face]
        seen.add(face.index)
        while stack:
            current = stack.pop()
            piece.append(current)
            for edge in current.edges:
                for other in edge.link_faces:
                    if other.index not in seen:
                        seen.add(other.index)
                        stack.append(other)
        pieces.append(piece)
    total = len(mesh.faces)
    small = [face for piece in pieces if len(piece) < settings["keep"] * total for face in piece]
    bmesh.ops.delete(mesh, geom=small, context="FACES")
    print(f"pieces: {len(pieces)}, dropped {len(small)} faces in small ones")
    # the bottom slab (Blender is z-up)
    if settings["cut"] > 0:
        low = min(vertex.co.z for vertex in mesh.verts)
        high = max(vertex.co.z for vertex in mesh.verts)
        cut = low + (high - low) * settings["cut"]
        geometry = mesh.verts[:] + mesh.edges[:] + mesh.faces[:]
        bmesh.ops.bisect_plane(mesh, geom=geometry, dist=1e-6, plane_co=(0, 0, cut), plane_no=(0, 0, 1), clear_inner=True)
        # close the cut so it has a floor
        edges = [edge for edge in mesh.edges if edge.is_boundary and all(abs(vertex.co.z - cut) < 1e-4 for vertex in edge.verts)]
        if edges:
            bmesh.ops.holes_fill(mesh, edges=edges, sides=0)
    mesh.to_mesh(target.data)
    mesh.free()
    print(f"cleaned: {len(target.data.polygons)} faces")

    if settings["smooth"] > 0:
        apply_modifier(target, "LAPLACIANSMOOTH", iterations=int(settings["smooth"]), lambda_factor=0.5, use_volume_preserve=True)
    # triangles, so --faces counts what the renderer draws
    apply_modifier(target, "TRIANGULATE")
    faces = len(target.data.polygons)
    if faces > settings["faces"]:
        apply_modifier(target, "DECIMATE", decimate_type="COLLAPSE", ratio=settings["faces"] / faces, use_collapse_triangulate=True)
    print(f"decimated: {len(target.data.polygons)} faces")
    if settings["image"]:
        paint(target, picture, box, settings)

    # centre on x/y, base at z = 0, height 1
    corners = [vertex.co for vertex in target.data.vertices]
    low = [min(corner[axis] for corner in corners) for axis in range(3)]
    high = [max(corner[axis] for corner in corners) for axis in range(3)]
    scale = 1 / (high[2] - low[2])
    for vertex in target.data.vertices:
        vertex.co.x = (vertex.co.x - (low[0] + high[0]) / 2) * scale
        vertex.co.y = (vertex.co.y - (low[1] + high[1]) / 2) * scale
        vertex.co.z = (vertex.co.z - low[2]) * scale
    for polygon in target.data.polygons:
        polygon.use_smooth = False
    target.data.update()

    export = dict(filepath=destination, export_format="GLB", use_selection=False, export_materials="NONE",
                  export_normals=True, export_apply=True)
    if settings["image"]:
        export["export_vertex_color"] = "ACTIVE"
    bpy.ops.export_scene.gltf(**export)
    print(f"saved {destination}: {len(target.data.polygons)} faces")


main()
