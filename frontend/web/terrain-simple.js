// The 3D map's "simple" ground (?terrain=simple, or the V key): plain colours per terrain type
// instead of the original's tile art, with shapes: flat land with a little unevenness, rolling
// bumps on hills, spiky peaks on mountains, rivers cut as channels. Water is one blue plane at
// height 0; land stands above it and water tiles' ground sinks below it.
//
// Each tile is its own patch of vertices, finer where there is shape to show (subdivisionsOf);
// along an edge shared with a coarser tile the finer patch follows the coarser one's vertices, so
// no cracks open between them. Heights and colours blend between tiles over a band at their edges
// (blendBand). Patches are cached per tile and rebuilt only when their 3x3 neighbourhood changes
// (exploring reveals a few tiles at a time).
//
// The shader shades each facet by its slope (flat shading from screen-space derivatives: the
// low-poly look) and lays the 2D map's canvas over it: roads, fog, the dimming, the grid and paths
// (MapRenderer with drawsTerrain off paints only those, on a transparent canvas). Water uses the
// same shader, so fog and paths cover it too.
//
// World units are tiles: tile (x, y) covers world x [x, x+1] and z [y, y+1] (y up), as map3d.js.

import * as THREE from "three";

// a fixed pseudo-random value in [0, 1) for a lattice point
export function hash(x, y) {
    let value = Math.imul(x, 374761393) + Math.imul(y, 668265263);
    value = Math.imul(value ^ (value >>> 13), 1274126177);
    return ((value ^ (value >>> 16)) >>> 0) / 4294967296;
}

// half the width of the band at a tile's edge where it blends into its neighbours, in tiles
const blendBand = 0.2;
// rivers: the channel's half width, its bed's depth, and how far its bank slopes
const riverHalfWidth = 0.08;
const riverBed = -0.07;
const riverBank = 0.1;
// where the land turns to sand along the water (heights below this)
const beachHeight = 0.035;
// water by its distance from the coast: shallow by the land, deep from deepFrom tiles out
const waterColor = 0x3a74b8;
const deepWaterColor = 0x17336a;
const deepFrom = 3;
const unexploredLevel = 0.06;
// the flat ledge around a lair's or city's foot on hills and mountains, in tiles
const siteRadius = 0.32;
const waterTerrains = new Set(["Ocean", "Lake", "Shore"]);
// a river flows into these
const riverJoins = new Set(["River", "Ocean", "Lake", "Shore"]);

function smoothstep(edge0, edge1, value) {
    const t = Math.min(1, Math.max(0, (value - edge0) / (edge1 - edge0)));
    return t * t * (3 - 2 * t);
}

// smooth value noise on a lattice of `frequency` cells per tile; wraps east-west after the map's
// width (periodX tiles), so the ground joins across the map's edge
function valueNoise(x, z, frequency, periodX, seed) {
    const gx = x * frequency;
    const gz = z * frequency;
    const period = periodX * frequency;
    const ix = Math.floor(gx);
    const iz = Math.floor(gz);
    const fx = gx - ix;
    const fz = gz - iz;
    const sx = fx * fx * (3 - 2 * fx);
    const sz = fz * fz * (3 - 2 * fz);
    const column = (value) => ((value % period) + period) % period;
    const x0 = column(ix);
    const x1 = column(ix + 1);
    const row = iz + seed * 7919;
    const a = hash(x0, row);
    const b = hash(x1, row);
    const c = hash(x0, row + 1);
    const d = hash(x1, row + 1);
    return (a + (b - a) * sx) * (1 - sz) + (c + (d - c) * sx) * sz;
}

// colours, in the renderer's linear working space
const palette = {};
function colour(hex) {
    if (!palette[hex]) {
        const value = new THREE.Color(hex);
        palette[hex] = [value.r, value.g, value.b];
    }
    return palette[hex];
}

