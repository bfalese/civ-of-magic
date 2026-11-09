// The minimap in a corner (the desktop's minimap and cartographer): the explored terrain of the
// plane shown, in flat colours, dimmed where the player doesn't see now; cities in their banner's
// colour; the camera's rectangle. A click or drag moves the camera there.

const terrainColors = {
    Ocean: "#1f3a86", Shore: "#2d55a6", Lake: "#2d55a6", River: "#3f7a6a",
    Grasslands: "#4d8a36", Forest: "#2a5c26", Mountain: "#8a8070", Hills: "#7d7a48",
    Desert: "#c4ac68", Swamp: "#4b5a36", Tundra: "#c9c9c4", Volcano: "#a0402a",
    "Nature Node": "#3cb04a", "Sorcery Node": "#3a7ad8", "Chaos Node": "#d8602a",
};
const dim = 0.5;

export class Minimap {
    constructor(canvas, renderer, assets) {
        this.canvas = canvas;
        this.renderer = renderer;
        this.assets = assets;
        this.context = canvas.getContext("2d");
        // the terrain and cities, drawn once per new view; the camera rectangle goes over it
        this.base = document.createElement("canvas");
        this.scale = 3;
        this.lastCamera = "";
        this.bindInput();
    }

    bannerColor(banner) {
        const colors = (this.assets.manifest.bannerColors || {})[banner];
        if (!colors || !colors.length) return "#fff";
        const luminance = ([red, green, blue]) => 0.3 * red + 0.59 * green + 0.11 * blue;
        const [red, green, blue] = colors.reduce((best, color) => (luminance(color) > luminance(best) ? color : best));
        return `rgb(${red}, ${green}, ${blue})`;
    }

    // a new map view and player view
    setData(mapView, playerView) {
        const width = mapView.width;
        const height = mapView.height;
        // as large as fits 160 pixels each way, at least 2 per tile
        this.scale = Math.max(2, Math.min(4, Math.floor(160 / width), Math.floor(160 / height)));
        const scale = this.scale;
        this.base.width = width * scale;
        this.base.height = height * scale;
        this.canvas.width = this.base.width;
        this.canvas.height = this.base.height;

        const context = this.base.getContext("2d");
        context.fillStyle = "#000";
        context.fillRect(0, 0, this.base.width, this.base.height);
        mapView.tiles.forEach((row, y) => {
            row.forEach((tile, x) => {
                if (!tile) return;
                context.fillStyle = terrainColors[tile.terrain] || "#666";
                context.fillRect(x * scale, y * scale, scale, scale);
                if (!tile.visible) {
                    context.fillStyle = `rgba(0, 0, 0, ${dim})`;
                    context.fillRect(x * scale, y * scale, scale, scale);
                }
            });
        });

        const onPlane = (location) => location.plane === mapView.plane;
        const city = (location, banner) => {
            const size = scale + 2;
            context.fillStyle = "#000";
            context.fillRect(location.x * scale - 2, location.y * scale - 2, size + 2, size + 2);
            context.fillStyle = this.bannerColor(banner);
            context.fillRect(location.x * scale - 1, location.y * scale - 1, size, size);
        };
        for (const foreign of playerView["foreign-cities"]) {
            if (onPlane(foreign.location)) city(foreign.location, foreign.owner.banner);
        }
        for (const own of playerView.cities) {
            if (onPlane(own.location)) city(own.location, playerView.you.player.banner);
        }
        this.lastCamera = "";
        this.update();
    }

    // the camera's view in tiles: {x, y, width, height}
    cameraTiles() {
        return this.renderer.viewRect();
    }

    // redraw if the camera moved (called every frame)
    update() {
        if (!this.renderer.mapView || this.base.width === 0) {
            return;
        }
        const view = this.cameraTiles();
        const key = `${view.x.toFixed(2)},${view.y.toFixed(2)},${view.width.toFixed(2)}`;
        if (key === this.lastCamera) {
            return;
        }
        this.lastCamera = key;
        const context = this.context;
        const scale = this.scale;
        context.drawImage(this.base, 0, 0);
        context.strokeStyle = "#fff";
        context.lineWidth = 1;
        // the map wraps east-west: the rectangle may show on both edges
        const mapWidth = this.base.width;
        const x = ((view.x * scale) % mapWidth + mapWidth) % mapWidth;
        for (const offset of [0, -mapWidth]) {
            context.strokeRect(Math.round(x + offset) + 0.5, Math.round(view.y * scale) + 0.5, Math.round(view.width * scale), Math.round(view.height * scale));
        }
    }

    bindInput() {
        let dragging = false;
        const move = (event) => {
            const bounds = this.canvas.getBoundingClientRect();
            const mapView = this.renderer.mapView;
            if (!mapView) return;
            const x = Math.floor((event.clientX - bounds.left) / bounds.width * mapView.width);
            const y = Math.floor((event.clientY - bounds.top) / bounds.height * mapView.height);
            this.renderer.centerOn(Math.max(0, Math.min(mapView.width - 1, x)), Math.max(0, Math.min(mapView.height - 1, y)));
            this.update();
        };
        this.canvas.addEventListener("pointerdown", (event) => {
            dragging = true;
            this.canvas.setPointerCapture(event.pointerId);
            move(event);
        });
        this.canvas.addEventListener("pointermove", (event) => { if (dragging) move(event); });
        this.canvas.addEventListener("pointerup", () => { dragging = false; });
    }
}
