// The overland map in 3D: the terrain (painted as in the 2D map, with roads, bonuses, fog and
// paths) lies on a ground plane with a tile grid, seen through an isometric orthographic camera;
// cities, stacks, encounters (lairs, caves, towers) and nodes stand on their tiles as billboards.
// The ground has height: each terrain type has a level (water low and flat, hills and mountains
// up), smoothed between tiles with a little fixed noise, and shaded by slope (baked into vertex
// colours: the ground is unlit). Billboards stand at the ground's height; picking follows it.
//
// World units are tiles: tile (x, y) covers world x [x, x+1] and z [y, y+1] on the ground (y up).
// The map wraps east-west: three copies of the ground and its billboards sit side by side.

import * as THREE from "three";
import { MapRenderer, isStanding } from "./map.js";

// true isometric: the camera looks down 35.26° from the horizon, turned 45° (north-east is up)
const cameraOffset = new THREE.Vector3(-1, 1, 1).normalize().multiplyScalar(200);
const pitch = Math.asin(cameraOffset.y / cameraOffset.length());
// on the ground: the screen's right and up directions
const groundRight = new THREE.Vector3(1, 0, 1).normalize();
const groundUp = new THREE.Vector3(1, 0, -1).normalize();
// how dark billboards on explored land the player doesn't see now are drawn
const dimColor = 0.55;
// billboards' size against the ground: a sprite's pixels at this fraction of the terrain's (a
// 20-pixel stack is 0.65 tile wide, well inside its tile's diamond)
const billboardScale = 0.65;
// how far below the tile's centre (on screen) a billboard's foot stands, as a fraction of its
// height: its middle then sits over the tile, not on the corner behind it
const billboardFoot = 0.55;

// the ground's level per terrain, in tiles (unexplored land is flat at 0)
const terrainHeights = {
    "Ocean": -0.14, "Lake": -0.1, "Shore": -0.02, "River": 0.03, "Swamp": 0.03,
    "Grasslands": 0.07, "Desert": 0.08, "Tundra": 0.09, "Forest": 0.14,
    "Hills": 0.38, "Mountain": 0.8, "Volcano": 0.95,
    "Sorcery Node": 0.08, "Nature Node": 0.14, "Chaos Node": 0.8,
};
const waterTerrain = new Set(["Ocean", "Lake", "Shore"]);
// vertices per tile edge
const subdivisions = 2;
// a little fixed unevenness on land, in tiles
const noiseAmount = 0.03;
// where the light comes from (north, high up: the screen's upper left) and how strong the slope
// shading is
const lightDirection = new THREE.Vector3(0.15, 1, -0.75).normalize();
const shadeAmbient = 0.55;

// a fixed pseudo-random value in [0, 1) for a vertex
function hash(x, y) {
    let value = Math.imul(x, 374761393) + Math.imul(y, 668265263);
    value = Math.imul(value ^ (value >>> 13), 1274126177);
    return ((value ^ (value >>> 16)) >>> 0) / 4294967296;
}

export class MapRenderer3D extends MapRenderer {
    constructor(container, assets, callbacks) {
        super(container, assets, callbacks);
        this.drawsThings = false;
        this.zoomIndex = 2;
        // 1: the terrain's height; 0: flat (toggleHeight)
        this.heightScale = 1;
        this.raycaster = new THREE.Raycaster();
        this.groundPlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
        // billboard textures and materials by sprite (and dimming)
        this.spriteTextures = new Map();
        this.spriteMaterials = new Map();
        // per copy of the map: a group of billboards
        this.billboards = [];
        // the selected stack's billboards (they blink), and the animated ones
        this.selectedSprites = [];
        this.animatedSprites = [];
        // the tile grid, painted on the ground (it scales with the zoom like the terrain)
        this.showGrid = true;
        this.updateCamera();
    }

    // the point on the ground the camera looks at
    get focus() {
        if (!this.focusPoint) {
            this.focusPoint = new THREE.Vector3(0, 0, 0);
        }
        return this.focusPoint;
    }

    get mapWidth() {
        return this.mapView ? this.mapView.width : 1;
    }

