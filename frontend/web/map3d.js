// The overland map in 3D: the terrain (painted as in the 2D map, with roads, bonuses, fog and
// paths) lies on a ground plane with a tile grid, seen through an isometric orthographic camera;
// cities, stacks, encounters (lairs, caves, towers) and nodes stand on their tiles as billboards.
// The ground has height: each terrain type has a level (water low and flat, hills and mountains
// up), smoothed between tiles with a little fixed noise, and shaded by slope (baked into vertex
// colours: the ground is unlit). Billboards stand at the ground's height; picking follows it.
//
// World units are tiles: tile (x, y) covers world x [x, x+1] and z [y, y+1] on the ground (y up).
// The map wraps east-west: three copies of the ground and its billboards sit side by side.
//
// Three looks for the ground (setTerrainLook, ?terrain=simple|polygon, the V key cycles):
// "textured", the original's tile art draped over the heights; "simple", plain colours per terrain
// with shapes and a water plane (terrain-simple.js), minerals standing up as billboards; "polygon",
// the simple ground styled as flat-filled polygons (one colour per face from a small palette,
// light in a few tones, a sky behind the map).

import * as THREE from "three";
import { MapRenderer, isStanding } from "./map.js";
import { SimpleGround, hash, stylizedAmbient } from "./terrain-simple.js";
import { Vegetation } from "./vegetation.js";
import { NodeEffects } from "./nodes3d.js";

