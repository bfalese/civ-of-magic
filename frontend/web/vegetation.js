// Trees and shrubs on the simple ground (terrain-simple.js): small low-poly models, one
// InstancedMesh per kind (one draw call per kind per copy of the map). Forests are dense with
// broadleaf trees and conifers, grasslands and hills get a few shrubs and lone trees, tundra
// sparse small conifers, swamps dark shrubs, deserts the odd dry bush.
//
// Placements are fixed per tile (from hash) and cached by the tile's ground patch; they stay off
// water and steep slopes, and out of the way of what stands on a tile (its billboard's foot).
// Trees on explored land the player doesn't see now are dimmed like the billboards.

import * as THREE from "three";
import { hash } from "./terrain-simple.js";

// per terrain: how many of each kind a tile may get, tried at jittered spots (a tile gets
// `tries` spots; each picks a kind by these weights, or nothing)
const plantings = {
    "Forest": { tries: 25, kinds: { broadleaf: 0.5, conifer: 0.42, shrub: 0.08 } },
    "Nature Node": { tries: 16, kinds: { broadleaf: 0.7, shrub: 0.25 } },
    "Grasslands": { tries: 4, kinds: { shrub: 0.25, broadleaf: 0.08 } },
    "River": { tries: 4, kinds: { shrub: 0.3, broadleaf: 0.1 } },
    "Hills": { tries: 4, kinds: { shrub: 0.3, conifer: 0.15 } },
    "Tundra": { tries: 3, kinds: { conifer: 0.2, shrub: 0.1 } },
    "Swamp": { tries: 5, kinds: { shrub: 0.5, broadleaf: 0.08 } },
    "Desert": { tries: 3, kinds: { dryShrub: 0.15 } },
    "Sorcery Node": { tries: 3, kinds: { shrub: 0.2 } },
    "Mountain": { tries: 3, kinds: { conifer: 0.15 } },
};

// canopy shades per kind and terrain (a few, picked per plant)
const canopyShades = {
    broadleaf: [0x2e6a2a, 0x3a7a30, 0x4a8a34, 0x58923a],
    conifer: [0x1e4a2e, 0x24563a, 0x2c6034],
    shrub: [0x3e7a30, 0x4c8a36, 0x5a9440, 0x6a8e3a],
    dryShrub: [0x8a7a3a, 0x9a8444, 0x7a6a34],
    swampShrub: [0x3a5030, 0x44582e, 0x34483a],
    tundraConifer: [0x3a5a48, 0x46644e],
};
const trunkColour = 0x5a4028;
// how dark plants on explored land the player doesn't see now are (as dimColor in map3d.js)
const dimColour = 0.55;
// plants stay this far (tiles) from a billboard's foot, off ground lower than this (water, the
// river channels, the beach) and off ground steeper than this
const clearRadius = 0.26;
const lowestGround = 0.035;
const steepest = 0.9;

// a part of a model: its geometry (non-indexed), its colour, and whether the plant's shade
// tints it (canopies) or not (trunks)
function part(geometry, hex, tinted) {
    const flat = geometry.index ? geometry.toNonIndexed() : geometry;
    const count = flat.attributes.position.count;
    const colours = new Float32Array(count * 3);
    const value = new THREE.Color(hex);
    for (let index = 0; index < count; index++) {
        colours[index * 3] = value.r;
        colours[index * 3 + 1] = value.g;
        colours[index * 3 + 2] = value.b;
    }
    return { positions: flat.attributes.position.array, colours, tint: new Float32Array(count).fill(tinted ? 1 : 0) };
}

