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
