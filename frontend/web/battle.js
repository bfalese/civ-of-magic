// The tactical battle screen: the battlefield from the battle view (core BattleSession.View),
// drawn isometrically like the original (the grid turned 45 degrees), units as groups of figures
// in their owner's colours. Clicking a tile sends battle-act (move there, or attack the enemy on
// it); the buttons send battle-done / battle-wait / battle-auto / battle-flee.
//
// Geometry, as the Ebitengine battle screen: a tile's centre is S * R(-45deg) * (x, y) with
// S = (w*3/4 - 2, h*3/4 - 1) for the ground tile size w x h; the selection diamond, units and
// tops are placed relative to that centre.

const animationMilliseconds = 160;

export class BattleScreen {
    constructor(root, assets, { onAct, onCommand }) {
        this.root = root;
        this.assets = assets;
        this.onAct = onAct;
        this.onCommand = onCommand;
        this.view = null;
        this.hover = null;
        this.frame = 0;
        this.running = false;

        const [tileWidth, tileHeight] = assets.size("battle-tile/cmbgrass/0");
        this.tileWidth = tileWidth || 30;
        this.tileHeight = tileHeight || 16;
        this.scaleX = this.tileWidth * 3 / 4 - 2;
        this.scaleY = this.tileHeight * 3 / 4 - 1;

        root.innerHTML = `
            <div class="battle-bar panel">
                <span class="battle-title"></span>
                <span class="battle-unit"></span>
                <span class="spacer"></span>
                <button data-command="battle-done" title="D">Done</button>
                <button data-command="battle-wait" title="W">Wait</button>
                <button data-command="battle-auto" title="A">Auto</button>
                <button data-command="battle-flee">Flee</button>
            </div>
            <canvas class="battle-field"></canvas>
            <div class="battle-hint panel">click a highlighted tile to move · a red one to attack · D done · W wait · A auto</div>`;
        this.canvas = root.querySelector("canvas");
        this.context = this.canvas.getContext("2d");
        for (const button of root.querySelectorAll("button[data-command]")) {
            button.onclick = () => this.onCommand(button.dataset.command);
        }
        this.canvas.addEventListener("click", (event) => {
            const tile = this.tileAt(event);
            if (tile) {
                this.onAct(tile.x, tile.y);
            }
        });
        this.canvas.addEventListener("mousemove", (event) => {
            this.hover = this.tileAt(event);
        });
        window.addEventListener("resize", () => this.layout());
    }

    get open() {
        return this.root.classList.contains("open");
    }

    show(view) {
        const firstTime = !this.view || this.view.width !== view.width || this.view.height !== view.height;
        this.view = view;
        this.root.classList.add("open");
        if (firstTime) {
            this.computeBounds();
            this.layout();
        }
        this.updateBar();
        if (!this.running) {
            this.running = true;
            this.loop();
        }
    }

    hide() {
        this.root.classList.remove("open");
        this.running = false;
        this.view = null;
    }

    // a tile centre in battlefield pixels (before the canvas offset)
    project(x, y) {
        const rotatedX = Math.SQRT1_2 * (x + y);
        const rotatedY = Math.SQRT1_2 * (y - x);
        return [rotatedX * this.scaleX, rotatedY * this.scaleY];
    }

    // the part of the field the original shows: its 320x200 screen, where the grid's centre sits
    // (the Ebitengine screen translates tile centres by (-220, 80))
    computeBounds() {
        this.offsetX = -220;
        this.offsetY = 80;
        this.canvas.width = 320;
        this.canvas.height = 200;
    }

    // the largest whole-number zoom that fits the window
    layout() {
        if (!this.view) {
            return;
        }
        const availableWidth = window.innerWidth - 16;
        const availableHeight = window.innerHeight - 110;
        const zoom = Math.max(1, Math.floor(Math.min(availableWidth / this.canvas.width, availableHeight / this.canvas.height)));
        this.canvas.style.width = `${this.canvas.width * zoom}px`;
        this.canvas.style.height = `${this.canvas.height * zoom}px`;
    }