function merge(parts) {
    const total = parts.reduce((sum, item) => sum + item.tint.length, 0);
    const positions = new Float32Array(total * 3);
    const colours = new Float32Array(total * 3);
    const tint = new Float32Array(total);
    let offset = 0;
    for (const item of parts) {
        positions.set(item.positions, offset * 3);
        colours.set(item.colours, offset * 3);
        tint.set(item.tint, offset);
        offset += item.tint.length;
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute("color", new THREE.BufferAttribute(colours, 3));
    geometry.setAttribute("tint", new THREE.BufferAttribute(tint, 1));
    return geometry;
}

// the models, in tiles, standing on y = 0
function makeModels() {
    const trunk = (height, radius) => new THREE.CylinderGeometry(radius * 0.8, radius, height, 5).translate(0, height / 2, 0);
    const conifer = merge([
        part(trunk(0.05, 0.014), trunkColour, false),
        part(new THREE.ConeGeometry(0.07, 0.13, 6).translate(0, 0.1, 0), 0xffffff, true),
        part(new THREE.ConeGeometry(0.052, 0.11, 6).translate(0, 0.18, 0), 0xffffff, true),
    ]);
    const broadleaf = merge([
        part(trunk(0.07, 0.014), trunkColour, false),
        part(new THREE.IcosahedronGeometry(0.068, 0).scale(1, 0.9, 1).translate(0, 0.12, 0), 0xffffff, true),
        part(new THREE.IcosahedronGeometry(0.045, 0).translate(0.03, 0.17, -0.02), 0xffffff, true),
    ]);
    const shrub = merge([
        part(new THREE.IcosahedronGeometry(0.04, 0).scale(1.2, 0.7, 1).translate(0, 0.02, 0), 0xffffff, true),
        part(new THREE.IcosahedronGeometry(0.028, 0).translate(0.03, 0.025, 0.015), 0xffffff, true),
    ]);
    const dryShrub = merge([
        part(new THREE.DodecahedronGeometry(0.03, 0).scale(1.2, 0.6, 1).translate(0, 0.015, 0), 0xffffff, true),
    ]);
    return { conifer, broadleaf, shrub, dryShrub };
}

const vertexShader = `
attribute float tint;
varying vec3 worldPosition;
varying vec3 surfaceColour;
void main() {
    vec3 shade = vec3(1.0);
#ifdef USE_INSTANCING_COLOR
    shade = instanceColor;
#endif
    // canopies take the plant's shade; trunks keep their colour
    surfaceColour = mix(color, shade, tint);
    vec4 world = modelMatrix * instanceMatrix * vec4(position, 1.0);
    worldPosition = world.xyz;
    gl_Position = projectionMatrix * viewMatrix * world;
}
`;

const fragmentShader = `
uniform vec3 lightDirection;
uniform float ambient;
varying vec3 worldPosition;
varying vec3 surfaceColour;
void main() {
    vec3 normal = normalize(cross(dFdx(worldPosition), dFdy(worldPosition)));
    float light = min(1.25, ambient + (1.0 - ambient) * max(0.0, dot(normal, lightDirection)) / lightDirection.y);
    gl_FragColor = vec4(surfaceColour * light, 1.0);
    #include <colorspace_fragment>
}
`;

export class Vegetation {
    constructor(lightDirection, ambient) {
        this.models = makeModels();
        this.material = new THREE.ShaderMaterial({
            uniforms: { lightDirection: { value: lightDirection.clone() }, ambient: { value: ambient } },
            vertexShader,
            fragmentShader,
            vertexColors: true,
        });
        // per tile: its plants and the ground patch they were placed on
        this.placements = new Map();
        this.meshes = [];
    }

    // the plants of a tile: [{kind, x, y, z, scale, turn, shade}] (world position on the ground)
    plantsOf(ground, terrain, tx, tz) {
        const planting = plantings[terrain];
        if (!planting) return [];
        const plants = [];
        const tries = planting.tries;
        const side = Math.ceil(Math.sqrt(tries));
        for (let index = 0; index < tries; index++) {
            const roll = hash(tx * 97 + index * 13, tz * 89 + index * 7);
            let kind = null;
            let sum = 0;
            for (const [name, weight] of Object.entries(planting.kinds)) {
                sum += weight;
                if (roll < sum) {
                    kind = name;
                    break;
                }
            }
            if (!kind) continue;
            // a jittered grid over the tile, kept off its very edge
            const cellX = index % side;
            const cellZ = Math.floor(index / side);
            const x = tx + 0.03 + (cellX + hash(tx * 5 + index, tz * 3 + 11)) / side * 0.94;
            const z = tz + 0.03 + (cellZ + hash(tx * 7 + 19, tz * 11 + index)) / side * 0.94;
            const y = ground.heightAt(x, z);
            if (y < lowestGround * Math.max(ground.scale, 0.05)) continue;
            const slope = Math.hypot(ground.heightAt(x + 0.04, z) - ground.heightAt(x - 0.04, z), ground.heightAt(x, z + 0.04) - ground.heightAt(x, z - 0.04)) / 0.08;
            if (slope > steepest) continue;
            let palette = canopyShades[kind];
            if (terrain === "Swamp" && kind === "shrub") palette = canopyShades.swampShrub;
            if (terrain === "Tundra" && kind === "conifer") palette = canopyShades.tundraConifer;
            plants.push({
                kind, x, y, z,
                scale: 0.8 + hash(tx * 3 + index * 17, tz * 13 + 5) * 0.5,
                turn: hash(tx * 23 + index, tz * 29 + index * 3) * Math.PI * 2,
                shade: palette[Math.floor(hash(tx * 41 + index * 5, tz * 43 + 1) * palette.length)],
            });
        }
        return plants;
    }

    // build the meshes for the explored map: per copy of the map (the groups), one InstancedMesh
    // per kind; feet: "x/y" -> the feet ([{x, z}] from the tile's corner) of what stands on a tile
    update(ground, mapView, groups, feet) {
        this.remove();
        const byKind = { conifer: [], broadleaf: [], shrub: [], dryShrub: [] };
        mapView.tiles.forEach((row, tz) => {
            row.forEach((tile, tx) => {
                if (!tile) return;
                const key = tz * mapView.width + tx;
                const patch = ground.patches.get(key);
                let placed = this.placements.get(key);
                if (!placed || placed.patch !== patch) {
                    placed = { patch, plants: this.plantsOf(ground, tile.terrain, tx, tz) };
                    this.placements.set(key, placed);
                }
                const tileFeet = feet.get(`${tx}/${tz}`) || [];
                for (const plant of placed.plants) {
                    if (tileFeet.some((foot) => Math.hypot(plant.x - tx - foot.x, plant.z - tz - foot.z) < clearRadius)) continue;
                    byKind[plant.kind].push({ plant, dimmed: !tile.visible });
                }
            });
        });

        const matrix = new THREE.Matrix4();
        const rotation = new THREE.Quaternion();
        const up = new THREE.Vector3(0, 1, 0);
        const position = new THREE.Vector3();
        const size = new THREE.Vector3();
        const shade = new THREE.Color();
        for (const [kind, entries] of Object.entries(byKind)) {
            if (entries.length === 0) continue;
            const matrices = new Float32Array(entries.length * 16);
            const colours = new Float32Array(entries.length * 3);
            entries.forEach(({ plant, dimmed }, index) => {
                rotation.setFromAxisAngle(up, plant.turn);
                position.set(plant.x, plant.y - 0.005, plant.z);
                size.setScalar(plant.scale);
                matrix.compose(position, rotation, size).toArray(matrices, index * 16);
                shade.set(plant.shade);
                if (dimmed) shade.multiplyScalar(dimColour);
                shade.toArray(colours, index * 3);
            });
            const matrixAttribute = new THREE.InstancedBufferAttribute(matrices, 16);
            const colourAttribute = new THREE.InstancedBufferAttribute(colours, 3);
            for (const group of groups) {
                const mesh = new THREE.InstancedMesh(this.models[kind], this.material, entries.length);
                mesh.instanceMatrix = matrixAttribute;
                mesh.instanceColor = colourAttribute;
                mesh.frustumCulled = false;
                group.add(mesh);
                this.meshes.push(mesh);
            }
        }
    }

    // take the plants off the map (another look, or before rebuilding)
    remove() {
        for (const mesh of this.meshes) {
            if (mesh.parent) mesh.parent.remove(mesh);
        }
        this.meshes = [];
    }

    // forget the placements (the ground changed scale: heights moved)
    reset() {
        this.placements.clear();
    }
}
