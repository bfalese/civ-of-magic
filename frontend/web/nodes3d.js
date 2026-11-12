// Magic nodes on the simple ground (terrain-simple.js): code-driven effects instead of the tile
// art's four-frame sprite. Each realm has a faceted crystal of its own shape and colour that turns,
// bobs and pulses, a soft glow behind it, and motes drifting up:
//
//   Sorcery: a blue octahedron floating over the ground, with small shards orbiting it
//   Nature:  a cluster of green crystals growing from the ground
//   Chaos:   a jagged red crystal spinning over its peak, embers rising
//
// No baked animation: animate() moves the crystals, and the motes rise in their shader. Nodes on
// explored land the player doesn't see now are dimmed and still.

import * as THREE from "three";
import { hash } from "./terrain-simple.js";

const realms = {
    "Sorcery Node": { colour: 0x58b4ff, glow: 0x3c8cff, mote: 0x9ad4ff, hover: 0.4, spin: 0.6 },
    "Nature Node": { colour: 0x58e070, glow: 0x3cc850, mote: 0xb8ff9a, hover: 0.0, spin: 0.15 },
    "Chaos Node": { colour: 0xff5a28, glow: 0xff3c14, mote: 0xffb040, hover: 0.3, spin: 1.4 },
};
// motes per node, how high they rise (tiles) and their size (tiles)
const moteCount = 28;
const moteRise = 0.6;
const moteSize = 0.055;
const dimColour = 0.55;

// the crystals, in tiles, centred on the origin (Nature's stand on y = 0)
function makeCrystals() {
    const sorcery = new THREE.OctahedronGeometry(0.14, 0).scale(1, 1.7, 1);
    const shard = new THREE.OctahedronGeometry(0.04, 0).scale(1, 1.6, 1);
    const nature = [];
    for (let index = 0; index < 5; index++) {
        const height = 0.24 + (index % 3) * 0.08;
        const crystal = new THREE.CylinderGeometry(0, 0.055, height, 5).translate(0, height / 2, 0);
        crystal.rotateZ((index - 2) * 0.22);
        crystal.rotateY(index * 1.3);
        crystal.translate(Math.cos(index * 1.3) * 0.08, 0, Math.sin(index * 1.3) * 0.08);
        nature.push(crystal.toNonIndexed());
    }
    const chaos = new THREE.TetrahedronGeometry(0.17, 1);
    // jagged: push some vertices out
    const positions = chaos.attributes.position;
    for (let index = 0; index < positions.count; index++) {
        const scale = 0.8 + hash(index * 7 + 3, index * 13 + 1) * 0.6;
        positions.setXYZ(index, positions.getX(index) * scale, positions.getY(index) * scale * 1.3, positions.getZ(index) * scale);
    }
    return { sorcery, shard, nature: mergePositions(nature), chaos };
}

function mergePositions(geometries) {
    const total = geometries.reduce((sum, geometry) => sum + geometry.attributes.position.array.length, 0);
    const positions = new Float32Array(total);
    let offset = 0;
    for (const geometry of geometries) {
        positions.set(geometry.attributes.position.array, offset);
        offset += geometry.attributes.position.array.length;
    }
    const merged = new THREE.BufferGeometry();
    merged.setAttribute("position", new THREE.BufferAttribute(positions, 3));
    return merged;
}

const crystalVertexShader = `
varying vec3 worldPosition;
void main() {
    vec4 world = modelMatrix * vec4(position, 1.0);
    worldPosition = world.xyz;
    gl_Position = projectionMatrix * viewMatrix * world;
}
`;

const crystalFragmentShader = `
uniform vec3 colour;
uniform float pulse;
uniform vec3 lightDirection;
uniform float ambient;
varying vec3 worldPosition;
void main() {
    vec3 normal = normalize(cross(dFdx(worldPosition), dFdy(worldPosition)));
    float light = ambient + (1.0 - ambient) * max(0.0, dot(normal, lightDirection));
    // lit facets plus a glow from within
    gl_FragColor = vec4(colour * (0.55 * light + 0.75 + 0.7 * pulse), 1.0);
    #include <colorspace_fragment>
}
`;