    center(x, y) {
        const [px, py] = this.project(x, y);
        return [px + this.offsetX, py + this.offsetY];
    }

    tileAt(event) {
        if (!this.view) {
            return null;
        }
        const bounds = this.canvas.getBoundingClientRect();
        const px = (event.clientX - bounds.left) * this.canvas.width / bounds.width - this.offsetX;
        const py = (event.clientY - bounds.top) * this.canvas.height / bounds.height - this.offsetY;
        const rotatedX = px / this.scaleX;
        const rotatedY = py / this.scaleY;
        const x = Math.round(Math.SQRT1_2 * (rotatedX - rotatedY));
        const y = Math.round(Math.SQRT1_2 * (rotatedX + rotatedY));
        if (x < 0 || y < 0 || x >= this.view.width || y >= this.view.height) {
            return null;
        }
        return { x, y };
    }

    selectedUnit() {
        const view = this.view;
        return view && view.selected !== undefined ? view.units.find((unit) => unit.id === view.selected) : null;
    }

    updateBar() {
        const view = this.view;
        const side = view["you-attack"] ? "you attack" : "you defend";
        this.root.querySelector(".battle-title").textContent = `Battle · turn ${view.turn} · ${side}`;
        const unit = this.selectedUnit();
        this.root.querySelector(".battle-unit").textContent = unit
            ? `${unit.name}: ${unit.health}/${unit["max-health"]} hp · ${unit["moves-left"]} moves${unit["ranged-attacks"] ? ` · ${unit["ranged-attacks"]} shots` : ""}`
            : "waiting…";
        for (const button of this.root.querySelectorAll("button[data-command='battle-done'], button[data-command='battle-wait']")) {
            button.disabled = !unit;
        }
    }

    loop() {
        let last = performance.now();
        const step = (now) => {
            if (!this.running) {
                return;
            }
            if (now - last >= animationMilliseconds) {
                last = now;
                this.frame += 1;
            }
            this.draw(now);
            requestAnimationFrame(step);
        };
        requestAnimationFrame(step);
    }

    diamond(x, y, stroke, fill) {
        const [cx, cy] = this.center(x, y);
        const context = this.context;
        const halfWidth = this.tileWidth / 2;
        const halfHeight = this.tileHeight / 2;
        context.beginPath();
        context.moveTo(cx - halfWidth, cy);
        context.lineTo(cx, cy - halfHeight);
        context.lineTo(cx + halfWidth, cy);
        context.lineTo(cx, cy + halfHeight);
        context.closePath();
        if (fill) {
            context.fillStyle = fill;
            context.fill();
        }
        if (stroke) {
            context.strokeStyle = stroke;
            context.lineWidth = 1;
            context.stroke();
        }
    }

