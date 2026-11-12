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
// water by its distance from the coast: shallow by the land, deep from deepFrom tiles out (the
// colours are the plane's: palettes)
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

// ranges: neighbouring mountains (or hills) are joined by a ridge from peak to peak, dipping to a
// saddle halfway; a mountain next to hills sends a foothill ridge down to them. Both tiles of a
// pair compute the same ridge, so it runs on unbroken across their border where their shapes
// blend (SimpleGround.rangeSegments)
const rangeKinds = { "Mountain": "mountain", "Volcano": "mountain", "Chaos Node": "mountain", "Hills": "hill" };
const neighbourSteps = [[1, 0], [-1, 0], [0, 1], [0, -1], [1, 1], [1, -1], [-1, 1], [-1, -1]];
// per pair of kinds (this tile's, then its neighbour's): the ridge's height at this tile's peak
// and at the neighbour's, how far it dips between them, and its half width
const ridgeProfiles = {
    "mountain/mountain": { from: 0.6, to: 0.6, dip: 0.15, width: 0.32 },
    "hill/hill": { from: 0.15, to: 0.15, dip: 0.04, width: 0.42 },
    "mountain/hill": { from: 0.45, to: 0.12, dip: 0.04, width: 0.36 },
    "hill/mountain": { from: 0.12, to: 0.45, dip: 0.04, width: 0.36 },
};

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
    // the ranges to neighbouring mountains and hills, rocky along their crest
    const ridge = ground.rangeHeight(wx, wz, tx, tz) * (0.75 + 0.5 * rocky);
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

// each plane's colours: Arcanus green and bright, Myrror gloomier (brown-purple ground, ashen rock,
// pale lavender snow, dark violet water); nodes keep their magic colours on both
const palettes = {
    Arcanus: {
        grass: [0x457d37, 0x518b3f, 0x609945, 0x73a34d, 0x82a254],
        forest: [0x24542a, 0x2c6230, 0x386e36, 0x44783c],
        swamp: [0x3a4a3a, 0x46522e, 0x56603a, 0x4a5a40, 0x3c5852],
        desert: [0xc99a4a, 0xd6ae4c, 0xe2c062, 0xedd286, 0xe8c878],
        tundra: [0x96a294, 0xa8b4aa, 0xc2ccc6, 0xdde4e0],
        hills: [0x6c7a44, 0x7a7444, 0x857a4a, 0x8c6c48, 0x7c6444],
        hillTop: 0x8a8478,
        rock: [0x6a5642, 0x7a6450, 0x847a70, 0x908c88, 0xa09c98],
        snow: 0xf0f0f0,
        volcano: [0x4e423a, 0x443a36, 0x3a3434, 0x302c2c],
        sand: 0xd6c48a,
        seabed: 0xc8b07a,
        water: 0x3a74b8,
        deepWater: 0x17336a,
    },
    Myrror: {
        grass: [0x4a4236, 0x54493a, 0x5e503e, 0x584650, 0x665648],
        forest: [0x2e2a2c, 0x362e34, 0x3e3240, 0x463a42],
        swamp: [0x2e3430, 0x3a3834, 0x443e3a, 0x3a3644, 0x343e4a],
        desert: [0x8e765e, 0x9c8262, 0xa88e6a, 0xb49a74, 0x9e866e],
        tundra: [0x7e788c, 0x8e889c, 0x9e98ac, 0xb2acbe],
        hills: [0x584840, 0x62503e, 0x6c5648, 0x5c4a4e, 0x685250],
        hillTop: 0x746a6e,
        rock: [0x463a3c, 0x544648, 0x625456, 0x706466, 0x7e7274],
        snow: 0xd6cce4,
        volcano: [0x3a3030, 0x342a2c, 0x2c2628, 0x241f22],
        sand: 0x9a8a7c,
        seabed: 0x6a5e58,
        water: 0x34467c,
        deepWater: 0x10152e,
    },
};

