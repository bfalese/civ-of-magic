// The overland map in 3D: the terrain (painted as in the 2D map, with roads, bonuses, fog and
// paths) lies on a ground plane with a tile grid, seen through an isometric orthographic camera;
// cities, stacks, encounters (lairs, caves, towers) and nodes stand on their tiles as billboards.
// A first step towards the retro 3D look: no height yet, the original sprites.
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

export class MapRenderer3D extends MapRenderer {
    constructor(container, assets, callbacks) {
        super(container, assets, callbacks);
        this.drawsThings = false;
        this.zoomIndex = 2;
        this.raycaster = new THREE.Raycaster();
        this.groundPlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), 0);
        // billboard textures and materials by sprite (and dimming)
        this.spriteTextures = new Map();
        this.spriteMaterials = new Map();
        // per copy of the map: a group of billboards
        this.billboards = [];
        // the selected stack's billboards (they blink)
        this.selectedSprites = [];
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

    // ground planes, each with the tile grid
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
        const ground = new THREE.PlaneGeometry(width, height);
        ground.rotateX(-Math.PI / 2);

        const lines = [];
        for (let x = 0; x <= width; x++) lines.push(x, 0, 0, x, 0, height);
        for (let z = 0; z <= height; z++) lines.push(0, 0, z, width, 0, z);
        const gridGeometry = new THREE.BufferGeometry();
        gridGeometry.setAttribute("position", new THREE.Float32BufferAttribute(lines, 3));
        const gridMaterial = new THREE.LineBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.35 });

        this.copies = [-1, 0, 1].map((offset) => {
            const copy = new THREE.Group();
            copy.position.set(offset * width, 0, 0);
            const mesh = new THREE.Mesh(ground, this.material);
            mesh.position.set(width / 2, 0, height / 2);
            copy.add(mesh);
            const grid = new THREE.LineSegments(gridGeometry, gridMaterial);
            grid.position.y = 0.002;
            copy.add(grid);
            this.scene.add(copy);
            return copy;
        });
        this.billboards = [];
    }

    setData(mapView, playerView) {
        super.setData(mapView, playerView);
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
            texture.userData = { width: canvas.width, height: canvas.height };
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

    // a billboard standing on the tile's centre, the sprite's pixels at the ground's scale
    billboard(texture, location, dimmed) {
        const sprite = new THREE.Sprite(this.spriteMaterial(texture, dimmed));
        sprite.center.set(0.5, 0);
        const { width, height } = texture.userData;
        sprite.scale.set(width / this.tileWidth, height / this.tileWidth, 1);
        sprite.position.set(location.x + 0.5, 0, location.y + 0.5);
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
        const plain = (id) => this.spriteTexture(id, (context) => {
            if (!context) return assets.size(id);
            assets.draw(context, id, 0, 0);
        });

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
            const stackTexture = (background, unit) => this.spriteTexture(`stack/${background}/${unit}`, (context) => {
                if (!context) return assets.size(background);
                assets.draw(context, background, 0, 0);
                assets.draw(context, unit, 1, 1);
            });
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
        // the selected stack blinks, like the original
        for (const sprite of this.selectedSprites || []) {
            sprite.visible = this.frame % 2 === 0;
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
        return { ndc, point: this.raycaster.ray.intersectPlane(this.groundPlane, new THREE.Vector3()) };
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
        // a thing standing on the ground tile under the pointer wins over a taller one in front
        // whose sprite reaches over it; otherwise the frontmost billboard, then the ground
        if (ground && hits.some((hit) => hit.object.userData.tile.x === ground.x && hit.object.userData.tile.y === ground.y)) {
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
        const point = new THREE.Vector3(worldX, 0, y + 0.5);
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