    // ground planes (the grid is painted on the ground texture: see MapRenderer.showGrid)
    buildPlanes() {
        for (const copy of this.copies) {
            this.scene.remove(copy);
        }
        this.canvas.width = this.pixelWidth;
        this.canvas.height = this.pixelHeight;
        this.texture.dispose();
        this.texture = new THREE.CanvasTexture(this.canvas);
        this.texture.magFilter = THREE.NearestFilter;
        this.texture.minFilter = THREE.NearestFilter;
        this.texture.generateMipmaps = false;
        this.texture.colorSpace = THREE.SRGBColorSpace;
        this.material.map = this.texture;

        const width = this.mapView.width;
        const height = this.mapView.height;
        this.ground = this.makeGround(width, height);
        this.material.vertexColors = true;
        this.material.needsUpdate = true;

        this.copies = [-1, 0, 1].map((offset) => {
            const copy = new THREE.Group();
            copy.position.set(offset * width, 0, 0);
            copy.add(new THREE.Mesh(this.ground, this.material));
            this.scene.add(copy);
            return copy;
        });
        this.billboards = [];
        this.terrainKey = null;
    }

    // a grid of vertices over the map (subdivisions per tile), with the texture's coordinates;
    // heights and shading come from updateHeights
    makeGround(width, height) {
        const columns = width * subdivisions + 1;
        const rows = height * subdivisions + 1;
        const positions = new Float32Array(columns * rows * 3);
        const uvs = new Float32Array(columns * rows * 2);
        const colors = new Float32Array(columns * rows * 3).fill(1);
        for (let row = 0; row < rows; row++) {
            for (let column = 0; column < columns; column++) {
                const index = row * columns + column;
                positions[index * 3] = column / subdivisions;
                positions[index * 3 + 2] = row / subdivisions;
                uvs[index * 2] = column / (columns - 1);
                uvs[index * 2 + 1] = 1 - row / (rows - 1);
            }
        }
        const indices = [];
        for (let row = 0; row < rows - 1; row++) {
            for (let column = 0; column < columns - 1; column++) {
                const a = row * columns + column;
                const b = a + 1;
                const c = a + columns;
                const d = c + 1;
                indices.push(a, c, b, b, c, d);
            }
        }
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute("uv", new THREE.BufferAttribute(uvs, 2));
        geometry.setAttribute("color", new THREE.BufferAttribute(colors, 3));
        geometry.setIndex(indices);
        geometry.userData = { columns, rows };
        this.heights = new Float32Array(columns * rows);
        return geometry;
    }

    // the vertex heights from the explored terrain (smoothed, with noise on land) and the slope
    // shading; only when the terrain changed (exploring, Change Terrain...)
    updateHeights() {
        const mapView = this.mapView;
        const key = mapView.tiles.map((row) => row.map((tile) => (tile ? tile.terrain : "")).join(",")).join(";");
        if (key === this.terrainKey) {
            return;
        }
        this.terrainKey = key;
        const width = mapView.width;
        const height = mapView.height;
        const scale = this.heightScale;
        const terrainAt = (x, y) => {
            const row = mapView.tiles[Math.max(0, Math.min(height - 1, y))];
            const tile = row[((x % width) + width) % width];
            return tile ? tile.terrain : null;
        };
        const levelAt = (x, y) => {
            const terrain = terrainAt(x, y);
            return terrain === null ? 0 : (terrainHeights[terrain] ?? 0.07);
        };

        // a tile's level, softened by its neighbours (a lone mountain doesn't spike)
        const smoothed = new Float32Array(width * height);
        for (let y = 0; y < height; y++) {
            for (let x = 0; x < width; x++) {
                const own = levelAt(x, y);
                let neighbours = 0;
                for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
                    neighbours += levelAt(x + dx, y + dy);
                }
                smoothed[y * width + x] = waterTerrain.has(terrainAt(x, y)) ? own : own * 0.7 + neighbours / 4 * 0.3;
            }
        }
        const tileLevel = (x, y) => smoothed[Math.max(0, Math.min(height - 1, y)) * width + (((x % width) + width) % width)];

