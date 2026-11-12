// The overland map in Three.js. For now the map is drawn into a canvas at the original pixel size
// (terrain, overlays, cities, stacks) and shown as a texture on a plane under an orthographic
// camera; height, tilt and the retro look come later. World units are original pixels; y grows
// downward on the map and upward in the world, so tile (x, y) covers world x [x*w, (x+1)*w] and
// y [-(y+1)*h, -y*h]. The map wraps east-west: three copies of the plane sit side by side.

import * as THREE from "three";

const zoomLevels = [1, 2, 3, 4, 6, 8];
const animationMilliseconds = 250;

// how dark explored land the player doesn't see now is drawn
const dimAlpha = 0.45;

// the soft fog edge pieces (mapback.lbx indexes, core/assets FogEdge)
const fogPieces = {
    northEast: 0, southEast: 1, east: 2, southWest: 3, south: 5, northWest: 7, north: 8, west: 11,
    cornerNorthEast: 10, cornerSouthWest: 13, cornerSouthEast: 6, cornerNorthWest: 12,
};
const fogEdgeAsset = (index) => `fog/${index}`;

// overlays that stand up from the ground (lairs, caves, towers...): billboards in the 3D view
export const isStanding = (overlay) => overlay.startsWith("encounter/");

export class MapRenderer {
    constructor(container, assets, { onTileClick, onTileHover, onFrame }) {
        this.container = container;
        this.assets = assets;
        this.onTileClick = onTileClick;
        // the tile under the pointer changed (null when it leaves the map)
        this.onTileHover = onTileHover || (() => {});
        // after every rendered frame (the minimap follows the camera)
        this.onFrame = onFrame || (() => {});
        this.hovered = null;
        this.tileWidth = assets.tileWidth;
        this.tileHeight = assets.tileHeight;

        this.renderer = new THREE.WebGLRenderer({ antialias: false });
        this.renderer.setPixelRatio(window.devicePixelRatio);
        this.renderer.setClearColor(0x000000);
        container.appendChild(this.renderer.domElement);

        this.scene = new THREE.Scene();
        this.camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 100);
        this.camera.position.set(0, 0, 10);
        this.zoomIndex = 3;

        this.canvas = document.createElement("canvas");
        this.context = this.canvas.getContext("2d");
        this.texture = new THREE.CanvasTexture(this.canvas);
        this.texture.magFilter = THREE.NearestFilter;
        this.texture.minFilter = THREE.NearestFilter;
        this.texture.generateMipmaps = false;
        this.texture.colorSpace = THREE.SRGBColorSpace;
        this.material = new THREE.MeshBasicMaterial({ map: this.texture });
        this.copies = [];

        this.mapView = null;
        this.playerView = null;
        this.plane = "Arcanus";
        this.selected = null;
        // whether cities, stacks and encounters are painted on the map (the 3D view shows them as
        // billboards instead)
        this.drawsThings = true;
        // whether the terrain's art is painted (the 3D view's simple ground has its own colours
        // and shows only roads, fog, the grid and paths from this canvas, on a transparent one)
        this.drawsTerrain = true;
        // a faint line along the tiles' edges (the 3D view's grid)
        this.showGrid = false;
        // a path being picked (a road): [{x, y}] on the shown plane, drawn over everything
        this.previewPath = null;
        this.frame = 0;
        this.keys = new Set();