const moteVertexShader = `
attribute float phase;
attribute float speed;
uniform float time;
uniform float pixelsPerTile;
uniform float rise;
uniform float size;
varying float fade;
void main() {
    float t = fract(time * speed + phase);
    vec3 moved = position + vec3(sin(t * 6.2832 + phase * 20.0) * 0.03, t * rise, cos(t * 6.2832 + phase * 17.0) * 0.03);
    fade = sin(t * 3.14159);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(moved, 1.0);
    gl_PointSize = max(1.0, size * pixelsPerTile * (0.6 + 0.4 * fade));
}
`;

const moteFragmentShader = `
uniform vec3 colour;
varying float fade;
void main() {
    vec2 offset = gl_PointCoord - 0.5;
    if (dot(offset, offset) > 0.25) discard;
    gl_FragColor = vec4(colour * fade, 1.0);
    #include <colorspace_fragment>
}
`;

// a soft round glow (additive)
function glowTexture() {
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 64;
    const context = canvas.getContext("2d");
    const gradient = context.createRadialGradient(32, 32, 0, 32, 32, 32);
    gradient.addColorStop(0, "rgba(255,255,255,1)");
    gradient.addColorStop(0.35, "rgba(255,255,255,0.35)");
    gradient.addColorStop(1, "rgba(255,255,255,0)");
    context.fillStyle = gradient;
    context.fillRect(0, 0, 64, 64);
    const texture = new THREE.CanvasTexture(canvas);
    texture.colorSpace = THREE.SRGBColorSpace;
    return texture;
}

export class NodeEffects {
    constructor(lightDirection, ambient) {
        this.lightDirection = lightDirection;
        this.ambient = ambient;
        this.crystals = makeCrystals();
        this.glow = glowTexture();
        // the nodes shown: per node and copy of the map, its parts to animate
        this.nodes = [];
        this.objects = [];
        this.geometries = [];
        // per realm and dimming: materials (shared by the nodes, so one pulse uniform each)
        this.materials = new Map();
        this.time = { value: 0 };
        this.pixelsPerTile = { value: 20 };
    }

    materialsFor(terrain, dimmed) {
        const key = `${terrain}/${dimmed}`;
        let materials = this.materials.get(key);
        if (!materials) {
            const realm = realms[terrain];
            const tone = (hex) => new THREE.Color(hex).multiplyScalar(dimmed ? dimColour : 1);
            materials = {
                crystal: new THREE.ShaderMaterial({
                    uniforms: {
                        colour: { value: tone(realm.colour) },
                        pulse: { value: 0 },
                        lightDirection: { value: this.lightDirection.clone() },
                        ambient: { value: this.ambient },
                    },
                    vertexShader: crystalVertexShader,
                    fragmentShader: crystalFragmentShader,
                }),
                glow: new THREE.SpriteMaterial({
                    map: this.glow, color: tone(realm.glow), blending: THREE.AdditiveBlending,
                    transparent: true, depthWrite: false, opacity: dimmed ? 0.3 : 1,
                }),
                // a wide faint halo around the glow, pulsing (animate)
                halo: new THREE.SpriteMaterial({
                    map: this.glow, color: tone(realm.glow), blending: THREE.AdditiveBlending,
                    transparent: true, depthWrite: false, opacity: dimmed ? 0.15 : 0.5,
                }),
                mote: new THREE.ShaderMaterial({
                    uniforms: {
                        colour: { value: tone(realm.mote) },
                        time: this.time,
                        pixelsPerTile: this.pixelsPerTile,
                        rise: { value: moteRise },
                        size: { value: moteSize },
                    },
                    vertexShader: moteVertexShader,
                    fragmentShader: moteFragmentShader,
                    blending: THREE.AdditiveBlending,
                    transparent: true,
                    depthWrite: false,
                }),
                dimmed,
                realm,
            };
            this.materials.set(key, materials);
        }
        return materials;
    }

    // motes rising from around a node's centre
    moteGeometry(x, y, groundAt) {
        const positions = new Float32Array(moteCount * 3);
        const phases = new Float32Array(moteCount);
        const speeds = new Float32Array(moteCount);
        for (let index = 0; index < moteCount; index++) {
            const angle = hash(x * 31 + index, y * 17 + 3) * Math.PI * 2;
            const radius = 0.08 + hash(x * 5 + index * 3, y * 7 + index) * 0.28;
            const px = x + 0.5 + Math.cos(angle) * radius;
            const pz = y + 0.5 + Math.sin(angle) * radius;
            positions.set([px, groundAt(px, pz) + 0.02, pz], index * 3);
            phases[index] = hash(x * 13 + index * 11, y * 29 + index);
            speeds[index] = 0.25 + hash(x * 3 + index * 19, y * 23 + 5) * 0.3;
        }
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute("phase", new THREE.BufferAttribute(phases, 1));
        geometry.setAttribute("speed", new THREE.BufferAttribute(speeds, 1));
        return geometry;
    }