        // each vertex: between the centres of the four tiles around it (bilinear), plus noise on land
        const { columns, rows } = this.ground.userData;
        const heights = this.heights;
        for (let row = 0; row < rows; row++) {
            for (let column = 0; column < columns; column++) {
                const worldX = column / subdivisions - 0.5;
                const worldZ = row / subdivisions - 0.5;
                const x0 = Math.floor(worldX);
                const z0 = Math.floor(worldZ);
                const fx = worldX - x0;
                const fz = worldZ - z0;
                const level = tileLevel(x0, z0) * (1 - fx) * (1 - fz) + tileLevel(x0 + 1, z0) * fx * (1 - fz)
                    + tileLevel(x0, z0 + 1) * (1 - fx) * fz + tileLevel(x0 + 1, z0 + 1) * fx * fz;
                // the map wraps: the last column's noise is the first's
                const noise = level > 0 ? (hash(column % (columns - 1), row) - 0.5) * noiseAmount : 0;
                heights[row * columns + column] = (level + noise) * scale;
            }
        }

        // positions, and shading by the slope facing the light (flat ground stays as painted)
        const positions = this.ground.attributes.position.array;
        const colors = this.ground.attributes.color.array;
        const step = 1 / subdivisions;
        const flat = lightDirection.y;
        const normal = new THREE.Vector3();
        const at = (column, row) => heights[Math.max(0, Math.min(rows - 1, row)) * columns + (((column % (columns - 1)) + (columns - 1)) % (columns - 1))];
        for (let row = 0; row < rows; row++) {
            for (let column = 0; column < columns; column++) {
                const index = row * columns + column;
                positions[index * 3 + 1] = heights[index];
                normal.set(at(column - 1, row) - at(column + 1, row), 2 * step, at(column, row - 1) - at(column, row + 1)).normalize();
                const light = shadeAmbient + (1 - shadeAmbient) * Math.max(0, normal.dot(lightDirection)) / flat;
                colors[index * 3] = colors[index * 3 + 1] = colors[index * 3 + 2] = light;
            }
        }
        this.ground.attributes.position.needsUpdate = true;
        this.ground.attributes.color.needsUpdate = true;
        this.ground.computeBoundingSphere();
        this.ground.computeBoundingBox();
    }

    // the ground's height at a world point (bilinear between the vertices; wraps east-west)
    heightAt(worldX, worldZ) {
        if (!this.ground) {
            return 0;
        }
        const { columns, rows } = this.ground.userData;
        const span = columns - 1;
        const gx = worldX * subdivisions;
        const gz = Math.max(0, Math.min(rows - 1.001, worldZ * subdivisions));
        const x0 = Math.floor(gx);
        const z0 = Math.floor(gz);
        const fx = gx - x0;
        const fz = gz - z0;
        const at = (column, row) => this.heights[row * columns + (((column % span) + span) % span)];
        return at(x0, z0) * (1 - fx) * (1 - fz) + at(x0 + 1, z0) * fx * (1 - fz) + at(x0, z0 + 1) * (1 - fx) * fz + at(x0 + 1, z0 + 1) * fx * fz;
    }

    // flatten the ground or give it back its height (the H key)
    toggleHeight() {
        this.heightScale = this.heightScale > 0 ? 0 : 1;
        this.terrainKey = null;
        if (this.mapView) {
            this.updateHeights();
            this.buildBillboards();
        }
    }

    setData(mapView, playerView) {
        super.setData(mapView, playerView);
        this.updateHeights();
        this.buildBillboards();
    }

    setSelected(location) {
        super.setSelected(location);
        this.buildBillboards();
    }

    // a texture of a sprite (frame 0); a stack's is its banner square with the leader on it
    spriteTexture(key, paint) {
        let texture = this.spriteTextures.get(key);
        if (!texture) {
            const canvas = document.createElement("canvas");
            const size = paint(null);
            canvas.width = Math.max(1, size[0]);
            canvas.height = Math.max(1, size[1]);
            paint(canvas.getContext("2d"));
            texture = new THREE.CanvasTexture(canvas);
            texture.magFilter = THREE.NearestFilter;
            texture.minFilter = THREE.NearestFilter;
            texture.generateMipmaps = false;
            texture.colorSpace = THREE.SRGBColorSpace;
            // the rows with pixels: sprites stand on their lowest one (city art has empty rows)
            const pixels = canvas.getContext("2d").getImageData(0, 0, canvas.width, canvas.height).data;
            const rowFilled = (row) => {
                for (let column = 0; column < canvas.width; column++) {
                    if (pixels[(row * canvas.width + column) * 4 + 3] > 0) return true;
                }
                return false;
            };
            let top = 0;
            while (top < canvas.height - 1 && !rowFilled(top)) top++;
            let bottom = canvas.height - 1;
            while (bottom > top && !rowFilled(bottom)) bottom--;
            texture.userData = { width: canvas.width, height: canvas.height, top, bottom };
            this.spriteTextures.set(key, texture);
        }
        return texture;
    }

    spriteMaterial(texture, dimmed) {
        const key = `${texture.id}/${dimmed}`;
        let material = this.spriteMaterials.get(key);
        if (!material) {
            // the pixel art's alpha is all or nothing: alpha test instead of blending, no sorting
            material = new THREE.SpriteMaterial({ map: texture, alphaTest: 0.5, transparent: false });
            if (dimmed) material.color.setScalar(dimColor);
            this.spriteMaterials.set(key, material);
        }
        return material;
    }

    // a billboard standing on its tile (its middle over the tile's centre), smaller than the ground's
