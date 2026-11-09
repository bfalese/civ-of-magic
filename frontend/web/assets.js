// Sprites by asset id (core/assets), from assets.json and its sheets (util/export-assets).
// Manifest: {"tileWidth", "tileHeight", "sheets": [...], "sprites": {id: [[sheet, x, y, w, h], ...frames]}}

export async function loadAssets(baseUrl) {
    const response = await fetch(`${baseUrl}/assets.json`);
    if (!response.ok) {
        throw new Error(`assets.json: ${response.status}`);
    }
    const manifest = await response.json();
    const sheets = await Promise.all(manifest.sheets.map((name) => new Promise((resolve, reject) => {
        const image = new Image();
        image.onload = () => resolve(image);
        image.onerror = () => reject(new Error(`${name} didn't load`));
        image.src = `${baseUrl}/${name}`;
    })));
    return new Assets(manifest, sheets);
}

export class Assets {
    constructor(manifest, sheets) {
        this.manifest = manifest;
        this.sheets = sheets;
        this.tileWidth = manifest.tileWidth;
        this.tileHeight = manifest.tileHeight;
        this.missing = new Set();
        // recoloured frames of banner-neutral sprites (battle units), by id/frame/banner
        this.recolored = new Map();
    }

    frameCount(id) {
        const frames = this.frames(id);
        return frames ? frames.length : 0;
    }

    // a frame with the banner marker colours replaced by the banner's (see util/export-assets),
    // as a canvas; cached
    bannerFrame(id, frame, banner) {
        const frames = this.frames(id);
        if (!frames) {
            return null;
        }
        frame = frame % frames.length;
        const key = `${id}/${frame}/${banner}`;
        let canvas = this.recolored.get(key);
        if (canvas) {
            return canvas;
        }
        const [sheet, sourceX, sourceY, width, height] = frames[frame];
        canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext("2d");
        context.drawImage(this.sheets[sheet], sourceX, sourceY, width, height, 0, 0, width, height);
        const markers = this.manifest.bannerMarker || [];
        const colors = (this.manifest.bannerColors || {})[banner];
        if (colors && markers.length) {
            const pixels = context.getImageData(0, 0, width, height);
            const data = pixels.data;
            for (let i = 0; i < data.length; i += 4) {
                if (data[i + 3] === 0) continue;
                for (let m = 0; m < markers.length; m++) {
                    const marker = markers[m];
                    if (data[i] === marker[0] && data[i + 1] === marker[1] && data[i + 2] === marker[2]) {
                        data[i] = colors[m][0];
                        data[i + 1] = colors[m][1];
                        data[i + 2] = colors[m][2];
                        break;
                    }
                }
            }
            context.putImageData(pixels, 0, 0);
        }
        this.recolored.set(key, canvas);
        return canvas;
    }

    frames(id) {
        const frames = this.manifest.sprites[id];
        if (!frames && !this.missing.has(id)) {
            this.missing.add(id);
            console.warn(`missing sprite ${id}`);
        }
        return frames;
    }

    has(id) {
        return id in this.manifest.sprites;
    }

    // a canvas with the sprite's first frame, shown scale times larger (pixelated); null when the
    // export doesn't have it (an older export: re-run util/export-assets)
    picture(id, scale = 1, className = "") {
        if (!this.has(id)) {
            return null;
        }
        const [width, height] = this.size(id);
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        canvas.style.width = `${width * scale}px`;
        canvas.style.height = `${height * scale}px`;
        canvas.style.imageRendering = "pixelated";
        if (className) canvas.className = className;
        this.draw(canvas.getContext("2d"), id, 0, 0);
        return canvas;
    }

    // [width, height] of the first frame
    size(id) {
        const frames = this.frames(id);
        return frames ? [frames[0][3], frames[0][4]] : [0, 0];
    }

    // draw a frame (animated sprites loop on frame) with its top left at x, y
    draw(context, id, x, y, frame = 0) {
        const frames = this.frames(id);
        if (!frames) {
            return false;
        }
        const [sheet, sourceX, sourceY, width, height] = frames[frame % frames.length];
        context.drawImage(this.sheets[sheet], sourceX, sourceY, width, height, x, y, width, height);
        return true;
    }
}