function mixInto(out, a, b, t) {
    out[0] = a[0] + (b[0] - a[0]) * t;
    out[1] = a[1] + (b[1] - a[1]) * t;
    out[2] = a[2] + (b[2] - a[2]) * t;
    return out;
}

// a tile's peak: near its centre, moved a little per tile so a range doesn't line up
function peakCentre(tx, tz, periodX) {
    // the same for a tile and its copy across the map's east-west edge
    const column = ((tx % periodX) + periodX) % periodX;
    return [tx + 0.5 + (hash(column * 7 + 1, tz * 13 + 5) - 0.5) * 0.25, tz + 0.5 + (hash(column * 11 + 3, tz * 5 + 9) - 0.5) * 0.25];
}

function peak(wx, wz, tx, tz, radius, periodX) {
    const [cx, cz] = peakCentre(tx, tz, periodX);
    return Math.max(0, 1 - Math.hypot(wx - cx, wz - cz) / radius);
}

// ranges: neighbouring tiles of the same kind are joined by a ridge from peak to peak, dipping to
// a saddle halfway. Both tiles compute the same ridge (it is symmetric), so it runs on unbroken
// across their border where their shapes blend
const rangeKinds = { "Mountain": "mountain", "Volcano": "mountain", "Chaos Node": "mountain", "Hills": "hill" };
const neighbourSteps = [[1, 0], [-1, 0], [0, 1], [0, -1], [1, 1], [1, -1], [-1, 1], [-1, -1]];

function range(wx, wz, tx, tz, periodX, ground, kind, width, top, saddle) {
    const [ax, az] = peakCentre(tx, tz, periodX);
    let best = 0;
    for (const [dx, dz] of neighbourSteps) {
        if (tz + dz < 0 || tz + dz >= ground.height) continue;
        if (rangeKinds[ground.terrainAt(tx + dx, tz + dz)] !== kind) continue;
        const [bx, bz] = peakCentre(tx + dx, tz + dz, periodX);
        const abx = bx - ax;
        const abz = bz - az;
        const t = Math.max(0, Math.min(1, ((wx - ax) * abx + (wz - az) * abz) / (abx * abx + abz * abz)));
        const across = Math.hypot(wx - ax - abx * t, wz - az - abz * t);
        if (across >= width) continue;
        const crest = saddle + (top - saddle) * 4 * (t - 0.5) * (t - 0.5);
        best = Math.max(best, crest * Math.pow(1 - across / width, 1.5));
    }
    return best;
}

// sharp ridges: value noise folded at its middle
function ridges(wx, wz, frequency, periodX, seed) {
    const value = 1 - Math.abs(2 * valueNoise(wx, wz, frequency, periodX, seed) - 1);
    return value * value * value;
}

function mountainShape(wx, wz, tx, tz, periodX, ground) {
    const cone = Math.pow(peak(wx, wz, tx, tz, 0.5, periodX), 1.8);
    // a second, smaller peak beside the main one
    const side = Math.pow(peak(wx + 0.22, wz - 0.14, tx, tz, 0.3, periodX), 1.8);
    const rocky = ridges(wx, wz, 3, periodX, 11) * 0.6 + ridges(wx, wz, 6, periodX, 12) * 0.4;
    const own = 0.75 * cone + 0.3 * side + 0.25 * rocky * (0.3 + 0.7 * cone);
    // the range to neighbouring mountains, rocky along its crest
    const ridge = range(wx, wz, tx, tz, periodX, ground, "mountain", 0.32, 0.6, 0.45) * (0.75 + 0.5 * rocky);
    return Math.max(own, ridge);
}

// a tile's own shade: each tile a little lighter or darker than its neighbours
function tileTint(out, tx, tz) {
    const factor = 0.92 + hash(tx * 31 + 17, tz * 37 + 29) * 0.16;
    out[0] *= factor;
    out[1] *= factor;
    out[2] *= factor;
    return out;
}