    // show the nodes ([{terrain, x, y, dimmed}]) on each copy of the map (groups); groundAt(x, z)
    // gives the ground's height
    update(nodes, groups, groundAt) {
        this.remove();
        for (const node of nodes) {
            const materials = this.materialsFor(node.terrain, node.dimmed);
            const realm = materials.realm;
            const centreX = node.x + 0.5;
            const centreZ = node.y + 0.5;
            const ground = groundAt(centreX, centreZ);
            const motes = this.moteGeometry(node.x, node.y, groundAt);
            this.geometries.push(motes);
            for (const group of groups) {
                const parts = { node, realm, base: ground + realm.hover, spinners: [], orbiters: [] };
                const holder = new THREE.Group();
                holder.position.set(centreX, parts.base, centreZ);
                if (node.terrain === "Sorcery Node") {
                    const crystal = new THREE.Mesh(this.crystals.sorcery, materials.crystal);
                    holder.add(crystal);
                    parts.spinners.push(crystal);
                    for (let index = 0; index < 3; index++) {
                        const shard = new THREE.Mesh(this.crystals.shard, materials.crystal);
                        holder.add(shard);
                        parts.orbiters.push({ mesh: shard, angle: index * Math.PI * 2 / 3, radius: 0.26, height: (index - 1) * 0.06 });
                    }
                } else if (node.terrain === "Nature Node") {
                    const crystal = new THREE.Mesh(this.crystals.nature, materials.crystal);
                    holder.add(crystal);
                    parts.spinners.push(crystal);
                } else {
                    const crystal = new THREE.Mesh(this.crystals.chaos, materials.crystal);
                    holder.add(crystal);
                    parts.spinners.push(crystal);
                }
                const glow = new THREE.Sprite(materials.glow);
                glow.scale.set(1.1, 1.1, 1);
                glow.position.set(0, node.terrain === "Nature Node" ? 0.15 : 0, 0);
                holder.add(glow);
                const halo = new THREE.Sprite(materials.halo);
                halo.scale.set(2, 2, 1);
                halo.position.copy(glow.position);
                holder.add(halo);
                group.add(holder);
                parts.holder = holder;
                this.objects.push(holder);
                if (!node.dimmed) {
                    const points = new THREE.Points(motes, materials.mote);
                    points.frustumCulled = false;
                    group.add(points);
                    this.objects.push(points);
                }
                this.nodes.push(parts);
            }
        }
    }

    // move the crystals and the motes: seconds since the page loaded; pixelsPerTile for the motes'
    // size on screen
    animate(seconds, pixelsPerTile) {
        this.time.value = seconds;
        this.pixelsPerTile.value = pixelsPerTile;
        const pulse = 0.5 + 0.5 * Math.sin(seconds * 2.2);
        for (const materials of this.materials.values()) {
            materials.crystal.uniforms.pulse.value = materials.dimmed ? 0 : pulse;
            if (!materials.dimmed) materials.halo.opacity = 0.3 + 0.4 * pulse;
        }
        for (const parts of this.nodes) {
            if (parts.node.dimmed) continue;
            const offset = hash(parts.node.x, parts.node.y) * 10;
            const hover = parts.realm.hover > 0 ? Math.sin(seconds * 1.6 + offset) * 0.03 : 0;
            parts.holder.position.y = parts.base + hover;
            for (const spinner of parts.spinners) {
                spinner.rotation.y = seconds * parts.realm.spin + offset;
            }
            for (const orbiter of parts.orbiters) {
                const angle = orbiter.angle + seconds * 1.2;
                orbiter.mesh.position.set(Math.cos(angle) * orbiter.radius, orbiter.height + Math.sin(seconds * 2 + orbiter.angle) * 0.02, Math.sin(angle) * orbiter.radius);
                orbiter.mesh.rotation.y = seconds * 2;
            }
        }
    }

    remove() {
        for (const object of this.objects) {
            if (object.parent) object.parent.remove(object);
        }
        this.objects = [];
        this.nodes = [];
        for (const geometry of this.geometries) geometry.dispose();
        this.geometries = [];
    }
}