// pixels (billboardScale); an
    // animated sprite keeps a material per frame and steps through them (afterDraw)
    billboard(textures, location, dimmed) {
        const texture = textures[0];
        const materials = textures.map((frame) => this.spriteMaterial(frame, dimmed));
        const sprite = new THREE.Sprite(materials[this.frame % materials.length]);
        if (materials.length > 1) {
            sprite.userData.frames = materials;
            this.animatedSprites.push(sprite);
        }
        const { width, height, top, bottom } = texture.userData;
        // the foot is the lowest row with pixels
        sprite.center.set(0.5, (height - 1 - bottom) / height);
        const scale = billboardScale / this.tileWidth;
        sprite.scale.set(width * scale, height * scale, 1);
        // the foot moves towards the camera (down the screen) from the tile's centre, by a part
        // of the drawn height; going down the screen by d covers d / sin(pitch) of ground. It
        // stays inside the tile's diamond (its front corner is 0.71 away).
        const drawn = (bottom - top + 1) * scale;
        const toward = Math.min(0.6, drawn * billboardFoot / Math.sin(pitch));
        const worldX = location.x + 0.5 - groundUp.x * toward;
        const worldZ = location.y + 0.5 - groundUp.z * toward;
        sprite.position.set(worldX, this.heightAt(worldX, worldZ), worldZ);
        sprite.userData.tile = { plane: this.plane, x: location.x, y: location.y };
        return sprite;
    }

    // what stands on the map: nodes, encounters, cities and stacks (as the 2D map shows them)
    buildBillboards() {
        if (!this.mapView || this.copies.length === 0) {
            return;
        }
        const assets = this.assets;
        const things = [];
        const visible = (location) => {
            const row = this.mapView.tiles[location.y];
            const tile = row ? row[location.x] : null;
            return tile ? tile.visible : false;
        };
        // a sprite's frames as textures (one for still sprites; nodes have four)
        const plain = (id) => {
            const count = Math.max(1, (assets.frames(id) || []).length);
            return Array.from({ length: count }, (_, frame) => this.spriteTexture(`${id}#${frame}`, (context) => {
                if (!context) return assets.size(id);
                assets.draw(context, id, 0, 0, frame);
            }));
        };

        this.mapView.tiles.forEach((row, y) => {
            row.forEach((tile, x) => {
                if (!tile) return;
                // a node's tile stands up as well as lying on the ground
                if (tile.terrain && tile.terrain.endsWith("Node")) {
                    things.push({ texture: plain(tile.asset), location: { x, y }, dimmed: !tile.visible });
                }
                for (const overlay of tile.overlays || []) {
                    if (isStanding(overlay) && assets.has(overlay)) {
                        things.push({ texture: plain(overlay), location: { x, y }, dimmed: !tile.visible });
                    }
                }
            });
        });

        const view = this.playerView;
        this.selectedSprites = [];
        if (view) {
            const onPlane = (location) => location.plane === this.plane;
            const cities = [...view["foreign-cities"], ...view.cities].filter((city) => onPlane(city.location));
            const cityTiles = new Set(cities.map((city) => `${city.location.x}/${city.location.y}`));
            for (const city of cities) {
                if (city.asset && assets.has(city.asset)) {
                    things.push({ texture: plain(city.asset), location: city.location, dimmed: !visible(city.location) });
                }
            }
            const stackTexture = (background, unit) => [this.spriteTexture(`stack/${background}/${unit}`, (context) => {
                if (!context) return assets.size(background);
                assets.draw(context, background, 0, 0);
                assets.draw(context, unit, 1, 1);
            })];
            for (const stack of view["foreign-stacks"]) {
                if (onPlane(stack.location) && !cityTiles.has(`${stack.location.x}/${stack.location.y}`)) {
                    things.push({ texture: stackTexture(stack.background, stack.asset), location: stack.location, dimmed: !visible(stack.location) });
                }
            }
            const selected = this.selected;
            for (const stack of view.stacks) {
                if (!onPlane(stack.location) || stack.units.length === 0) continue;
                const isSelected = selected && selected.plane === stack.location.plane && selected.x === stack.location.x && selected.y === stack.location.y;
                if (!isSelected && cityTiles.has(`${stack.location.x}/${stack.location.y}`)) continue;
                // the selected stack stands in front of its city
                things.push({ texture: stackTexture(stack.background, stack.units[0].asset), location: stack.location, dimmed: false, selected: isSelected });
            }
        }

        for (const group of this.billboards) {
            group.parent.remove(group);
        }
        this.animatedSprites = [];
        this.occupiedTiles = new Set(things.map((thing) => `${thing.location.x}/${thing.location.y}`));
        this.billboards = this.copies.map((copy) => {
            const group = new THREE.Group();
            for (const thing of things) {
                const sprite = this.billboard(thing.texture, thing.location, thing.dimmed);
                if (thing.selected) {
                    // in front of a city on the same tile
                    sprite.position.x += 0.12;
                    sprite.position.z += 0.12;
                    this.selectedSprites.push(sprite);
                }
                group.add(sprite);
            }
            copy.add(group);
            return group;
        });
    }

    afterDraw() {
        // the selected stack blinks, like the original; animated sprites (nodes) step a frame
        for (const sprite of this.selectedSprites || []) {
            sprite.visible = this.frame % 2 === 0;
        }
        for (const sprite of this.animatedSprites || []) {
            const frames = sprite.userData.frames;
            sprite.material = frames[this.frame % frames.length];
        }
    }

    updateCamera() {
        // the view's size in tiles: the zoom is screen pixels per tile pixel, as the 2D map
        const scale = this.tileWidth * this.zoom;
        const width = this.container.clientWidth / scale;
        const height = this.container.clientHeight / scale;
        this.camera.left = -width / 2;
        this.camera.right = width / 2;
        this.camera.top = height / 2;
        this.camera.bottom = -height / 2;
        this.camera.near = 1;
        this.camera.far = 1000;
        this.camera.updateProjectionMatrix();
        this.clampCamera();
    }

    clampCamera() {
        const focus = this.focus;
        if (this.mapView) {
            const width = this.mapView.width;
            focus.x = ((focus.x % width) + width) % width;
            focus.z = Math.min(this.mapView.height, Math.max(0, focus.z));
        }
        this.camera.position.copy(focus).add(cameraOffset);
        this.camera.lookAt(focus);
        this.camera.updateMatrixWorld();
    }

    centerOn(x, y) {
        this.focus.set(x + 0.5, 0, y + 0.5);
        this.clampCamera();
    }

    panBy(dx, dy) {
        // screen pixels to world units; going up the screen covers more ground (the tilt)
        const perPixel = 1 / (this.tileWidth * this.zoom);
        this.focus.addScaledVector(groundRight, -dx * perPixel);
        this.focus.addScaledVector(groundUp, dy * perPixel / Math.sin(pitch));
        this.clampCamera();
    }

    // the ground point under a point of the page, or null
    groundAt(clientX, clientY) {
        const bounds = this.renderer.domElement.getBoundingClientRect();
        const ndc = new THREE.Vector2(((clientX - bounds.left) / bounds.width) * 2 - 1, -((clientY - bounds.top) / bounds.height) * 2 + 1);
        this.camera.updateMatrixWorld();
        this.raycaster.setFromCamera(ndc, this.camera);
        return { ndc, point: this.groundHit(this.raycaster.ray) };
    }

    // where a ray from the camera meets the ground: walk from above the highest ground down to
    // below the lowest, then narrow the crossing down
    groundHit(ray) {
        const top = new THREE.Plane(new THREE.Vector3(0, 1, 0), -1.2 * Math.max(1, this.heightScale));
        const bottom = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0.3);
        const start = ray.intersectPlane(top, new THREE.Vector3());
        const end = ray.intersectPlane(bottom, new THREE.Vector3());
        if (!start || !end) {
            return ray.intersectPlane(this.groundPlane, new THREE.Vector3());
        }
        const above = (point) => point.y - this.heightAt(point.x, point.z);
        const point = new THREE.Vector3();
        let low = 0;
        let high = 1;
        const steps = 48;
        for (let index = 1; index <= steps; index++) {
            const t = index / steps;
            point.lerpVectors(start, end, t);
            if (above(point) <= 0) {
                low = (index - 1) / steps;
                high = t;
                break;
            }
        }
        for (let index = 0; index < 16; index++) {
            const middle = (low + high) / 2;
            point.lerpVectors(start, end, middle);
            if (above(point) > 0) low = middle; else high = middle;
        }
        return point.lerpVectors(start, end, (low + high) / 2).clone();
    }

    // the tile under a point of the page: a billboard's, else the ground's (see below)
    tileAt(clientX, clientY) {
        if (!this.mapView) {
            return null;
        }
        const { point } = this.groundAt(clientX, clientY);
        const hits = this.raycaster.intersectObjects(this.billboards, true).filter((hit) => hit.object.visible);
        let ground = null;
        if (point) {
            const width = this.mapView.width;
            const y = Math.floor(point.z);
            if (y >= 0 && y < this.mapView.height) {
                ground = { plane: this.plane, x: ((Math.floor(point.x) % width) + width) % width, y };
            }
        }
        // a tile with something standing on it wins when the pointer is on its ground, over a
        // taller thing in front whose sprite reaches over it; otherwise the frontmost billboard,
        // then the ground
        if (ground && this.occupiedTiles && this.occupiedTiles.has(`${ground.x}/${ground.y}`)) {
            return ground;
        }
        if (hits.length > 0) {
            return { ...hits[0].object.userData.tile };
        }
        return ground;
    }

    // a tile's centre on the ground in page coordinates (the copy nearest the view)
    screenPoint(x, y) {
        const width = this.mapWidth;
        let worldX = x + 0.5;
        worldX += Math.round((this.focus.x - worldX) / width) * width;
        const point = new THREE.Vector3(worldX, this.heightAt(worldX, y + 0.5), y + 0.5);
        this.camera.updateMatrixWorld();
        point.project(this.camera);
        const bounds = this.renderer.domElement.getBoundingClientRect();
        return [bounds.left + (point.x + 1) / 2 * bounds.width, bounds.top + (1 - point.y) / 2 * bounds.height];
    }

    // the ground in view, as the bounding box of the screen's corners on the ground (for the minimap)
    viewRect() {
        const bounds = this.renderer.domElement.getBoundingClientRect();
        const corners = [[bounds.left, bounds.top], [bounds.right, bounds.top], [bounds.left, bounds.bottom], [bounds.right, bounds.bottom]]
            .map(([x, y]) => this.groundAt(x, y).point)
            .filter(Boolean);
        if (corners.length === 0) {
            return { x: this.focus.x, y: this.focus.z, width: 0, height: 0 };
        }
        const xs = corners.map((point) => point.x);
        const zs = corners.map((point) => point.z);
        const left = Math.min(...xs);
        const top = Math.min(...zs);
        return { x: left, y: top, width: Math.max(...xs) - left, height: Math.max(...zs) - top };
    }
}