// patches of a few shades: noise picks a place along the list, the shades meeting at fairly sharp
// borders (a patchwork more than a gradient)
function shaded(out, shades, wx, wz, periodX, seed) {
    const value = valueNoise(wx, wz, 2, periodX, seed) * 0.6 + valueNoise(wx, wz, 5, periodX, seed + 50) * 0.4;
    const place = Math.min(0.999, Math.max(0, (value - 0.2) / 0.6)) * (shades.length - 1);
    const index = Math.floor(place);
    return mixInto(out, colour(shades[index]), colour(shades[index + 1]), smoothstep(0.3, 0.7, place - index));
}

// a flat terrain's look: a little unevenness, patches of its shades, each tile its own tint
function flatLook(level, bumps, bumpFrequency, shades, seed) {
    return {
        level, subdivisions: 4,
        shape: (wx, wz, tx, tz, periodX) => valueNoise(wx, wz, bumpFrequency, periodX, seed) * bumps,
        colour: (out, wx, wz, height, periodX, tx, tz) => tileTint(shaded(out, shades, wx, wz, periodX, seed + 1), tx, tz),
    };
}

// rock by height (several shades), and a top colour (snow, lava) on the high parts
function mountainColour(out, wx, wz, height, periodX, tx, tz, shades, top, topFrom) {
    const variation = valueNoise(wx, wz, 5, periodX, 21);
    const place = Math.min(0.999, Math.max(0, smoothstep(0.15, 0.75, height + (variation - 0.5) * 0.2))) * (shades.length - 1);
    const index = Math.floor(place);
    mixInto(out, colour(shades[index]), colour(shades[index + 1]), place - index);
    tileTint(out, tx, tz);
    if (top) {
        mixInto(out, out, colour(top), smoothstep(topFrom, topFrom + 0.08, height + (variation - 0.5) * 0.06));
    }
    return out;
}

const grassShades = [0x4f8f3e, 0x5e9e48, 0x72b050, 0x8bbd5a, 0x9db862];

// per terrain: its level (tiles), vertices per tile edge, the shape on top of the level, and its
// colour (given the ground's final height there)
const terrainLooks = {
    "Ocean": { level: -0.2, subdivisions: 2 },
    "Lake": { level: -0.16, subdivisions: 2 },
    "Shore": { level: -0.12, subdivisions: 2 },
    "River": { ...flatLook(0.06, 0.01, 4, grassShades, 1), subdivisions: 16 },
    "Grasslands": flatLook(0.06, 0.015, 4, grassShades, 1),
    "Forest": flatLook(0.08, 0.035, 5, [0x24542a, 0x2c6230, 0x386e36, 0x44783c], 3),
    "Swamp": flatLook(0.04, 0.012, 4, [0x3a4a3a, 0x46522e, 0x56603a, 0x4a5a40, 0x3c5852], 4),
    "Desert": flatLook(0.07, 0.03, 3, [0xc99a4a, 0xd6ae4c, 0xe2c062, 0xedd286, 0xe8c878], 6),
    "Tundra": flatLook(0.07, 0.015, 4, [0x96a294, 0xa8b4aa, 0xc2ccc6, 0xdde4e0], 7),
    "Hills": {
        level: 0.12, subdivisions: 8,
        shape: (wx, wz, tx, tz, periodX, ground) => {
            const mound = smoothstep(0, 1, peak(wx, wz, tx, tz, 0.6, periodX)) * 0.12;
            // rolling ground joining neighbouring hills
            const ridge = range(wx, wz, tx, tz, periodX, ground, "hill", 0.42, 0.15, 0.11);
            const bumps = Math.pow(valueNoise(wx, wz, 3, periodX, 8), 2) * 0.14 + valueNoise(wx, wz, 6, periodX, 9) * 0.04;
            return Math.max(mound, ridge) + bumps;
        },
        colour: (out, wx, wz, height, periodX, tx, tz) => {
            tileTint(shaded(out, [0x6c7a44, 0x7a7444, 0x857a4a, 0x8c6c48, 0x7c6444], wx, wz, periodX, 10), tx, tz);
            return mixInto(out, out, colour(0x8a8478), smoothstep(0.2, 0.36, height));
        },
    },
    "Mountain": {
        level: 0.22, subdivisions: 16,
        shape: mountainShape,
        colour: (out, wx, wz, height, periodX, tx, tz) => mountainColour(out, wx, wz, height, periodX, tx, tz, [0x6a5642, 0x7a6450, 0x847a70, 0x908c88, 0xa09c98], 0xf0f0f0, 0.78),
    },
    "Volcano": {
        level: 0.22, subdivisions: 16,
        shape: mountainShape,
        colour: (out, wx, wz, height, periodX, tx, tz) => mountainColour(out, wx, wz, height, periodX, tx, tz, [0x4e423a, 0x443a36, 0x3a3434, 0x302c2c], 0xb8401c, 0.72),
    },
    "Nature Node": flatLook(0.08, 0.035, 5, [0x2a8034, 0x3a9a40, 0x5cc25a, 0x7ad070], 3),
    "Sorcery Node": flatLook(0.06, 0.015, 4, [0x3f8a98, 0x4f9aa8, 0x7cc0c8, 0x9ad0d4], 1),
    "Chaos Node": {
        level: 0.22, subdivisions: 16,
        shape: mountainShape,
        colour: (out, wx, wz, height, periodX, tx, tz) => mountainColour(out, wx, wz, height, periodX, tx, tz, [0x5a3028, 0x6a3a30, 0x7a4a3a, 0x8a5a4a], 0xd04a20, 0.75),
    },
};
const fallbackLook = terrainLooks["Grasslands"];
const waterFloor = (out) => mixInto(out, colour(0xc8b07a), colour(0xc8b07a), 0);
const unexploredColour = (out) => mixInto(out, colour(0x101010), colour(0x101010), 0);