// true isometric: the camera looks down 35.26° from the horizon, turned 45° (north-east is up);
// the view turns around the vertical axis in quarter turns (rotateView), these being for turn 0
const baseCameraOffset = new THREE.Vector3(-1, 1, 1).normalize().multiplyScalar(200);
const pitch = Math.asin(baseCameraOffset.y / baseCameraOffset.length());
// on the ground: the screen's right and up directions
const baseGroundRight = new THREE.Vector3(1, 0, 1).normalize();
const baseGroundUp = new THREE.Vector3(1, 0, -1).normalize();
const verticalAxis = new THREE.Vector3(0, 1, 0);
// where standingHeight looks around a billboard's foot, in tiles
const footSamples = [[0.08, 0], [-0.08, 0], [0, 0.08], [0, -0.08], [0.06, 0.06], [-0.06, -0.06], [0.06, -0.06], [-0.06, 0.06]];
// how long a quarter turn of the view takes, in milliseconds
const turnMilliseconds = 250;
// how dark billboards on explored land the player doesn't see now are drawn
const dimColor = 0.55;
// billboards' size against the ground: a sprite's pixels at this fraction of the terrain's (a
// 20-pixel stack is 0.65 tile wide, well inside its tile's diamond)
const billboardScale = 0.65;
// how long a moving stack takes per tile, in milliseconds
const stepMilliseconds = 220;
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
        // stacks walking: the walk being played (its sprites, one per copy of the map) and those
        // waiting; their destinations' billboards stay hidden until they arrive
        this.walk = null;
        this.walks = [];
        // the tile grid, painted on the ground (it scales with the zoom like the terrain)
        this.showGrid = true;
        // the simple ground (plain colours, a water plane) instead of the tile art: setTerrainLook
        this.simple = false;
        // the polygon look (on the simple ground) and its sky
        this.stylized = false;
        this.backdrop = null;
        this.backdropKey = null;
        this.simpleGround = null;
        // trees and shrubs on the simple ground
        this.vegetation = null;
        // magic nodes' crystals and motes on the simple ground
        this.nodeEffects = null;
        // the view's turn around the vertical axis, in radians: where it is and where it's going
        // (quarter turns: rotateView)
        this.rotation = 0;
        this.targetRotation = 0;
        this.turnTime = null;
        this.updateCamera();
    }

    // the camera's offset from its focus and the screen's right and up on the ground, for the
    // view's turn
    get cameraOffset() {
        return baseCameraOffset.clone().applyAxisAngle(verticalAxis, this.rotation || 0);
    }

    get groundRight() {
        return baseGroundRight.clone().applyAxisAngle(verticalAxis, this.rotation || 0);
    }

    get groundUp() {
        return baseGroundUp.clone().applyAxisAngle(verticalAxis, this.rotation || 0);
    }

    // where a mineral stands on a tile with a city or lair: aside, to the right on screen
    get mineralAside() {
        const right = this.groundRight;
        return { x: right.x * 0.3, z: right.z * 0.3 };
    }

    // turn the view a quarter turn around the point it looks at: direction 1 or -1 (the Q and E
    // keys); it turns smoothly (beforeRender)
    rotateView(direction) {
        this.targetRotation += direction * Math.PI / 2;
    }

    // the turn done: billboards stand towards the camera again, and the simple ground's ledges
    // follow them
    finishTurn() {
        // keep the angle within one full turn (the same view)
        const turns = Math.round(this.targetRotation / (Math.PI / 2));
        this.targetRotation = (((turns % 4) + 4) % 4) * Math.PI / 2;
        this.rotation = this.targetRotation;
        if (this.mapView) {
            if (this.simple) {
                this.terrainKey = null;
                this.updateHeights();
            }
            this.buildBillboards();
        }
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
        if (this.simple) {
            this.buildSimplePlanes(width, height);
            return;
        }
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

    // the tiles with a lair or city on them ("x/y"): things that don't move
    settledTiles() {
        const tiles = new Set();
        this.mapView.tiles.forEach((row, y) => {
            row.forEach((tile, x) => {
                if (tile && (tile.overlays || []).some(isStanding)) tiles.add(`${x}/${y}`);
            });
        });
        const view = this.playerView;
        if (view) {
            for (const city of [...view["foreign-cities"], ...view.cities]) {
                if (city.location.plane === this.plane) tiles.add(`${city.location.x}/${city.location.y}`);
            }
        }
        return tiles;
    }

    // where lairs, cities and minerals stand on the simple ground: "x/y" -> their billboards'
    // feet from the tile's corner (hills and mountains flatten a ledge there)
    sites() {
        const sites = new Map();
        const settled = this.settledTiles();
        const add = (id, x, y, offset) => {
            if (!this.assets.has(id)) return;
            const toward = this.footToward(this.stillTexture(id));
            const key = `${x}/${y}`;
            if (!sites.has(key)) sites.set(key, []);
            sites.get(key).push({ x: 0.5 - this.groundUp.x * toward + offset.x, z: 0.5 - this.groundUp.z * toward + offset.z });
        };
        this.mapView.tiles.forEach((row, y) => {
            row.forEach((tile, x) => {
                for (const overlay of (tile && tile.overlays) || []) {
                    if (isStanding(overlay)) add(overlay, x, y, { x: 0, z: 0 });
                    else if (this.standsUp(overlay)) add(overlay, x, y, settled.has(`${x}/${y}`) ? this.mineralAside : { x: 0, z: 0 });
                }
            });
        });
        const view = this.playerView;
        if (view) {
            for (const city of [...view["foreign-cities"], ...view.cities]) {
                if (city.location.plane === this.plane && city.asset) add(city.asset, city.location.x, city.location.y, { x: 0, z: 0 });
            }
        }
        return sites;
    }

    // a sprite's first frame as a texture
    stillTexture(id) {
        return this.spriteTexture(`${id}#0`, (context) => {
            if (!context) return this.assets.size(id);
            this.assets.draw(context, id, 0, 0, 0);
        });
    }

    // the simple ground: per copy, the ground mesh (its geometry comes from updateHeights) and the
    // water plane
    buildSimplePlanes(width, height) {
        if (!this.simpleGround) {
            this.simpleGround = new SimpleGround(lightDirection, shadeAmbient);
        }
        this.simpleGround.setOverlay(this.texture);
        this.simpleGround.setStylized(this.stylized);
        this.ground = null;
        this.copies = [-1, 0, 1].map((offset) => {
            const copy = new THREE.Group();
            copy.position.set(offset * width, 0, 0);
            const { ground, water } = this.simpleGround.makeCopy(width, height);
            copy.add(ground, water);
            copy.userData.ground = ground;
            this.scene.add(copy);
            return copy;
        });
        this.billboards = [];
        this.terrainKey = null;
    }

    // switch the ground's look: "textured" (the tile art) or "simple" (plain colours, terrain-simple.js)
    setTerrainLook(look) {
        this.simple = look === "simple" || look === "polygon";
        this.stylized = look === "polygon";
        this.drawsTerrain = !this.simple;
        this.drawsFog = !this.stylized;
        this.drawsRoads = !this.stylized;
        if (this.mapView) {
            this.buildPlanes();
            if (this.simpleGround) this.simpleGround.setStylized(this.stylized);
            this.updateHeights();
            this.draw();
            this.buildBillboards();
        }
        this.updateBackdrop();
    }

    get terrainLook() {
        return this.stylized ? "polygon" : this.simple ? "simple" : "textured";
    }

    // the V key: textured, simple, polygon, and round again
    toggleTerrainLook() {
        const looks = ["textured", "simple", "polygon"];
        this.setTerrainLook(looks[(looks.indexOf(this.terrainLook) + 1) % looks.length]);
    }

    // behind the map: a sky in the polygon look (dusky blue over Arcanus, violet over Myrror),
    // black otherwise
    updateBackdrop() {
        if (!this.stylized) {
            this.scene.background = null;
            return;
        }
        const key = this.plane;
        if (this.backdropKey === key && this.backdropGround === this.simpleGround) return;
        this.backdropGround = this.simpleGround;
        this.backdropKey = key;
        const [top, bottom] = key === "Myrror" ? ["#140c22", "#3a2448"] : ["#0e1a30", "#34506a"];
        // unexplored land is painted in it
        if (this.simpleGround) this.simpleGround.setSky(top, bottom);
        const canvas = document.createElement("canvas");
        canvas.width = 2;
        canvas.height = 256;
        const context = canvas.getContext("2d");
        const gradient = context.createLinearGradient(0, 0, 0, 256);
        gradient.addColorStop(0, top);
        gradient.addColorStop(1, bottom);
        context.fillStyle = gradient;
        context.fillRect(0, 0, 2, 256);
        if (this.backdrop) this.backdrop.dispose();
        this.backdrop = new THREE.CanvasTexture(canvas);
        this.backdrop.colorSpace = THREE.SRGBColorSpace;
        this.scene.background = this.backdrop;
    }

    // minerals and other bonuses stand up on the simple ground (its colours have no art to show them)
    standsUp(overlay) {
        return isStanding(overlay) || (this.simple && overlay.startsWith("bonus/"));
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
    // the ground's shape, and the polygon look's roads on it
    updateHeights() {
        this.updateGround();
        this.updateRoads();
    }

    // the polygon look's road strips on each copy of the map (the canvas paints roads otherwise)
    updateRoads() {
        const wanted = this.simple && this.stylized && this.copies;
        const key = wanted ? `${this.terrainKey}|${this.heightScale}|` + this.mapView.tiles.map((row) => row.map((tile) => (tile ? (tile.overlays || []).filter((overlay) => overlay.startsWith("road")).join("+") : "")).join(",")).join(";") : null;
        if (key === this.roadKey && this.roadCopies === this.copies) return;
        this.roadKey = key;
        this.roadCopies = this.copies;
        if (this.roadGeometry) this.roadGeometry.dispose();
        this.roadGeometry = null;
        for (const copy of this.copies || []) {
            if (copy.userData.roads) copy.remove(copy.userData.roads);
            copy.userData.roads = null;
        }
        if (!wanted) return;
        this.roadGeometry = this.simpleGround.roadGeometry(this.mapView);
        if (!this.roadGeometry) return;
        for (const copy of this.copies) {
            copy.userData.roads = new THREE.Mesh(this.roadGeometry, this.simpleGround.roadMaterial);
            copy.add(copy.userData.roads);
        }
    }

    updateGround() {
        const mapView = this.mapView;
        let key = mapView.tiles.map((row) => row.map((tile) => (tile ? tile.terrain : "")).join(",")).join(";");
        if (this.simple) {
            key += [...this.sites()].map(([tile, feet]) => `${tile}@${feet.map((foot) => `${foot.x},${foot.z}`).join("+")}`).join(";");
        }
        // what's seen changes without the terrain changing
        if (this.simple) this.simpleGround.updateFog(mapView);
        if (key === this.terrainKey) {
            return;
        }
        this.terrainKey = key;
        if (this.simple) {
            const geometry = this.simpleGround.update(mapView, this.heightScale, this.sites());
            for (const copy of this.copies) {
                copy.userData.ground.geometry = geometry;
            }
            return;
        }
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
        if (this.simple && this.simpleGround) {
            return this.simpleGround.heightAt(worldX, worldZ);
        }
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
        this.updateBackdrop();
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

    // a billboard standing on its tile (its middle over the tile's centre), smaller than the
    // ground's pixels (billboardScale); in front: before anything else on the tile (the selected
    // stack in its city). An animated sprite keeps a material per frame and steps through them
    // (afterDraw)
    billboard(textures, location, dimmed, inFront = false, offset = null) {
        const texture = textures[0];
        const materials = textures.map((frame) => this.spriteMaterial(frame, dimmed));
        const sprite = new THREE.Sprite(materials[this.frame % materials.length]);
        if (materials.length > 1) {
            sprite.userData.frames = materials;
            this.animatedSprites.push(sprite);
        }
        const { width, height, bottom } = texture.userData;
        // the foot is the lowest row with pixels
        sprite.center.set(0.5, (height - 1 - bottom) / height);
        const scale = billboardScale / this.tileWidth;
        sprite.scale.set(width * scale, height * scale, 1);
        sprite.position.copy(this.footAt(texture, location.x + (offset ? offset.x : 0), location.y + (offset ? offset.z : 0), inFront));
        sprite.userData.tile = { plane: this.plane, x: location.x, y: location.y };
        return sprite;
    }

    // where a billboard of this texture stands on tile (x, y) (fractions for a stack between two
    // tiles): its foot moves towards the camera (down the screen) from the tile's centre, by a part
    // of the drawn height; going down the screen by d covers d / sin(pitch) of ground. It stays
    // inside the tile's diamond (its front corner is 0.71 away). In front: nearer the camera than
    // any other billboard's foot on the tile (at most 0.6), and a little up (nearer still)
    footAt(texture, x, y, inFront = false) {
        const toward = inFront ? 0.68 : this.footToward(texture);
        let worldX = x + 0.5 - this.groundUp.x * toward;
        let worldZ = y + 0.5 - this.groundUp.z * toward;
        // where the polygon look's bent mesh shows that point of the ground
        if (this.simple && this.simpleGround) {
            ({ x: worldX, z: worldZ } = this.simpleGround.warp(worldX, worldZ, { x: 0, z: 0 }));
        }
        return new THREE.Vector3(worldX, this.standingHeight(worldX, worldZ) + (inFront ? 0.05 : 0), worldZ);
    }

    // the height a billboard stands at: the highest ground right around its foot, so on a slope
    // it stands on it instead of sinking into the side rising behind it
    standingHeight(worldX, worldZ) {
        let height = this.heightAt(worldX, worldZ);
        for (const [dx, dz] of footSamples) {
            height = Math.max(height, this.heightAt(worldX + dx, worldZ + dz));
        }
        return height;
    }

    // how far towards the camera from the tile's centre a billboard of this texture stands
    footToward(texture) {
        const { top, bottom } = texture.userData;
        const drawn = (bottom - top + 1) * billboardScale / this.tileWidth;
        return Math.min(0.6, drawn * billboardFoot / Math.sin(pitch));
    }

    // a stack's texture: its banner square with the leader on it
    stackTextures(background, unit) {
        return [this.spriteTexture(`stack/${background}/${unit}`, (context) => {
            if (!context) return this.assets.size(background);
            this.assets.draw(context, background, 0, 0);
            this.assets.draw(context, unit, 1, 1);
        })];
    }

    // walks to play: [{plane, asset, background, steps: [{from, to}]}], one after the other
    playMoves(walks) {
        const shown = walks.filter((walk) => walk.plane === this.plane && walk.steps.length && this.assets.has(walk.background));
        if (shown.length === 0) return;
        this.walks.push(...shown);
        this.buildBillboards();
    }

    // the tiles whose stacks are still walking there (hidden until they arrive)
    get walkingTo() {
        const tiles = new Set();
        for (const walk of [this.walk, ...this.walks]) {
            if (!walk) continue;
            const last = walk.steps[walk.steps.length - 1].to;
            tiles.add(`${last.x}/${last.y}`);
        }
        return tiles;
    }

    beforeRender(now) {
        if (this.simpleGround) this.simpleGround.uniforms.skyHeight.value = this.renderer.domElement.height;
        if (this.nodeEffects) {
            this.nodeEffects.animate(now / 1000, this.tileWidth * this.zoom * this.renderer.getPixelRatio());
        }
        // the view turning
        if (this.rotation !== this.targetRotation) {
            const elapsed = this.turnTime === null ? 0 : now - this.turnTime;
            const step = (Math.PI / 2) * elapsed / turnMilliseconds;
            const left = this.targetRotation - this.rotation;
            this.turnTime = now;
            if (Math.abs(left) <= step) {
                this.turnTime = null;
                this.finishTurn();
            } else {
                this.rotation += Math.sign(left) * step;
            }
            this.clampCamera();
        }
        if (!this.walk && this.walks.length && this.copies.length) {
            const walk = this.walks.shift();
            walk.start = now;
            walk.texture = this.stackTextures(walk.background, walk.asset)[0];
            walk.sprites = this.copies.map((copy) => {
                const sprite = this.billboard([walk.texture], walk.steps[0].from, false);
                copy.add(sprite);
                return sprite;
            });
            this.walk = walk;
        }
        const walk = this.walk;
        if (!walk) return;
        const progress = (now - walk.start) / stepMilliseconds;
        const index = Math.floor(progress);
        if (index >= walk.steps.length) {
            for (const sprite of walk.sprites) sprite.parent.remove(sprite);
            this.walk = null;
            this.buildBillboards();
            return;
        }
        // a step across the map's east-west edge goes the short way
        const { from, to } = walk.steps[index];
        let toX = to.x;
        if (toX - from.x > this.mapWidth / 2) toX -= this.mapWidth;
        if (from.x - toX > this.mapWidth / 2) toX += this.mapWidth;
        const fraction = progress - index;
        const position = this.footAt(walk.texture, from.x + (toX - from.x) * fraction, from.y + (to.y - from.y) * fraction);
        for (const sprite of walk.sprites) sprite.position.copy(position);
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
                // a node's tile stands up as well as lying on the ground (the simple ground colours
                // nodes instead: their tile art has its own ground)
                if (!this.simple && tile.terrain && tile.terrain.endsWith("Node")) {
                    things.push({ texture: plain(tile.asset), location: { x, y }, dimmed: !tile.visible });
                }
                for (const overlay of tile.overlays || []) {
                    if (this.standsUp(overlay) && assets.has(overlay)) {
                        things.push({ texture: plain(overlay), location: { x, y }, dimmed: !tile.visible, bonus: !isStanding(overlay) });
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
            const stackTexture = (background, unit) => this.stackTextures(background, unit);
            // stacks still walking to their tile show up when they get there
            const walkingTo = this.walkingTo;
            for (const stack of view["foreign-stacks"]) {
                if (onPlane(stack.location) && !cityTiles.has(`${stack.location.x}/${stack.location.y}`) && !walkingTo.has(`${stack.location.x}/${stack.location.y}`)) {
                    things.push({ texture: stackTexture(stack.background, stack.asset), location: stack.location, dimmed: !visible(stack.location) });
                }
            }
            const selected = this.selected;
            for (const stack of view.stacks) {
                if (!onPlane(stack.location) || stack.units.length === 0 || walkingTo.has(`${stack.location.x}/${stack.location.y}`)) continue;
                const isSelected = selected && selected.plane === stack.location.plane && selected.x === stack.location.x && selected.y === stack.location.y;
                if (!isSelected && cityTiles.has(`${stack.location.x}/${stack.location.y}`)) continue;
                // the selected stack stands in front of its city
                things.push({ texture: stackTexture(stack.background, stack.units[0].asset), location: stack.location, dimmed: false, selected: isSelected, inCity: cityTiles.has(`${stack.location.x}/${stack.location.y}`) });
            }
        }

        for (const group of this.billboards) {
            group.parent.remove(group);
        }
        this.animatedSprites = [];
        this.occupiedTiles = new Set(things.map((thing) => `${thing.location.x}/${thing.location.y}`));
        // a mineral sharing its tile with a city or lair stands aside, to the right (not for
        // stacks: they come and go, and the simple ground's ledge under the mineral stays put)
        const crowded = this.settledTiles();
        const aside = this.mineralAside;
        this.billboards = this.copies.map((copy) => {
            const group = new THREE.Group();
            for (const thing of things) {
                // the selected stack stands in front of its city
                const offset = thing.bonus && crowded.has(`${thing.location.x}/${thing.location.y}`) ? aside : null;
                const sprite = this.billboard(thing.texture, thing.location, thing.dimmed, !!thing.inCity, offset);
                if (thing.selected) {
                    this.selectedSprites.push(sprite);
                }
                group.add(sprite);
            }
            copy.add(group);
            return group;
        });

        // trees and shrubs on the simple ground, out of the way of what stands on a tile
        if (this.simple && this.simpleGround && this.simpleGround.terrains) {
            if (!this.vegetation) this.vegetation = new Vegetation(lightDirection, shadeAmbient);
            this.vegetation.setStylized(this.stylized, stylizedAmbient);
            const feet = new Map();
            for (const thing of things) {
                const key = `${thing.location.x}/${thing.location.y}`;
                const offset = thing.bonus && crowded.has(key) ? aside : { x: 0, z: 0 };
                const toward = thing.inCity ? 0.68 : this.footToward(thing.texture[0]);
                if (!feet.has(key)) feet.set(key, []);
                feet.get(key).push({ x: 0.5 - this.groundUp.x * toward + offset.x, z: 0.5 - this.groundUp.z * toward + offset.z });
            }
            // no trees in the middle of a node (its crystals stand there)
            this.mapView.tiles.forEach((row, y) => row.forEach((tile, x) => {
                if (!tile || !tile.terrain || !tile.terrain.endsWith("Node")) return;
                const key = `${x}/${y}`;
                if (!feet.has(key)) feet.set(key, []);
                feet.get(key).push({ x: 0.5, z: 0.5 });
            }));
            this.vegetation.update(this.simpleGround, this.mapView, this.copies, feet);

            // magic nodes: crystals and motes (their tile art stands up only on the textured ground)
            if (!this.nodeEffects) this.nodeEffects = new NodeEffects(lightDirection, shadeAmbient);
            const nodes = [];
            this.mapView.tiles.forEach((row, y) => row.forEach((tile, x) => {
                if (tile && tile.terrain && tile.terrain.endsWith("Node")) nodes.push({ terrain: tile.terrain, x, y, dimmed: !tile.visible });
            }));
            this.nodeEffects.update(nodes, this.copies, (x, z) => this.standingHeight(x, z));
        } else {
            if (this.vegetation) this.vegetation.remove();
            if (this.nodeEffects) this.nodeEffects.remove();
        }
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
        this.camera.position.copy(focus).add(this.cameraOffset);
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
        this.focus.addScaledVector(this.groundRight, -dx * perPixel);
        this.focus.addScaledVector(this.groundUp, dy * perPixel / Math.sin(pitch));
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
        const top = new THREE.Plane(new THREE.Vector3(0, 1, 0), -1.6 * Math.max(1, this.heightScale));
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
            // the point of the ground under the polygon look's bent mesh
            const onGround = this.simple && this.simpleGround ? this.simpleGround.unwarp(point.x, point.z, { x: 0, z: 0 }) : point;
            const y = Math.floor(onGround.z);
            if (y >= 0 && y < this.mapView.height) {
                ground = { plane: this.plane, x: ((Math.floor(onGround.x) % width) + width) % width, y };
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