        this.bindInput();
        this.resize();
        window.addEventListener("resize", () => this.resize());
    }

    get pixelWidth() {
        return this.mapView ? this.mapView.width * this.tileWidth : 0;
    }

    get pixelHeight() {
        return this.mapView ? this.mapView.height * this.tileHeight : 0;
    }

    // a new map view (after a refresh or a plane switch) and player view
    setData(mapView, playerView) {
        const sizeChanged = !this.mapView || this.mapView.width !== mapView.width || this.mapView.height !== mapView.height;
        this.mapView = mapView;
        this.playerView = playerView;
        this.plane = mapView.plane;
        if (sizeChanged) {
            this.buildPlanes();
        }
        this.draw();
    }

    // the selected stack's location (it blinks), or null
    setSelected(location) {
        this.selected = location;
        this.draw();
    }

    // a path to show while the player picks a destination (null clears it)
    setPreviewPath(points) {
        this.previewPath = points && points.length ? points : null;
        this.draw();
    }

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

        const geometry = new THREE.PlaneGeometry(this.pixelWidth, this.pixelHeight);
        this.copies = [-1, 0, 1].map((offset) => {
            const mesh = new THREE.Mesh(geometry, this.material);
            mesh.position.set(this.pixelWidth * (offset + 0.5), -this.pixelHeight / 2, 0);
            this.scene.add(mesh);
            return mesh;
        });
    }

    // center the camera on a tile
    centerOn(x, y) {
        this.camera.position.x = (x + 0.5) * this.tileWidth;
        this.camera.position.y = -(y + 0.5) * this.tileHeight;
        this.clampCamera();
    }

    draw() {
        if (!this.mapView) {
            return;
        }
        const context = this.context;
        const assets = this.assets;
        const tileWidth = this.tileWidth;
        const tileHeight = this.tileHeight;
        const frame = this.frame;

        if (this.drawsTerrain) {
            context.fillStyle = "#000";
            context.fillRect(0, 0, this.canvas.width, this.canvas.height);
        } else {
            // unexplored land stays black
            context.clearRect(0, 0, this.canvas.width, this.canvas.height);
            context.fillStyle = "#000";
            this.mapView.tiles.forEach((row, y) => {
                row.forEach((tile, x) => {
                    if (!tile) context.fillRect(x * tileWidth, y * tileHeight, tileWidth, tileHeight);
                });
            });
        }

        // terrain and what lies on it
        this.mapView.tiles.forEach((row, y) => {
            row.forEach((tile, x) => {
                if (!tile) {
                    return;
                }
                if (this.drawsTerrain) {
                    assets.draw(context, tile.asset, x * tileWidth, y * tileHeight, frame);
                }
                for (const overlay of tile.overlays || []) {
                    if (this.standsUp(overlay)) continue;
                    assets.draw(context, overlay, x * tileWidth, y * tileHeight, frame);
                }
            });
        });

        const view = this.playerView;
        if (view) {
            const onPlane = (location) => location.plane === this.plane;
            const centered = (id, location) => {
                const [width, height] = assets.size(id);
                assets.draw(context, id, location.x * tileWidth + Math.floor((tileWidth - width) / 2), location.y * tileHeight + Math.floor((tileHeight - height) / 2), frame);
            };

            for (const city of [...view["foreign-cities"], ...view.cities]) {
                if (this.drawsThings && onPlane(city.location) && city.asset) {
                    centered(city.asset, city.location);
                }
            }

            // stacks in a city aren't drawn (the city is), unless selected: as the original
            const cityTiles = new Set([...view["foreign-cities"], ...view.cities].map((city) => `${city.location.plane}/${city.location.x}/${city.location.y}`));
            const inCity = (location) => cityTiles.has(`${location.plane}/${location.x}/${location.y}`);

            const drawStack = (location, background, unitAsset) => {
                assets.draw(context, background, location.x * tileWidth, location.y * tileHeight);
                assets.draw(context, unitAsset, location.x * tileWidth + 1, location.y * tileHeight + 1);
            };
            for (const stack of view["foreign-stacks"]) {
                if (this.drawsThings && onPlane(stack.location) && !inCity(stack.location)) {
                    drawStack(stack.location, stack.background, stack.asset);
                }
            }

            const selected = this.selected;
            for (const stack of view.stacks) {
                if (!onPlane(stack.location) || stack.units.length === 0) {
                    continue;
                }
                const isSelected = selected && selected.plane === stack.location.plane && selected.x === stack.location.x && selected.y === stack.location.y;
                if (isSelected) {
                    // the walk ahead
                    context.fillStyle = "#fff";
                    for (const point of stack.path || []) {
                        context.fillRect(point.x * tileWidth + tileWidth / 2 - 1, point.y * tileHeight + tileHeight / 2 - 1, 2, 2);
                    }
                    // blinks, like the original
                    if (frame % 2 === 1 || !this.drawsThings) {
                        continue;
                    }
                } else if (inCity(stack.location) || !this.drawsThings) {
                    continue;
                }
                drawStack(stack.location, stack.background, stack.units[0].asset);
            }
        }

        // the tile grid: one texel along each explored tile's top and left edge
        if (this.showGrid) {
            context.fillStyle = "rgba(0, 0, 0, 0.35)";
            this.mapView.tiles.forEach((row, y) => {
                row.forEach((tile, x) => {
                    if (tile) {
                        context.fillRect(x * tileWidth, y * tileHeight, tileWidth, 1);
                        context.fillRect(x * tileWidth, y * tileHeight, 1, tileHeight);
                    }
                });
            });
        }

        // what the player saw before but doesn't see now is dimmed
        context.fillStyle = `rgba(0, 0, 0, ${dimAlpha})`;
        this.mapView.tiles.forEach((row, y) => {
            row.forEach((tile, x) => {
                if (tile && !tile.visible) {
                    context.fillRect(x * tileWidth, y * tileHeight, tileWidth, tileHeight);
                }
            });
        });
        this.drawFogEdges(context);

        // the path being picked: a yellow line through the tile centres, the end marked
        if (this.previewPath) {
            const centre = (point) => [point.x * tileWidth + tileWidth / 2, point.y * tileHeight + tileHeight / 2];
            context.strokeStyle = "#ffd94a";
            context.fillStyle = "#ffd94a";
            context.lineWidth = 1;
            context.beginPath();
            this.previewPath.forEach((point, index) => {
                const [x, y] = centre(point);
                // the map wraps: don't draw a line across it between neighbours on the two edges
                const previous = index > 0 ? this.previewPath[index - 1] : null;
                if (!previous || Math.abs(previous.x - point.x) > 1) context.moveTo(x, y); else context.lineTo(x, y);
            });
            context.stroke();
            for (const point of this.previewPath) {
                const [x, y] = centre(point);
                context.fillRect(x - 1, y - 1, 3, 3);
            }
            const [endX, endY] = centre(this.previewPath[this.previewPath.length - 1]);
            context.strokeRect(endX - tileWidth / 2 + 0.5, endY - tileHeight / 2 + 0.5, tileWidth - 1, tileHeight - 1);
        }

        this.texture.needsUpdate = true;
        this.afterDraw();
    }

    // whether an overlay stands up as a billboard instead of being painted on the ground
    standsUp(overlay) {
        return !this.drawsThings && isStanding(overlay);
    }

    // after each repaint (the 3D view blinks the selected stack's billboard here)
    afterDraw() {}

    // every animation frame, before rendering (the 3D map glides moving stacks here)
    beforeRender(now) {}

    // stacks walking (stack-moved events): only the 3D map animates them
    playMoves(walks) {}

    // move the view as if the map were dragged by dx, dy screen pixels
    panBy(dx, dy) {
        this.camera.position.x -= dx / this.zoom;
        this.camera.position.y += dy / this.zoom;
        this.clampCamera();
    }

    // the part of the map in view, in tiles: {x, y, width, height} (x may run past the map's
    // width, which wraps)
    viewRect() {
        const camera = this.camera;
        return {
            x: (camera.position.x + camera.left) / this.tileWidth,
            y: -(camera.position.y + camera.top) / this.tileHeight,
            width: (camera.right - camera.left) / this.tileWidth,
            height: (camera.top - camera.bottom) / this.tileHeight,
        };
    }

    // soft edges, as the desktop Overworld.DrawFog: an explored tile next to unexplored land gets
    // black edge pieces on those sides; a visible tile next to explored-but-not-visible land gets
    // them at the dimming strength, so the dim area fades in
    drawFogEdges(context) {
        const assets = this.assets;
        if (!assets.has(fogEdgeAsset(0))) {
            return;
        }
        const tiles = this.mapView.tiles;
        const width = this.mapView.width;
        const height = this.mapView.height;
        const { tileWidth, tileHeight } = assets;
        const kind = (x, y) => {
            if (y < 0 || y >= height) return "edge";
            const tile = tiles[y][((x % width) + width) % width];
            return !tile ? "unexplored" : tile.visible ? "visible" : "explored";
        };
        const piece = (index, x, y) => assets.draw(context, fogEdgeAsset(index), x * tileWidth, y * tileHeight);
        // which pieces border (x, y) where the neighbours are of the given kind
        const border = (x, y, fogKind) => {
            const is = (dx, dy) => kind(x + dx, y + dy) === fogKind;
            const n = is(0, -1), e = is(1, 0), s = is(0, 1), w = is(-1, 0);
            if (n && e) piece(fogPieces.northEast, x, y);
            else if (n) piece(fogPieces.north, x, y);
            else if (e) piece(fogPieces.east, x, y);
            else if (is(1, -1)) piece(fogPieces.cornerNorthEast, x, y);
            if (s && e) piece(fogPieces.southEast, x, y);
            else if (s) piece(fogPieces.south, x, y);
            else if (is(1, 1)) piece(fogPieces.cornerSouthEast, x, y);
            if (n && w) piece(fogPieces.northWest, x, y);
            else if (w) piece(fogPieces.west, x, y);
            else if (is(-1, -1)) piece(fogPieces.cornerNorthWest, x, y);
            if (s && w) piece(fogPieces.southWest, x, y);
            else if (is(-1, 1)) piece(fogPieces.cornerSouthWest, x, y);
        };
        for (let y = 0; y < height; y++) {
            for (let x = 0; x < width; x++) {
                const here = kind(x, y);
                if (here === "visible") {
                    context.globalAlpha = dimAlpha;
                    border(x, y, "explored");
                    context.globalAlpha = 1;
                }
                if (here !== "unexplored") {
                    border(x, y, "unexplored");
                }
            }
        }
        context.globalAlpha = 1;
    }

    resize() {
        const width = this.container.clientWidth;
        const height = this.container.clientHeight;
        this.renderer.setSize(width, height);
        this.updateCamera();
    }

    get zoom() {
        return zoomLevels[this.zoomIndex];
    }

    updateCamera() {
        const width = this.container.clientWidth / this.zoom;
        const height = this.container.clientHeight / this.zoom;
        this.camera.left = -width / 2;
        this.camera.right = width / 2;
        this.camera.top = height / 2;
        this.camera.bottom = -height / 2;
        this.camera.updateProjectionMatrix();
        this.clampCamera();
    }

    clampCamera() {
        if (!this.mapView) {
            return;
        }
        const camera = this.camera;
        // east-west wraps; north-south stops at the edges
        const width = this.pixelWidth;
        camera.position.x = ((camera.position.x % width) + width) % width;
        const halfHeight = (camera.top - camera.bottom) / 2;
        const minimum = -this.pixelHeight + halfHeight;
        const maximum = -halfHeight;
        camera.position.y = minimum > maximum ? -this.pixelHeight / 2 : Math.min(maximum, Math.max(minimum, camera.position.y));
    }

    zoomBy(steps) {
        this.zoomIndex = Math.max(0, Math.min(zoomLevels.length - 1, this.zoomIndex + steps));
        this.updateCamera();
    }

    // a tile's centre in page coordinates (the copy of the wrapping map nearest the camera), for
    // tests and anything that points at a tile
    screenPoint(x, y) {
        const bounds = this.renderer.domElement.getBoundingClientRect();
        const width = this.pixelWidth;
        let dx = (x + 0.5) * this.tileWidth - this.camera.position.x;
        dx = ((dx % width) + width * 1.5) % width - width / 2;
        const dy = -(y + 0.5) * this.tileHeight - this.camera.position.y;
        return [bounds.left + bounds.width / 2 + dx * this.zoom, bounds.top + bounds.height / 2 - dy * this.zoom];
    }

    // the tile under a point of the canvas, or null
    tileAt(clientX, clientY) {
        if (!this.mapView) {
            return null;
        }
        const bounds = this.renderer.domElement.getBoundingClientRect();
        // the camera may have moved since the last frame (centerOn)
        this.camera.updateMatrixWorld();
        const ndc = new THREE.Vector3(((clientX - bounds.left) / bounds.width) * 2 - 1, -((clientY - bounds.top) / bounds.height) * 2 + 1, 0);
        ndc.unproject(this.camera);
        const width = this.mapView.width;
        const x = ((Math.floor(ndc.x / this.tileWidth) % width) + width) % width;
        const y = Math.floor(-ndc.y / this.tileHeight);
        if (y < 0 || y >= this.mapView.height) {
            return null;
        }
        return { plane: this.plane, x, y };
    }

    bindInput() {
        const element = this.renderer.domElement;
        let down = null;

        element.addEventListener("pointerdown", (event) => {
            down = { x: event.clientX, y: event.clientY, dragged: false };
            element.setPointerCapture(event.pointerId);
        });
        element.addEventListener("pointermove", (event) => {
            if (!down) {
                const tile = this.tileAt(event.clientX, event.clientY);
                const same = tile && this.hovered && tile.x === this.hovered.x && tile.y === this.hovered.y && tile.plane === this.hovered.plane;
                if (!same && (tile || this.hovered)) {
                    this.hovered = tile;
                    this.onTileHover(tile, event);
                }
                return;
            }
            const dx = event.clientX - down.x;
            const dy = event.clientY - down.y;
            if (!down.dragged && Math.hypot(dx, dy) > 4) {
                down.dragged = true;
            }
            if (down.dragged) {
                this.panBy(dx, dy);
                down.x = event.clientX;
                down.y = event.clientY;
            }
        });
        element.addEventListener("pointerup", (event) => {
            if (down && !down.dragged) {
                const tile = this.tileAt(event.clientX, event.clientY);
                if (tile) {
                    this.onTileClick(tile, event);
                }
            }
            down = null;
        });
        element.addEventListener("wheel", (event) => {
            event.preventDefault();
            this.zoomBy(event.deltaY < 0 ? 1 : -1);
        }, { passive: false });
        element.addEventListener("contextmenu", (event) => event.preventDefault());
        element.addEventListener("pointerleave", () => {
            if (this.hovered) {
                this.hovered = null;
                this.onTileHover(null);
            }
        });

        window.addEventListener("keydown", (event) => {
            if (event.target instanceof HTMLInputElement) {
                return;
            }
            this.keys.add(event.key);
            if (event.key === "+" || event.key === "=") this.zoomBy(1);
            if (event.key === "-") this.zoomBy(-1);
        });
        window.addEventListener("keyup", (event) => this.keys.delete(event.key));
    }

    // the render loop: animation frames, keyboard panning
    start() {
        let lastFrame = performance.now();
        let lastTime = performance.now();
        const loop = (now) => {
            const seconds = (now - lastTime) / 1000;
            lastTime = now;
            // screen pixels per second, whatever the zoom
            const speed = 600 * seconds;
            if (this.keys.has("ArrowLeft")) this.panBy(speed, 0);
            if (this.keys.has("ArrowRight")) this.panBy(-speed, 0);
            if (this.keys.has("ArrowUp")) this.panBy(0, speed);
            if (this.keys.has("ArrowDown")) this.panBy(0, -speed);

            if (now - lastFrame >= animationMilliseconds) {
                lastFrame = now;
                this.frame += 1;
                this.draw();
            }
            this.beforeRender(now);
            this.renderer.render(this.scene, this.camera);
            this.onFrame();
            requestAnimationFrame(loop);
        };
        requestAnimationFrame(loop);
    }
}