function lookOf(terrain) {
    return terrainLooks[terrain] || fallbackLook;
}

function subdivisionsOf(terrain) {
    return terrain === null ? 2 : lookOf(terrain).subdivisions;
}

// how much of tile centre at distance `distance` (on one axis) a point takes: 1 well inside, 0 well
// outside, blending over the band at the edge; the two sides of an edge add up to 1
function ramp(distance) {
    return 1 - smoothstep(0.5 - blendBand, 0.5 + blendBand, distance);
}

const vertexShader = `
uniform vec2 mapSize;
varying vec2 overlayUv;
varying vec2 tileUv;
varying vec3 worldPosition;
varying vec3 groundColour;
void main() {
    overlayUv = vec2(position.x / mapSize.x, 1.0 - position.z / mapSize.y);
    tileUv = vec2(position.x / mapSize.x, position.z / mapSize.y);
#ifdef USE_COLOR
    groundColour = color;
#else
    groundColour = vec3(1.0);
#endif
    vec4 world = modelMatrix * vec4(position, 1.0);
    worldPosition = world.xyz;
    gl_Position = projectionMatrix * viewMatrix * world;
}
`;

const fragmentShader = `
uniform sampler2D overlay;
uniform vec3 tint;
uniform vec3 lightDirection;
uniform float ambient;
uniform sampler2D depthMap;
uniform vec3 deepTint;
varying vec2 overlayUv;
varying vec2 tileUv;
varying vec3 worldPosition;
varying vec3 groundColour;
void main() {
    // the facet's normal (flat shading)
    vec3 normal = normalize(cross(dFdx(worldPosition), dFdy(worldPosition)));
    if (normal.y < 0.0) normal = -normal;
    // flat ground at full brightness, as the textured map's baked shading
    float light = min(1.25, ambient + (1.0 - ambient) * max(0.0, dot(normal, lightDirection)) / lightDirection.y);
    vec4 painted = texture2D(overlay, overlayUv);
#ifdef WATER
    // darker away from the coast (the distance per tile, smoothed between tile centres)
    vec3 surface = mix(tint, deepTint, texture2D(depthMap, tileUv).r);
#else
    vec3 surface = groundColour * tint;
#endif
    gl_FragColor = vec4(mix(surface, painted.rgb, painted.a) * light, 1.0);
    #include <colorspace_fragment>
}
`;