// per terrain: its level (tiles), vertices per tile edge, the shape on top of the level, and its
// colour (given the ground's final height there), in a plane's colours
function makeLooks(palette) {
    return {
        "Ocean": { level: -0.2, subdivisions: 2 },
        "Lake": { level: -0.16, subdivisions: 2 },
        "Shore": { level: -0.12, subdivisions: 2 },
        "River": { ...flatLook(0.06, 0.01, 4, palette.grass, 1), subdivisions: 16 },
        "Grasslands": flatLook(0.06, 0.015, 4, palette.grass, 1),
        "Forest": flatLook(0.08, 0.035, 5, palette.forest, 3),
        "Swamp": flatLook(0.04, 0.012, 4, palette.swamp, 4),
        "Desert": flatLook(0.07, 0.03, 3, palette.desert, 6),
        "Tundra": flatLook(0.07, 0.015, 4, palette.tundra, 7),
        "Hills": {
            level: 0.12, subdivisions: 8,
            shape: (wx, wz, tx, tz, periodX, ground) => {
                const mound = smoothstep(0, 1, peak(wx, wz, tx, tz, 0.6, periodX)) * 0.12;
                // rolling ground joining neighbouring hills, rising towards neighbouring mountains
                const ridge = ground.rangeHeight(wx, wz, tx, tz);
                const bumps = Math.pow(valueNoise(wx, wz, 3, periodX, 8), 2) * 0.14 + valueNoise(wx, wz, 6, periodX, 9) * 0.04;
                return Math.max(mound, ridge) + bumps;
            },
            colour: (out, wx, wz, height, periodX, tx, tz) => {
                tileTint(shaded(out, palette.hills, wx, wz, periodX, 10), tx, tz);
                return mixInto(out, out, colour(palette.hillTop), smoothstep(0.2, 0.36, height));
            },
        },
        "Mountain": {
            level: 0.22, subdivisions: 16,
            shape: mountainShape,
            colour: (out, wx, wz, height, periodX, tx, tz) => mountainColour(out, wx, wz, height, periodX, tx, tz, palette.rock, palette.snow, 0.78),
        },
        "Volcano": {
            level: 0.22, subdivisions: 16,
            shape: mountainShape,
            colour: (out, wx, wz, height, periodX, tx, tz) => mountainColour(out, wx, wz, height, periodX, tx, tz, palette.volcano, 0xb8401c, 0.72),
        },
        "Nature Node": flatLook(0.08, 0.035, 5, [0x2a8034, 0x3a9a40, 0x5cc25a, 0x7ad070], 3),
        "Sorcery Node": flatLook(0.06, 0.015, 4, [0x3f8a98, 0x4f9aa8, 0x7cc0c8, 0x9ad0d4], 1),
        "Chaos Node": {
            level: 0.22, subdivisions: 16,
            shape: mountainShape,
            colour: (out, wx, wz, height, periodX, tx, tz) => mountainColour(out, wx, wz, height, periodX, tx, tz, [0x5a3028, 0x6a3a30, 0x7a4a3a, 0x8a5a4a], 0xd04a20, 0.75),
        },
        sand: palette.sand,
        seabed: palette.seabed,
    };
}
// the polygon look's palette for a plane: its terrain shades and the nodes' (linear colours, as a
// flat array of paletteSize vec3s)
function paletteColours(plane) {
    const palette = palettes[plane] || palettes.Arcanus;
    const hexes = [];
    for (const [name, value] of Object.entries(palette)) {
        if (name === "water" || name === "deepWater") continue;
        hexes.push(...(Array.isArray(value) ? value : [value]));
    }
    hexes.push(0x2a8034, 0x3a9a40, 0x5cc25a, 0x7ad070, 0x3f8a98, 0x4f9aa8, 0x7cc0c8, 0x9ad0d4, 0x5a3028, 0x6a3a30, 0x7a4a3a, 0x8a5a4a, 0xd04a20, 0xb8401c, 0x101010);
    const values = new Float32Array(paletteSize * 3);
    for (let index = 0; index < paletteSize; index++) {
        const value = new THREE.Color(hexes[index < hexes.length ? index : 0]);
        values.set([value.r, value.g, value.b], index * 3);
    }
    return values;
}

const planeLooks = { Arcanus: makeLooks(palettes.Arcanus), Myrror: makeLooks(palettes.Myrror) };
// the plane being built (SimpleGround.update sets it; one ground at a time)
let terrainLooks = planeLooks.Arcanus;

const waterFloor = (out) => mixInto(out, colour(terrainLooks.seabed), colour(terrainLooks.seabed), 0);
const unexploredColour = (out) => mixInto(out, colour(0x101010), colour(0x101010), 0);