    draw(now) {
        const view = this.view;
        if (!view) {
            return;
        }
        const context = this.context;
        const assets = this.assets;
        context.imageSmoothingEnabled = false;
        context.fillStyle = "#000";
        context.fillRect(0, 0, this.canvas.width, this.canvas.height);

        // ground, centred on each tile
        for (let y = 0; y < view.height; y++) {
            for (let x = 0; x < view.width; x++) {
                const tile = view.tiles[y][x];
                const [cx, cy] = this.center(x, y);
                const [width, height] = assets.size(tile.asset);
                assets.draw(context, tile.asset, Math.round(cx - width / 2), Math.round(cy - height / 2), this.frame);
                if (tile.mud) {
                    assets.draw(context, "battle-mud", Math.round(cx - width / 2), Math.round(cy - height / 2), this.frame);
                }
            }
        }

        if (view.road) {
            const [rx, ry] = this.center(10, 6);
            assets.draw(context, "battle-road", Math.round(rx), Math.round(ry + this.tileHeight / 2));
        }
        if (view.clouds) {
            const [cx, cy] = this.center(11, 5);
            assets.draw(context, "battle-clouds", Math.round(cx), Math.round(cy + this.tileHeight / 2));
        }

        // where the selected unit can go and what it can hit
        const pulse = (Math.sin(now / 160) + 1) / 2;
        for (const [x, y] of view.moves) {
            this.diamond(x, y, "rgba(255, 255, 255, 0.35)", "rgba(255, 255, 255, 0.08)");
        }
        const targets = new Map(view.targets.map((target) => [target.id, target.kind]));
        for (const unit of view.units) {
            if (targets.has(unit.id)) {
                const color = targets.get(unit.id) === "ranged" ? `rgba(255, 170, 40, ${0.5 + pulse / 2})` : `rgba(255, 50, 40, ${0.5 + pulse / 2})`;
                this.diamond(unit.x, unit.y, color, "rgba(255, 60, 40, 0.15)");
            }
        }
        const selected = this.selectedUnit();
        if (selected) {
            this.diamond(selected.x, selected.y, `rgba(255, 220, 60, ${0.6 + pulse * 0.4})`, "rgba(255, 220, 60, 0.15)");
        }
        if (this.hover) {
            this.diamond(this.hover.x, this.hover.y, "rgba(0, 230, 255, 0.8)");
        }

        // tops and units, back to front
        const things = [];
        for (let y = 0; y < view.height; y++) {
            for (let x = 0; x < view.width; x++) {
                const tile = view.tiles[y][x];
                if (tile.top) {
                    things.push({ depth: this.center(x, y)[1], order: 0, draw: () => this.drawTop(tile, x, y) });
                }
            }
        }
        for (const unit of view.units) {
            things.push({ depth: this.center(unit.x, unit.y)[1], order: 1, draw: () => this.drawUnit(unit, unit === selected, pulse) });
        }
        things.sort((a, b) => a.depth - b.depth || a.order - b.order);
        for (const thing of things) {
            thing.draw();
        }
    }

    drawTop(tile, x, y) {
        const [cx, cy] = this.center(x, y);
        const [, height] = this.assets.size(tile.top);
        // the original offsets both ways by half the image's height
        if (tile["top-align"] === "bottom") {
            this.assets.draw(this.context, tile.top, Math.round(cx - height / 2), Math.round(cy + this.tileHeight / 2 - height), this.frame);
        } else {
            this.assets.draw(this.context, tile.top, Math.round(cx - height / 2), Math.round(cy - height / 2), this.frame);
        }
    }

    drawUnit(unit, isSelected, pulse) {
        const frames = this.assets.frameCount(unit.asset);
        if (frames === 0) {
            return;
        }
        // flying units loop their movement frames (all but the last), others stand still
        const frame = unit.flying && frames > 1 ? this.frame % (frames - 1) : 0;
        const image = this.assets.bannerFrame(unit.asset, frame, unit.banner);
        const [cx, cy] = this.center(unit.x, unit.y);
        const context = this.context;
        // the figures' feet are 6 pixels above the bottom of the image
        for (const [dx, dy] of unit.figures) {
            const left = Math.round(cx + dx - image.width / 2);
            const top = Math.round(cy + dy - image.height + 6);
            context.drawImage(image, left, top);
            if (isSelected) {
                context.globalAlpha = 0.25 + pulse * 0.25;
                context.globalCompositeOperation = "lighter";
                context.drawImage(image, left, top);
                context.globalCompositeOperation = "source-over";
                context.globalAlpha = 1;
            }
        }
        // a health bar under the unit
        const ratio = unit["max-health"] > 0 ? unit.health / unit["max-health"] : 0;
        context.fillStyle = "rgba(0, 0, 0, 0.7)";
        context.fillRect(Math.round(cx - 8), Math.round(cy + 5), 16, 2);
        context.fillStyle = ratio > 0.5 ? "#6c6" : ratio > 0.25 ? "#dc4" : "#d44";
        context.fillRect(Math.round(cx - 8), Math.round(cy + 5), Math.max(1, Math.round(16 * ratio)), 2);
        if (!unit.yours) {
            context.fillStyle = "rgba(255, 80, 60, 0.9)";
            context.fillRect(Math.round(cx - 9), Math.round(cy + 5), 1, 2);
        }
    }
}