export class SimpleGround {
    // overlay: the 2D map's canvas texture; light: where the light comes from and the shade of
    // slopes facing away (as the textured map)
    constructor(lightDirection, ambient) {
        this.uniforms = {
            overlay: { value: null },
            mapSize: { value: new THREE.Vector2(1, 1) },
            lightDirection: { value: lightDirection.clone() },
            ambient: { value: ambient },
            depthMap: { value: new THREE.DataTexture(new Uint8Array(4), 1, 1) },
            deepTint: { value: new THREE.Color(deepWaterColor) },
        };
        const material = (tint, vertexColors, defines) => new THREE.ShaderMaterial({
            uniforms: { ...this.uniforms, tint: { value: new THREE.Color(tint) } },
            vertexShader,
            fragmentShader,
            vertexColors,
            defines,
        });
        this.groundMaterial = material(0xffffff, true, {});
        this.waterMaterial = material(waterColor, false, { WATER: 1 });
        this.geometry = new THREE.BufferGeometry();
        this.waterGeometry = null;
        // per tile: its patch and the neighbourhood it was built for
        this.patches = new Map();
        this.terrains = null;
        this.width = 0;
        this.height = 0;
        this.scale = 1;
    }

    setOverlay(texture) {
        this.uniforms.overlay.value = texture;
    }

    // a ground mesh and a water plane for one copy of the map; the ground's geometry is replaced
    // by update (see copyGround)
    makeCopy(width, height) {
        if (!this.waterGeometry || this.waterSize !== `${width}x${height}`) {
            this.waterGeometry = new THREE.PlaneGeometry(width, height);
            this.waterGeometry.rotateX(-Math.PI / 2);
            this.waterGeometry.translate(width / 2, 0, height / 2);
            this.waterSize = `${width}x${height}`;
        }
        this.uniforms.mapSize.value.set(width, height);
        const ground = new THREE.Mesh(this.geometry, this.groundMaterial);
        const water = new THREE.Mesh(this.waterGeometry, this.waterMaterial);
        return { ground, water };
    }

    siteAt(x, y) {
        if (!this.sites || y < 0 || y >= this.height) return null;
        return this.sites.get(`${((x % this.width) + this.width) % this.width}/${y}`) || null;
    }

    terrainAt(x, y) {
        const row = this.terrains[Math.max(0, Math.min(this.height - 1, y))];
        return row[((x % this.width) + this.width) % this.width];
    }

    // the explored terrain (null: unexplored), the height scale (0: flat, the H key) and the
    // sites: where lairs, cities and minerals stand ("x/y" -> their feet in the tile, [{x, z}]
    // from its corner), where hills and mountains leave a flat ledge; returns the new geometry
    update(mapView, scale, sites = new Map()) {
        const terrains = mapView.tiles.map((row) => row.map((tile) => (tile ? tile.terrain : null)));
        if (scale !== this.scale || mapView.width !== this.width || mapView.height !== this.height) {
            this.patches.clear();
        }
        this.terrains = terrains;
        this.sites = sites;
        this.width = mapView.width;
        this.height = mapView.height;
        this.scale = scale;

        let vertexCount = 0;
        let indexCount = 0;
        const patches = [];
        for (let y = 0; y < this.height; y++) {
            for (let x = 0; x < this.width; x++) {
                const signature = this.neighbourhood(x, y);
                const key = y * this.width + x;
                let patch = this.patches.get(key);
                if (!patch || patch.signature !== signature) {
                    patch = this.buildPatch(x, y);
                    patch.signature = signature;
                    this.patches.set(key, patch);
                }
                patches.push(patch);
                vertexCount += patch.positions.length / 3;
                indexCount += patch.size * patch.size * 6;
            }
        }

        const positions = new Float32Array(vertexCount * 3);
        const colours = new Float32Array(vertexCount * 3);
        const indices = new Uint32Array(indexCount);
        let vertexOffset = 0;
        let indexOffset = 0;
        for (const patch of patches) {
            positions.set(patch.positions, vertexOffset * 3);
            colours.set(patch.colours, vertexOffset * 3);
            const size = patch.size;
            const columns = size + 1;
            for (let row = 0; row < size; row++) {
                for (let column = 0; column < size; column++) {
                    const a = vertexOffset + row * columns + column;
                    const b = a + 1;
                    const c = a + columns;
                    const d = c + 1;
                    indices[indexOffset++] = a;
                    indices[indexOffset++] = c;
                    indices[indexOffset++] = b;
                    indices[indexOffset++] = b;
                    indices[indexOffset++] = c;
                    indices[indexOffset++] = d;
                }
            }
            vertexOffset += columns * columns;
        }

        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute("color", new THREE.BufferAttribute(colours, 3));
        geometry.setIndex(new THREE.BufferAttribute(indices, 1));
        geometry.computeBoundingSphere();
        geometry.computeBoundingBox();
        this.geometry.dispose();
        this.geometry = geometry;
        this.updateDepth();
        return geometry;
    }