function lookOf(terrain) {
    return terrainLooks[terrain] || terrainLooks["Grasslands"];
}

// the polygon look builds coarser flat land (2 vertices per edge, not 4): bigger, bolder faces
// (SimpleGround.update sets it, with terrainLooks)
let coarseFlat = false;

function subdivisionsOf(terrain) {
    if (terrain === null) return 2;
    const subdivisions = lookOf(terrain).subdivisions;
    return coarseFlat && subdivisions === 4 ? 2 : subdivisions;
}

// how far the polygon look moves a patch's inner vertices, in cells: irregular faces instead of a
// grid (the patch's border stays, so neighbours still meet)
const jitter = 0.32;
// the polygon look bends the whole mesh sideways (smooth noise, the same for a vertex shared by
// neighbours): borders between tiles wander instead of following the grid. Up to warpAmount
// tiles, over waves of 1/warpFrequency tiles
const warpAmount = 0.16;
const warpFrequency = 1.25;

// how much of tile centre at distance `distance` (on one axis) a point takes: 1 well inside, 0 well
// outside, blending over the band at the edge; the two sides of an edge add up to 1
function ramp(distance) {
    return 1 - smoothstep(0.5 - blendBand, 0.5 + blendBand, distance);
}

// the polygon look's shading: four flat tones instead of a smooth gradient (shared with
// vegetation.js)
export const lightBands = `
#ifdef STYLIZED
float lightBand(float light) {
    return light < 0.62 ? 0.5 : light < 0.88 ? 0.74 : light < 1.06 ? 1.0 : 1.14;
}
#endif
`;
// how many colours the polygon look's palette holds (shorter palettes repeat their first)
const paletteSize = 64;
// the ambient light of the polygon look: a darker shadow side
export const stylizedAmbient = 0.4;