    // how deep each water tile is: its distance (8 neighbours, wrapping east-west) from the
    // nearest explored land, 0 by the coast to 1 from deepFrom tiles out, as a texture with a
    // texel per tile (filtered linearly, so the colour fades between tile centres)
    updateDepth() {
        const { width, height } = this;
        const distance = new Int32Array(width * height).fill(-1);
        let frontier = [];
        for (let y = 0; y < height; y++) {
            for (let x = 0; x < width; x++) {
                const terrain = this.terrains[y][x];
                if (terrain !== null && !waterTerrains.has(terrain)) {
                    distance[y * width + x] = 0;
                    frontier.push([x, y]);
                }
            }
        }
        for (let step = 1; frontier.length && step <= deepFrom + 1; step++) {
            const next = [];
            for (const [x, y] of frontier) {
                for (let dy = -1; dy <= 1; dy++) {
                    for (let dx = -1; dx <= 1; dx++) {
                        const ny = y + dy;
                        if (ny < 0 || ny >= height) continue;
                        const nx = (x + dx + width) % width;
                        if (distance[ny * width + nx] !== -1) continue;
                        distance[ny * width + nx] = step;
                        next.push([nx, ny]);
                    }
                }
            }
            frontier = next;
        }
        let texture = this.uniforms.depthMap.value;
        if (texture.image.width !== width || texture.image.height !== height) {
            texture.dispose();
            texture = new THREE.DataTexture(new Uint8Array(width * height * 4), width, height);
            texture.magFilter = THREE.LinearFilter;
            texture.minFilter = THREE.LinearFilter;
            texture.wrapS = THREE.RepeatWrapping;
            this.uniforms.depthMap.value = texture;
        }
        const data = texture.image.data;
        for (let index = 0; index < width * height; index++) {
            const tiles = distance[index] === -1 ? deepFrom : distance[index];
            data[index * 4] = Math.round(smoothstep(1, deepFrom, tiles) * 255);
        }
        texture.needsUpdate = true;
    }

    // what a tile's patch depends on: the terrain and sites of its 5x5 neighbourhood (a
    // neighbour's ranges reach towards its own neighbours)
    neighbourhood(x, y) {
        let signature = "";
        for (let dy = -2; dy <= 2; dy++) {
            for (let dx = -2; dx <= 2; dx++) {
                signature += (y + dy < 0 || y + dy >= this.height ? "-" : this.terrainAt(x + dx, y + dy) ?? "?") + ",";
                for (const foot of this.siteAt(x + dx, y + dy) || []) {
                    signature += `${foot.x.toFixed(3)}:${foot.z.toFixed(3)},`;
                }
            }
        }
        return signature;
    }

    // a tile's grid of vertices; along an edge shared with a coarser tile, the vertices between
    // the coarser one's follow the straight line between them
    buildPatch(tx, tz) {
        const size = subdivisionsOf(this.terrainAt(tx, tz));
        const edgeSize = (dx, dz) => {
            if (tz + dz < 0 || tz + dz >= this.height) return size;
            return Math.min(size, subdivisionsOf(this.terrainAt(tx + dx, tz + dz)));
        };
        const left = edgeSize(-1, 0);
        const right = edgeSize(1, 0);
        const top = edgeSize(0, -1);
        const bottom = edgeSize(0, 1);
        const columns = size + 1;
        const positions = new Float32Array(columns * columns * 3);
        const colours = new Float32Array(columns * columns * 3);
        const sample = { height: 0, colour: [0, 0, 0] };
        const other = { height: 0, colour: [0, 0, 0] };

        for (let row = 0; row <= size; row++) {
            for (let column = 0; column <= size; column++) {
                const wx = tx + column / size;
                const wz = tz + row / size;
                // the coarser neighbour's step along this edge, if any
                let step = 1;
                let along = 0;
                let vertical = false;
                if ((column === 0 || column === size) && row > 0 && row < size) {
                    step = size / (column === 0 ? left : right);
                    along = row;
                    vertical = true;
                } else if ((row === 0 || row === size) && column > 0 && column < size) {
                    step = size / (row === 0 ? top : bottom);
                    along = column;
                }
                if (step > 1 && along % step !== 0) {
                    const start = along - (along % step);
                    const t = (along - start) / step;
                    if (vertical) {
                        this.sampleAt(wx, tz + start / size, sample);
                        this.sampleAt(wx, tz + (start + step) / size, other);
                    } else {
                        this.sampleAt(tx + start / size, wz, sample);
                        this.sampleAt(tx + (start + step) / size, wz, other);
                    }
                    sample.height += (other.height - sample.height) * t;
                    mixInto(sample.colour, sample.colour, other.colour, t);
                } else {
                    this.sampleAt(wx, wz, sample);
                }
                const index = (row * columns + column) * 3;
                positions[index] = column / size + tx;
                positions[index + 1] = this.scaled(sample.height);
                positions[index + 2] = row / size + tz;
                colours[index] = sample.colour[0];
                colours[index + 1] = sample.colour[1];
                colours[index + 2] = sample.colour[2];
            }
        }
        return { size, positions, colours };
    }

    // a height by the H key's scale; flat (0), land stays just above the water and water's
    // ground just below
    scaled(height) {
        if (this.scale > 0) return height * this.scale;
        return height > 0 ? 0.002 : -0.002;
    }

    // the ground's height (unscaled) and colour at a world point: the tiles around it by how much
    // of it each takes, their level and shape, then the river channels cut in
    sampleAt(wx, wz, out) {
        const periodX = this.width;
        const fx = Math.floor(wx);
        const fz = Math.floor(wz);
        const weights = [];
        for (let dz = -1; dz <= 1; dz++) {
            const wzWeight = ramp(Math.abs(wz - (fz + dz + 0.5)));
            if (wzWeight <= 0) continue;
            for (let dx = -1; dx <= 1; dx++) {
                const wxWeight = ramp(Math.abs(wx - (fx + dx + 0.5)));
                if (wxWeight <= 0) continue;
                weights.push({ tx: fx + dx, tz: fz + dz, weight: wxWeight * wzWeight });
            }
        }

        let height = 0;
        for (const entry of weights) {
            const terrain = this.terrainAt(entry.tx, entry.tz);
            entry.terrain = terrain;
            if (terrain === null) {
                height += entry.weight * unexploredLevel;
                continue;
            }
            const look = lookOf(terrain);
            let shape = look.shape ? look.shape(wx, wz, entry.tx, entry.tz, periodX, this) : 0;
            // a ledge where a lair, city or mineral stands, so it doesn't perch on a slope or
            // sink into one
            const feet = shape > 0 ? this.siteAt(entry.tx, entry.tz) : null;
            for (const foot of feet || []) {
                shape *= smoothstep(siteRadius * 0.4, siteRadius, Math.hypot(wx - entry.tx - foot.x, wz - entry.tz - foot.z));
            }
            height += entry.weight * (look.level + shape);
        }
        height = this.carveRivers(wx, wz, fx, fz, height);

        const colourSum = out.colour;
        colourSum[0] = colourSum[1] = colourSum[2] = 0;
        const part = [0, 0, 0];
        for (const entry of weights) {
            const terrain = entry.terrain;
            if (terrain === null) {
                unexploredColour(part);
            } else if (waterTerrains.has(terrain)) {
                waterFloor(part);
            } else {
                lookOf(terrain).colour(part, wx, wz, height, periodX, entry.tx, entry.tz);
            }
            colourSum[0] += part[0] * entry.weight;
            colourSum[1] += part[1] * entry.weight;
            colourSum[2] += part[2] * entry.weight;
        }
        // sand along the water
        mixInto(colourSum, colourSum, colour(0xd6c48a), 1 - smoothstep(0, beachHeight, height));
        out.height = height;
        return out;
    }