const vertexShader = `
uniform vec2 mapSize;
varying vec2 overlayUv;
varying vec2 tileUv;
varying vec3 worldPosition;
#ifdef STYLIZED
flat varying vec3 groundColour;
#else
varying vec3 groundColour;
#endif
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

const fragmentShader = /* glsl */ `
uniform sampler2D overlay;
uniform vec2 mapSize;
uniform vec3 tint;
uniform vec3 lightDirection;
uniform float ambient;
uniform sampler2D depthMap;
uniform vec3 deepTint;
varying vec2 overlayUv;
varying vec2 tileUv;
varying vec3 worldPosition;
#ifdef STYLIZED
flat varying vec3 groundColour;
uniform vec3 palette[PALETTE_SIZE];
// the nearest colour of the plane's palette (compared in a roughly perceptual space)
vec3 snapToPalette(vec3 colour) {
    vec3 best = palette[0];
    float bestDistance = 1e9;
    vec3 target = sqrt(colour);
    for (int index = 0; index < PALETTE_SIZE; index++) {
        vec3 difference = sqrt(palette[index]) - target;
        float distance = dot(difference, difference);
        if (distance < bestDistance) {
            bestDistance = distance;
            best = palette[index];
        }
    }
    return best;
}
#else
varying vec3 groundColour;
#endif
${lightBands}
void main() {
    // the facet's normal (flat shading)
    vec3 normal = normalize(cross(dFdx(worldPosition), dFdy(worldPosition)));
    if (normal.y < 0.0) normal = -normal;
    // flat ground at full brightness, as the textured map's baked shading
    float light = min(1.25, ambient + (1.0 - ambient) * max(0.0, dot(normal, lightDirection)) / lightDirection.y);
#ifdef STYLIZED
    light = lightBand(light);
#endif
    vec4 painted = texture2D(overlay, overlayUv);
#ifdef WATER
    // darker away from the coast (the distance per tile, smoothed between tile centres; in
    // clean bands when stylized)
#ifdef STYLIZED
    // the bands' borders wander like the land's (waves that repeat across the map's width, so
    // they join at its east-west edge)
    vec2 local = tileUv * mapSize;
    float waves = floor(mapSize.x * 1.3 / 6.2832 + 0.5) * 6.2832 / mapSize.x;
    vec2 wobble = vec2(sin(local.y * 2.1 + sin(local.x * waves) * 1.5), cos(local.x * waves * 2.0 + sin(local.y * 1.7) * 1.5)) * 0.3;
    float depth = texture2D(depthMap, tileUv + wobble / mapSize).r;
    depth = floor(depth * 2.999) / 2.0;
#else
    float depth = texture2D(depthMap, tileUv).r;
#endif
    vec3 surface = mix(tint, deepTint, depth);
#else
    vec3 surface = groundColour * tint;
#ifdef STYLIZED
    surface = snapToPalette(surface);
#endif
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
            deepTint: { value: new THREE.Color(palettes.Arcanus.deepWater) },
            palette: { value: paletteColours("Arcanus") },
        };
        this.ambient = ambient;
        this.stylized = false;
        const material = (tint, vertexColors, defines) => new THREE.ShaderMaterial({
            uniforms: { ...this.uniforms, tint: { value: new THREE.Color(tint) } },
            vertexShader,
            fragmentShader,
            vertexColors,
            defines,
        });
        this.groundMaterial = material(0xffffff, true, {});
        this.waterMaterial = material(palettes.Arcanus.water, false, { WATER: 1 });
        this.geometry = new THREE.BufferGeometry();
        this.waterGeometry = null;
        // per tile: its patch and the neighbourhood it was built for
        this.patches = new Map();
        this.terrains = null;
        this.width = 0;
        this.height = 0;
        this.scale = 1;
    }

    // the polygon look: flat faces in the plane's palette, light in bands, a darker shadow side
    setStylized(stylized) {
        if (stylized === this.stylized) return;
        this.stylized = stylized;
        // other patches: coarser, irregular, a colour per face
        this.patches.clear();
        for (const material of [this.groundMaterial, this.waterMaterial]) {
            if (stylized) {
                material.defines.STYLIZED = "";
                material.defines.PALETTE_SIZE = paletteSize;
            } else {
                delete material.defines.STYLIZED;
                delete material.defines.PALETTE_SIZE;
            }
            material.needsUpdate = true;
        }
        this.uniforms.ambient.value = stylized ? stylizedAmbient : this.ambient;
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

    // whether a tile has water in its 3x3
    byWater(x, y) {
        for (let dy = -1; dy <= 1; dy++) {
            if (y + dy < 0 || y + dy >= this.height) continue;
            for (let dx = -1; dx <= 1; dx++) {
                const terrain = this.terrainAt(x + dx, y + dy);
                if (terrain !== null && waterTerrains.has(terrain)) return true;
            }
        }
        return false;
    }

    siteAt(x, y) {
        if (!this.siteList || y < 0 || y >= this.height) return null;
        return this.siteList[y * this.width + ((x % this.width) + this.width) % this.width];
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
        // the plane's colours (another plane: every patch anew)
        const plane = palettes[mapView.plane] ? mapView.plane : "Arcanus";
        terrainLooks = planeLooks[plane];
        coarseFlat = this.stylized;
        this.waterMaterial.uniforms.tint.value.set(palettes[plane].water);
        this.uniforms.deepTint.value.set(palettes[plane].deepWater);
        if (plane !== this.plane) {
            this.patches.clear();
            this.plane = plane;
            this.uniforms.palette.value = paletteColours(plane);
        }
        if (scale !== this.scale || mapView.width !== this.width || mapView.height !== this.height) {
            this.patches.clear();
        }
        this.terrains = terrains;
        this.sites = sites;
        // per tile, for this terrain: river courses and range ridges
        this.courses = new Map();
        this.ranges = new Map();
        // per tile (y * width + x): a code for its terrain (signatures), whether it's a river,
        // and its sites
        const codes = new Map();
        this.terrainCodes = new Uint16Array(mapView.width * mapView.height);
        this.rivers = new Uint8Array(mapView.width * mapView.height);
        this.siteList = new Array(mapView.width * mapView.height).fill(null);
        terrains.forEach((row, y) => row.forEach((terrain, x) => {
            if (!codes.has(terrain)) codes.set(terrain, codes.size + 1);
            this.terrainCodes[y * mapView.width + x] = codes.get(terrain);
            this.rivers[y * mapView.width + x] = terrain === "River" ? 1 : 0;
        }));
        // tiles with a river tile in their 3x3 (only their samples look for channels)
        this.nearRivers = new Uint8Array(mapView.width * mapView.height);
        for (let y = 0; y < mapView.height; y++) {
            for (let x = 0; x < mapView.width; x++) {
                if (!this.rivers[y * mapView.width + x]) continue;
                for (let dy = -1; dy <= 1; dy++) {
                    if (y + dy < 0 || y + dy >= mapView.height) continue;
                    for (let dx = -1; dx <= 1; dx++) {
                        this.nearRivers[(y + dy) * mapView.width + ((x + dx + mapView.width) % mapView.width)] = 1;
                    }
                }
            }
        }
        for (const [key, feet] of sites) {
            const [x, y] = key.split("/").map(Number);
            if (rangeKinds[terrains[y][x]]) this.siteList[y * mapView.width + x] = feet;
        }
        this.width = mapView.width;
        this.height = mapView.height;
        this.scale = scale;
        this.patchSizes = new Uint8Array(this.width * this.height);
        for (let y = 0; y < this.height; y++) {
            for (let x = 0; x < this.width; x++) {
                const terrain = terrains[y][x];
                let size = subdivisionsOf(terrain);
                if (coarseFlat && size === 2 && terrain !== null && !waterTerrains.has(terrain) && this.byWater(x, y)) size = 4;
                this.patchSizes[y * this.width + x] = size;
            }
        }

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

        const geometry = this.stylized ? faceted(patches, indexCount) : smooth(patches, vertexCount, indexCount);
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
        const codes = neighbourCodes;
        let sites = "";
        let index = 0;
        for (let dy = -2; dy <= 2; dy++) {
            for (let dx = -2; dx <= 2; dx++) {
                const row = y + dy;
                if (row < 0 || row >= this.height) {
                    codes[index++] = 0;
                    continue;
                }
                const column = ((x + dx) % this.width + this.width) % this.width;
                codes[index++] = this.terrainCodes[row * this.width + column];
                // ledges change only their own tile's shape (and the blend around it), and only
                // on hills and mountains
                const feet = Math.abs(dx) <= 1 && Math.abs(dy) <= 1 ? this.siteList[row * this.width + column] : null;
                if (feet) {
                    for (const foot of feet) sites += `${index}:${foot.x.toFixed(3)}:${foot.z.toFixed(3)},`;
                }
            }
        }
        return String.fromCharCode.apply(null, codes) + sites;
    }

    // a tile's grid of vertices; along an edge shared with a coarser tile, the vertices between
    // the coarser one's follow the straight line between them
    buildPatch(tx, tz) {
        const size = this.subdivisionsAt(tx, tz);
        const edgeSize = (dx, dz) => {
            if (tz + dz < 0 || tz + dz >= this.height) return size;
            return Math.min(size, this.subdivisionsAt(tx + dx, tz + dz));
        };
        const left = edgeSize(-1, 0);
        const right = edgeSize(1, 0);
        const top = edgeSize(0, -1);
        const bottom = edgeSize(0, 1);
        const columns = size + 1;
        const positions = new Float32Array(columns * columns * 3);
        const colours = new Float32Array(columns * columns * 3);
        // unscaled heights, and colours without the beach (the polygon look picks sand per face)
        const heights = new Float32Array(columns * columns);
        const lands = new Float32Array(columns * columns * 3);
        const sample = { height: 0, colour: [0, 0, 0], land: [0, 0, 0] };
        const other = { height: 0, colour: [0, 0, 0], land: [0, 0, 0] };
        const shown = { x: 0, z: 0 };
        const otherShown = { x: 0, z: 0 };

        for (let row = 0; row <= size; row++) {
            for (let column = 0; column <= size; column++) {
                let wx = tx + column / size;
                let wz = tz + row / size;
                if (this.stylized && row > 0 && row < size && column > 0 && column < size) {
                    const latticeX = tx * size + column;
                    const latticeZ = tz * size + row;
                    wx += (hash(latticeX, latticeZ + 7919) - 0.5) * 2 * jitter / size;
                    wz += (hash(latticeX + 104729, latticeZ) - 0.5) * 2 * jitter / size;
                }
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
                    const ax = vertical ? wx : tx + start / size;
                    const az = vertical ? tz + start / size : wz;
                    const bx = vertical ? wx : tx + (start + step) / size;
                    const bz = vertical ? tz + (start + step) / size : wz;
                    this.sampleAt(ax, az, sample);
                    this.sampleAt(bx, bz, other);
                    sample.height += (other.height - sample.height) * t;
                    mixInto(sample.colour, sample.colour, other.colour, t);
                    mixInto(sample.land, sample.land, other.land, t);
                    // on the straight line between the coarser neighbour's (bent) vertices
                    this.warp(ax, az, shown);
                    this.warp(bx, bz, otherShown);
                    shown.x += (otherShown.x - shown.x) * t;
                    shown.z += (otherShown.z - shown.z) * t;
                } else {
                    this.sampleAt(wx, wz, sample);
                    this.warp(wx, wz, shown);
                }
                const vertex = row * columns + column;
                const index = vertex * 3;
                positions[index] = shown.x;
                positions[index + 1] = this.scaled(sample.height);
                positions[index + 2] = shown.z;
                heights[vertex] = sample.height;
                for (let axis = 0; axis < 3; axis++) {
                    colours[index + axis] = sample.colour[axis];
                    lands[index + axis] = sample.land[axis];
                }
            }
        }
        return { size, positions, colours, heights, lands };
    }

    // a tile's vertices per edge: its terrain's, but in the polygon look land by the water is
    // finer (a ring of faces for the beach)
    subdivisionsAt(x, y) {
        if (y < 0 || y >= this.height) return 2;
        return this.patchSizes[y * this.width + ((x % this.width) + this.width) % this.width];
    }

    // where the mesh shows a point of the ground (the polygon look's bend; wraps east-west, fades
    // out at the map's north and south edges)
    warp(x, z, out) {
        if (!this.stylized) {
            out.x = x;
            out.z = z;
            return out;
        }
        const edge = Math.max(0, Math.min(1, z, this.height - z));
        out.x = x + (valueNoise(x, z, warpFrequency, this.width, 31) - 0.5) * 2 * warpAmount;
        out.z = z + (valueNoise(x, z, warpFrequency, this.width, 37) - 0.5) * 2 * warpAmount * edge;
        return out;
    }

    // the point of the ground the mesh shows at (x, z): the bend undone (a few fixed-point steps;
    // the bend is gentle enough to converge)
    unwarp(x, z, out) {
        out.x = x;
        out.z = z;
        if (!this.stylized) return out;
        for (let step = 0; step < 5; step++) {
            this.warp(out.x, out.z, unwarpScratch);
            out.x = x - (unwarpScratch.x - out.x);
            out.z = z - (unwarpScratch.z - out.z);
        }
        return out;
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
        // the tiles around the point that take part of it (up to 4), and how much
        const tilesX = sampleTilesX;
        const tilesZ = sampleTilesZ;
        const weights = sampleWeights;
        const terrains = sampleTerrains;
        let count = 0;
        for (let dz = -1; dz <= 1; dz++) {
            const wzWeight = ramp(Math.abs(wz - (fz + dz + 0.5)));
            if (wzWeight <= 0) continue;
            for (let dx = -1; dx <= 1; dx++) {
                const wxWeight = ramp(Math.abs(wx - (fx + dx + 0.5)));
                if (wxWeight <= 0) continue;
                tilesX[count] = fx + dx;
                tilesZ[count] = fz + dz;
                weights[count] = wxWeight * wzWeight;
                count++;
            }
        }

        let height = 0;
        for (let index = 0; index < count; index++) {
            const tx = tilesX[index];
            const tz = tilesZ[index];
            const terrain = this.terrainAt(tx, tz);
            terrains[index] = terrain;
            if (terrain === null) {
                height += weights[index] * unexploredLevel;
                continue;
            }
            const look = lookOf(terrain);
            let shape = look.shape ? look.shape(wx, wz, tx, tz, periodX, this) : 0;
            // a ledge where a lair, city or mineral stands, so it doesn't perch on a slope or
            // sink into one
            const feet = shape > 0 && rangeKinds[terrain] ? this.siteAt(tx, tz) : null;
            if (feet) {
                for (const foot of feet) {
                    shape *= smoothstep(siteRadius * 0.4, siteRadius, Math.hypot(wx - tx - foot.x, wz - tz - foot.z));
                }
            }
            height += weights[index] * (look.level + shape);
        }
        height = this.carveRivers(wx, wz, fx, fz, height);

        const colourSum = out.colour;
        colourSum[0] = colourSum[1] = colourSum[2] = 0;
        const part = samplePart;
        for (let index = 0; index < count; index++) {
            const terrain = terrains[index];
            if (terrain === null) {
                unexploredColour(part);
            } else if (waterTerrains.has(terrain)) {
                waterFloor(part);
            } else {
                lookOf(terrain).colour(part, wx, wz, height, periodX, tilesX[index], tilesZ[index]);
            }
            colourSum[0] += part[0] * weights[index];
            colourSum[1] += part[1] * weights[index];
            colourSum[2] += part[2] * weights[index];
        }
        if (out.land) {
            out.land[0] = colourSum[0];
            out.land[1] = colourSum[1];
            out.land[2] = colourSum[2];
        }
        // sand along the water
        mixInto(colourSum, colourSum, colour(terrainLooks.sand), 1 - smoothstep(0, beachHeight, height));
        out.height = height;
        return out;
    }

    // a tile's ridges to its neighbours (rangeKinds, ridgeProfiles), from its corner, cached per
    // terrain: [{ax, az, abx, abz, length, from, to, dip, width}]
    rangeSegments(tx, tz) {
        const column = ((tx % this.width) + this.width) % this.width;
        const key = tz * this.width + column;
        let segments = this.ranges.get(key);
        if (!segments) {
            segments = [];
            const kind = rangeKinds[this.terrainAt(tx, tz)];
            if (kind) {
                const [ax, az] = peakCentre(column, tz, this.width);
                for (const [dx, dz] of neighbourSteps) {
                    if (tz + dz < 0 || tz + dz >= this.height) continue;
                    const profile = ridgeProfiles[`${kind}/${rangeKinds[this.terrainAt(column + dx, tz + dz)]}`];
                    if (!profile) continue;
                    const [bx, bz] = peakCentre(column + dx, tz + dz, this.width);
                    const abx = bx - ax;
                    const abz = bz - az;
                    segments.push({ ax: ax - column, az: az - tz, abx, abz, length: abx * abx + abz * abz, ...profile });
                }
            }
            this.ranges.set(key, segments);
        }
        return segments;
    }

    // the highest ridge of a tile's ranges at a point (0 off them)
    rangeHeight(wx, wz, tx, tz) {
        let best = 0;
        for (const segment of this.rangeSegments(tx, tz)) {
            const ax = tx + segment.ax;
            const az = tz + segment.az;
            const t = Math.max(0, Math.min(1, ((wx - ax) * segment.abx + (wz - az) * segment.abz) / segment.length));
            const across = Math.hypot(wx - ax - segment.abx * t, wz - az - segment.abz * t);
            if (across >= segment.width) continue;
            const crest = segment.from + (segment.to - segment.from) * t - segment.dip * 4 * t * (1 - t);
            best = Math.max(best, crest * Math.pow(1 - across / segment.width, 1.5));
        }
        return best;
    }

    // a river tile's course: from its middle (moved a little per tile, so rivers wind) to the
    // middle of each edge it shares with another river tile or water; a lone river tile gets a
    // short stretch across
    // (from the tile's corner, cached per terrain; the same for a tile and its copy across the
    // map's east-west edge)
    riverCourse(tx, tz) {
        const column = ((tx % this.width) + this.width) % this.width;
        const key = tz * this.width + column;
        let course = this.courses.get(key);
        if (!course) {
            const cx = 0.5 + (hash(column * 3 + 7, tz * 17 + 1) - 0.5) * 0.3;
            const cz = 0.5 + (hash(column * 19 + 2, tz * 5 + 3) - 0.5) * 0.3;
            const ends = [];
            for (const [dx, dz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
                const neighbour = tz + dz < 0 || tz + dz >= this.height ? null : this.terrainAt(tx + dx, tz + dz);
                if (neighbour !== null && riverJoins.has(neighbour)) {
                    ends.push([0.5 + dx * 0.5, 0.5 + dz * 0.5]);
                }
            }
            if (ends.length === 0) {
                ends.push([cx - 0.3, cz], [cx + 0.3, cz]);
            }
            course = { cx, cz, ends };
            this.courses.set(key, course);
        }
        return course;
    }

    // the ground lowered along the river channels near a point (the river tiles around it)
    carveRivers(wx, wz, fx, fz, height) {
        const row = Math.max(0, Math.min(this.height - 1, fz));
        if (!this.nearRivers[row * this.width + ((fx % this.width) + this.width) % this.width]) return height;
        let nearest = Infinity;
        for (let dz = -1; dz <= 1; dz++) {
            for (let dx = -1; dx <= 1; dx++) {
                const tx = fx + dx;
                const tz = fz + dz;
                if (tz < 0 || tz >= this.height || !this.rivers[tz * this.width + ((tx % this.width) + this.width) % this.width]) continue;
                const { cx, cz, ends } = this.riverCourse(tx, tz);
                for (const [ex, ez] of ends) {
                    nearest = Math.min(nearest, distanceToSegment(wx, wz, tx + cx, tz + cz, tx + ex, tz + ez));
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
        const point = this.unwarp(wx, wz, heightPoint);
        return this.groundHeight(point.x, point.z);
    }

    // the same at a point of the ground before the polygon look's bend (where the mesh shows it:
    // warp)
    groundHeight(x, z) {
        if (!this.terrains) return 0;
        const sample = this.sampleAt(x, Math.max(0, Math.min(this.height - 0.001, z)), heightSample);
        return Math.max(0, this.scaled(sample.height));
    }
}

const heightSample = { height: 0, colour: [0, 0, 0] };
const heightPoint = { x: 0, z: 0 };
const unwarpScratch = { x: 0, z: 0 };
const neighbourCodes = new Uint16Array(25);
// scratch space for sampleAt (no allocation per sample)
const sampleTilesX = new Int32Array(4);
const sampleTilesZ = new Int32Array(4);
const sampleWeights = new Float64Array(4);
const sampleTerrains = new Array(4);
const samplePart = [0, 0, 0];

function distanceToSegment(px, pz, ax, az, bx, bz) {
    const abx = bx - ax;
    const abz = bz - az;
    const length = abx * abx + abz * abz;
    const t = length > 0 ? Math.max(0, Math.min(1, ((px - ax) * abx + (pz - az) * abz) / length)) : 0;
    return Math.hypot(px - (ax + abx * t), pz - (az + abz * t));
}

// the ground's mesh from its patches: shared vertices, colours blending across faces
function smooth(patches, vertexCount, indexCount) {
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
    return geometry;
}

// the polygon look's mesh: every face its own three vertices and one colour (the average of its
// corners', so a border between colours runs along face edges instead of a sawtooth; sand if it
// reaches down to the water), the
// diagonals turning around the patch's middle (a fan, less of a grid)
function faceted(patches, indexCount) {
    const positions = new Float32Array(indexCount * 3);
    const colours = new Float32Array(indexCount * 3);
    let offset = 0;
    for (const patch of patches) {
        if (!patch.faces) patch.faces = patchFaces(patch);
        positions.set(patch.faces.positions, offset);
        colours.set(patch.faces.colours, offset);
        offset += patch.faces.positions.length;
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute("color", new THREE.BufferAttribute(colours, 3));
    return geometry;
}

// one patch's faces for faceted (kept with the patch)
function patchFaces(patch) {
    const size = patch.size;
    const positions = new Float32Array(size * size * 18);
    const colours = new Float32Array(size * size * 18);
    const source = patch.positions;
    const tint = patch.lands;
    const heights = patch.heights;
    const sand = colour(terrainLooks.sand);
    let offset = 0;
    const face = (a, b, c) => {
        // a face reaching down to the water is beach, all of it
        const beach = Math.min(heights[a], heights[b], heights[c]) < beachHeight;
        for (let axis = 0; axis < 3; axis++) {
            const average = beach ? sand[axis] : (tint[a * 3 + axis] + tint[b * 3 + axis] + tint[c * 3 + axis]) / 3;
            positions[offset + axis] = source[a * 3 + axis];
            positions[offset + 3 + axis] = source[b * 3 + axis];
            positions[offset + 6 + axis] = source[c * 3 + axis];
            colours[offset + axis] = colours[offset + 3 + axis] = colours[offset + 6 + axis] = average;
        }
        offset += 9;
    };
    const columns = size + 1;
    const half = size / 2;
    for (let row = 0; row < size; row++) {
        for (let column = 0; column < size; column++) {
            const a = row * columns + column;
            const b = a + 1;
            const c = a + columns;
            const d = c + 1;
            if ((row < half) === (column < half)) {
                face(a, c, d);
                face(a, d, b);
            } else {
                face(a, c, b);
                face(b, c, d);
            }
        }
    }
    return { positions, colours };
}