    // a river tile's course: from its middle (moved a little per tile, so rivers wind) to the
    // middle of each edge it shares with another river tile or water; a lone river tile gets a
    // short stretch across
    riverCourse(tx, tz) {
        // the same for a tile and its copy across the map's east-west edge
        const column = ((tx % this.width) + this.width) % this.width;
        const cx = tx + 0.5 + (hash(column * 3 + 7, tz * 17 + 1) - 0.5) * 0.3;
        const cz = tz + 0.5 + (hash(column * 19 + 2, tz * 5 + 3) - 0.5) * 0.3;
        const ends = [];
        for (const [dx, dz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
            const neighbour = tz + dz < 0 || tz + dz >= this.height ? null : this.terrainAt(tx + dx, tz + dz);
            if (neighbour !== null && riverJoins.has(neighbour)) {
                ends.push([tx + 0.5 + dx * 0.5, tz + 0.5 + dz * 0.5]);
            }
        }
        if (ends.length === 0) {
            ends.push([cx - 0.3, cz], [cx + 0.3, cz]);
        }
        return { cx, cz, ends };
    }

    // the ground lowered along the river channels near a point (the river tiles around it)
    carveRivers(wx, wz, fx, fz, height) {
        let nearest = Infinity;
        for (let dz = -1; dz <= 1; dz++) {
            for (let dx = -1; dx <= 1; dx++) {
                const tx = fx + dx;
                const tz = fz + dz;
                if (tz < 0 || tz >= this.height || this.terrainAt(tx, tz) !== "River") continue;
                const { cx, cz, ends } = this.riverCourse(tx, tz);
                for (const [ex, ez] of ends) {
                    nearest = Math.min(nearest, distanceToSegment(wx, wz, cx, cz, ex, ez));
                }
            }
        }
        if (nearest === Infinity) return height;
        const bank = smoothstep(riverHalfWidth * 0.5, riverHalfWidth + riverBank, nearest);
        return Math.min(height, riverBed + (height - riverBed) * bank);
    }

    // the ground's height for things standing on it: at least the water's (ships float), by the
    // H key's scale
    heightAt(wx, wz) {
        if (!this.terrains) return 0;
        const sample = this.sampleAt(wx, Math.max(0, Math.min(this.height - 0.001, wz)), { height: 0, colour: [0, 0, 0] });
        return Math.max(0, this.scaled(sample.height));
    }
}

function distanceToSegment(px, pz, ax, az, bx, bz) {
    const abx = bx - ax;
    const abz = bz - az;
    const length = abx * abx + abz * abz;
    const t = length > 0 ? Math.max(0, Math.min(1, ((px - ax) * abx + (pz - az) * abz) / length)) : 0;
    return Math.hypot(px - (ax + abx * t), pz - (az + abz * t));
}
