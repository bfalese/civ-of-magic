// The tactical battle screen: the battlefield from the battle view (core BattleSession.View),
// drawn isometrically like the original (the grid turned 45 degrees), units as groups of figures
// in their owner's colours. Clicking a tile sends battle-act (move there, or attack the enemy on
// it); the buttons send battle-done / battle-wait / battle-auto / battle-flee.
//
// Spells: the Spells button lists the wizard's combat spells (view.magic) and the selected unit's
// (view["unit-magic"]); picking one sends battle-cast without a target. A spell that needs one
// waits in view.casting, which lists the units, tiles or choices it accepts: the field highlights
// them, a click sends battle-target, Esc sends battle-cancel.
//
// The core resolves moves and attacks at once; each view carries the steps that led to it
// (view.steps: moves with the tiles walked, attacks, spells, health changes). The screen replays
// them from the previous view before showing the new one: units walk tile by tile, attackers
// lunge, damage floats up, the dead fade out. A click skips the rest of the replay. Hovering a unit shows its
// details; the latest lines of the battle log sit at the bottom of the field.
//
// Geometry, as the Ebitengine battle screen: a tile's centre is S * R(-45deg) * (x, y) with
// S = (w*3/4 - 2, h*3/4 - 1) for the ground tile size w x h; the selection diamond, units and
// tops are placed relative to that centre.

const animationMilliseconds = 160;

const facingNames = ["right", "up-right", "up", "up-left", "left", "down-left", "down", "down-right"];
const stepMilliseconds = { tile: 140, teleport: 260, melee: 460, ranged: 460, spell: 520, effect: 420 };
const floatMilliseconds = 1000;
const fadeMilliseconds = 450;

// as core combat.FaceTowards: the grid is turned 45 degrees on screen
function faceTowards(x1, y1, x2, y2) {
    if (x1 === x2 && y1 === y2) {
        return null;
    }
    const screenAngle = -(Math.atan2(y2 - y1, x2 - x1) - Math.PI / 4);
    const octant = ((Math.round(screenAngle / (Math.PI / 4)) % 8) + 8) % 8;
    return facingNames[octant];
}

function facedAsset(asset, facing) {
    return facing ? asset.replace(/[^/]+$/, facing) : asset;
}

function stepDuration(step) {
    if (step.kind === "move") {
        return Math.max(1, (step.path || []).length) * stepMilliseconds.tile;
    }
    return stepMilliseconds[step.kind] || stepMilliseconds.effect;
}

const realmColors = {
    Life: "#f4f0d8", Death: "#b07ae0", Chaos: "#ff7a50", Nature: "#7ed06a", Sorcery: "#6ab4ff", Arcane: "#c8c0b0",
};

export class BattleScreen {
    constructor(root, assets, { onCommand }) {
        this.root = root;
        this.assets = assets;
        this.onCommand = onCommand;
        this.view = null;
        this.hover = null;
        this.frame = 0;
        this.running = false;
        this.spellsOpen = false;
        this.message = "";
        // the steps being replayed, with the units as they were (see startReplay)
        this.replay = null;
        // damage numbers rising from tiles
        this.floaters = [];
        this.infoUnit = null;

        const [tileWidth, tileHeight] = assets.size("battle-tile/cmbgrass/0");
        this.tileWidth = tileWidth || 30;
        this.tileHeight = tileHeight || 16;
        this.scaleX = this.tileWidth * 3 / 4 - 2;
        this.scaleY = this.tileHeight * 3 / 4 - 1;

        root.innerHTML = `
            <div class="battle-bar panel">
                <span class="battle-title"></span>
                <span class="battle-unit"></span>
                <span class="battle-mana"></span>
                <span class="spacer"></span>
                <button class="battle-spells-button" title="S">Spells</button>
                <button data-command="battle-done" title="D">Done</button>
                <button data-command="battle-wait" title="W">Wait</button>
                <button data-command="battle-auto" title="A">Auto</button>
                <button data-command="battle-flee">Flee</button>
            </div>
            <div class="battle-stage">
                <canvas class="battle-field"></canvas>
                <div class="battle-spells panel" hidden></div>
                <div class="battle-info panel" hidden></div>
                <div class="battle-log"></div>
            </div>
            <div class="battle-hint panel"></div>`;
        this.canvas = root.querySelector("canvas");
        this.context = this.canvas.getContext("2d");
        for (const button of root.querySelectorAll("button[data-command]")) {
            button.onclick = () => this.command(button.dataset.command);
        }
        root.querySelector(".battle-spells-button").onclick = () => this.toggleSpells();
        this.canvas.addEventListener("click", (event) => {
            // a click during a replay skips to its end, and acts
            if (this.replay) {
                this.finishReplay();
            }
            const tile = this.tileAt(event);
            if (!tile) {
                return;
            }
            if (this.view && this.view.casting) {
                this.pickTarget(tile);
            } else {
                this.command("battle-act", { x: tile.x, y: tile.y });
            }
        });
        this.canvas.addEventListener("mousemove", (event) => {
            this.hover = this.tileAt(event);
            this.updateInfo();
        });
        this.canvas.addEventListener("mouseleave", () => {
            this.hover = null;
            this.updateInfo();
        });
        window.addEventListener("resize", () => this.layout());
    }

    get open() {
        return this.root.classList.contains("open");
    }

    show(view) {
        const previous = this.view;
        const firstTime = !previous || previous.width !== view.width || previous.height !== view.height;
        if (this.replay) {
            this.finishReplay();
        }
        this.view = view;
        if (!firstTime && view.steps && view.steps.length > 0) {
            this.startReplay(previous, view);
        }
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
        this.spellsOpen = false;
        this.message = "";
        this.replay = null;
        this.floaters = [];
        this.infoUnit = null;
    }

    // ---- replaying what a command did

    // the units as the previous view had them, changed step by step
    startReplay(previous, view) {
        const units = new Map(previous.units.map((unit) => [unit.id, { ...unit }]));
        this.replay = { steps: view.steps, index: 0, start: performance.now(), units };
        this.replayCount = (this.replayCount || 0) + 1;
    }

    // the units to draw: as the replay has them, or the view's
    displayUnits() {
        if (this.replay) {
            return [...this.replay.units.values()];
        }
        return this.view ? this.view.units : [];
    }

    // move the replay on to the step running at time now
    advanceReplay(now) {
        const replay = this.replay;
        if (!replay) {
            return;
        }
        while (replay.index < replay.steps.length) {
            const step = replay.steps[replay.index];
            if (now - replay.start < stepDuration(step)) {
                return;
            }
            replay.start += stepDuration(step);
            this.finishStep(step, replay.start);
            replay.index += 1;
        }
        // let the dead fade out, then show the new view
        if ([...replay.units.values()].every((unit) => !unit.dying || now - unit.dying >= fadeMilliseconds)) {
            this.replay = null;
            this.updateInfo();
        }
    }

    finishReplay() {
        const replay = this.replay;
        if (!replay) {
            return;
        }
        for (; replay.index < replay.steps.length; replay.index += 1) {
            this.finishStep(replay.steps[replay.index], performance.now(), true);
        }
        this.replay = null;
        this.updateInfo();
    }

    // a step's result on the replayed units
    finishStep(step, now, quiet = false) {
        const units = this.replay.units;
        const unit = step.unit !== undefined ? units.get(step.unit) : null;
        if (unit && step.path && step.path.length > 0) {
            const last = step.path[step.path.length - 1];
            const before = step.path.length > 1 ? step.path[step.path.length - 2] : [unit.x, unit.y];
            unit.asset = facedAsset(unit.asset, faceTowards(before[0], before[1], last[0], last[1]));
            [unit.x, unit.y] = last;
        }
        if (unit && step.target !== undefined && units.has(step.target)) {
            const target = units.get(step.target);
            unit.asset = facedAsset(unit.asset, faceTowards(unit.x, unit.y, target.x, target.y));
            if (step.kind === "melee") {
                target.asset = facedAsset(target.asset, faceTowards(target.x, target.y, unit.x, unit.y));
            }
        }
        for (const change of step.changes || []) {
            const changed = units.get(change.unit);
            if (!changed) {
                continue;
            }
            changed.health = Math.max(0, Math.min(changed["max-health"], changed.health - change.damage));
            if (!quiet) {
                const text = change.damage > 0 ? `-${change.damage}` : `+${-change.damage}`;
                this.floaters.push({ x: changed.x, y: changed.y, text, color: change.damage > 0 ? "#ff6a50" : "#8fe06a", start: now });
            }
            if (change.killed) {
                changed.dying = now;
            }
        }
        for (const [id, dead] of units) {
            if (dead.dying && (quiet || now - dead.dying >= fadeMilliseconds)) {
                units.delete(id);
            }
        }
    }

    // where a unit stands and how it looks at time now: [x, y, asset, frame]
    pose(unit, now) {
        const replay = this.replay;
        const frames = this.assets.frameCount(unit.asset);
        const standing = unit.flying && frames > 1 ? this.frame % (frames - 1) : 0;
        if (!replay || replay.index >= replay.steps.length) {
            return [unit.x, unit.y, unit.asset, standing];
        }
        const step = replay.steps[replay.index];
        if (step.unit !== unit.id) {
            return [unit.x, unit.y, unit.asset, standing];
        }
        const progress = Math.max(0, Math.min(1, (now - replay.start) / stepDuration(step)));
        const walking = frames > 1 ? Math.floor(now / 110) % Math.max(1, frames - 1) : 0;
        if (step.kind === "move" && step.path && step.path.length > 0) {
            const position = progress * step.path.length;
            const index = Math.min(step.path.length - 1, Math.floor(position));
            const from = index === 0 ? [unit.x, unit.y] : step.path[index - 1];
            const to = step.path[index];
            const part = position - index;
            const asset = facedAsset(unit.asset, faceTowards(from[0], from[1], to[0], to[1]));
            return [from[0] + (to[0] - from[0]) * part, from[1] + (to[1] - from[1]) * part, asset, walking];
        }
        if ((step.kind === "melee" || step.kind === "ranged") && step.target !== undefined && replay.units.has(step.target)) {
            const target = replay.units.get(step.target);
            const asset = facedAsset(unit.asset, faceTowards(unit.x, unit.y, target.x, target.y));
            const attackFrames = frames >= 4 ? [2, 3] : [Math.max(0, frames - 1)];
            const frame = attackFrames[Math.floor(progress * 4) % attackFrames.length];
            if (step.kind === "melee") {
                const lunge = Math.sin(Math.PI * progress) * 0.3;
                return [unit.x + (target.x - unit.x) * lunge, unit.y + (target.y - unit.y) * lunge, asset, frame];
            }
            return [unit.x, unit.y, asset, frame];
        }
        return [unit.x, unit.y, unit.asset, standing];
    }

    // a missile while a ranged step runs, a glow on the units a spell or effect touches
    drawStepEffects(now) {
        const replay = this.replay;
        if (!replay || replay.index >= replay.steps.length) {
            return;
        }
        const step = replay.steps[replay.index];
        const progress = Math.max(0, Math.min(1, (now - replay.start) / stepDuration(step)));
        if (step.kind === "ranged" && replay.units.has(step.unit) && replay.units.has(step.target)) {
            const shooter = replay.units.get(step.unit);
            const target = replay.units.get(step.target);
            const [fromX, fromY] = this.center(shooter.x, shooter.y);
            const [toX, toY] = this.center(target.x, target.y);
            const along = Math.min(1, progress * 1.4);
            const x = fromX + (toX - fromX) * along;
            const y = fromY + (toY - fromY) * along - Math.sin(Math.PI * along) * 12 - 8;
            this.context.fillStyle = "#ffe08a";
            this.context.fillRect(Math.round(x) - 1, Math.round(y) - 1, 3, 3);
        }
        if (step.kind === "spell" || step.kind === "effect") {
            const alpha = Math.sin(Math.PI * progress) * 0.7;
            for (const change of step.changes || []) {
                const unit = replay.units.get(change.unit);
                if (unit) {
                    const rgb = change.damage > 0 ? "255, 120, 60" : "140, 240, 120";
                    this.diamond(unit.x, unit.y, `rgba(${rgb}, ${alpha})`, `rgba(${rgb}, ${alpha / 3})`, 2);
                }
            }
        }
    }

    drawFloaters(now) {
        const context = this.context;
        this.floaters = this.floaters.filter((floater) => now - floater.start < floatMilliseconds);
        context.font = "bold 8px monospace";
        context.textAlign = "center";
        for (const floater of this.floaters) {
            const age = Math.max(0, (now - floater.start) / floatMilliseconds);
            const [cx, cy] = this.center(floater.x, floater.y);
            const y = Math.round(cy - 18 - age * 14);
            context.globalAlpha = 1 - age * age;
            context.fillStyle = "#000";
            context.fillText(floater.text, Math.round(cx) + 1, y + 1);
            context.fillStyle = floater.color;
            context.fillText(floater.text, Math.round(cx), y);
            context.globalAlpha = 1;
        }
    }

    // ---- unit details and the log

    updateInfo() {
        const panel = this.root.querySelector(".battle-info");
        const view = this.view;
        const unit = view && this.hover ? this.displayUnits().find((candidate) => candidate.x === this.hover.x && candidate.y === this.hover.y && !candidate.dying) : null;
        if (!unit) {
            panel.hidden = true;
            this.infoUnit = null;
            return;
        }
        const key = `${unit.id}:${unit.health}:${unit["moves-left"]}:${view.turn}`;
        if (this.infoUnit === key && !panel.hidden) {
            return;
        }
        this.infoUnit = key;
        panel.hidden = false;
        panel.innerHTML = "";
        const line = (text, className) => {
            const element = document.createElement(className === "title" ? "h3" : "div");
            if (className && className !== "title") {
                element.className = className;
            }
            element.textContent = text;
            panel.appendChild(element);
        };
        line(unit.name + (unit.level ? ` · ${unit.level}` : ""), "title");
        line(unit.yours ? "yours" : (unit.owner ? unit.owner.wizard : "neutral"), "dim");
        line(`${unit.health}/${unit["max-health"]} hp · ${unit.figures.length}/${unit["figure-count"]} figures × ${unit["hit-points"]} hp`);
        const ranged = unit.ranged > 0 ? ` · ranged ${unit.ranged}${unit["ranged-type"] ? ` (${unit["ranged-type"]})` : ""}${unit["ranged-attacks"] ? ` ×${unit["ranged-attacks"]}` : ""}` : "";
        line(`melee ${unit.melee}${ranged}`);
        line(`defense ${unit.defense} · resist ${unit.resistance} · moves ${unit["moves-left"]}`);
        for (const [label, list] of [["abilities", unit.abilities], ["enchantments", unit.enchantments]]) {
            const names = (list || []).filter((name) => name);
            if (names.length > 0) {
                line(`${label}: ${names.join(", ")}`, "battle-info-list");
            }
        }
    }

    updateLog() {
        const box = this.root.querySelector(".battle-log");
        box.innerHTML = "";
        for (const entry of (this.view.log || []).slice(-5)) {
            const line = document.createElement("div");
            line.textContent = entry.text;
            box.appendChild(line);
        }
    }

    // send a battle command; a refusal shows on the screen (the log is behind it)
    command(type, command = {}) {
        this.message = "";
        const waiting = this.view && this.view.casting ? this.view.casting.spell : null;
        const status = this.onCommand(type, command);
        if (status && this.open) {
            if (status.error) {
                this.message = status.error;
            } else if (type === "battle-target" && waiting) {
                this.message = `${waiting} cast`;
            }
            this.updateBar();
        }
        return status;
    }

    // keys while the battle is open; true if the key was used
    key(key) {
        if (key === "Escape") {
            if (this.view && this.view.casting) {
                this.command("battle-cancel");
            } else if (this.spellsOpen) {
                this.toggleSpells();
            }
            return true;
        }
        if (key === "s" || key === "S") {
            this.toggleSpells();
            return true;
        }
        const commands = { d: "battle-done", w: "battle-wait", a: "battle-auto" };
        const type = commands[key.toLowerCase()];
        if (type) {
            this.command(type);
            return true;
        }
        return false;
    }

    toggleSpells() {
        this.spellsOpen = !this.spellsOpen && !!this.view;
        this.updateBar();
    }

    cast(spell, caster) {
        this.spellsOpen = false;
        const command = { spell: spell.name };
        if (caster !== undefined) {
            command.caster = caster;
        }
        const status = this.command("battle-cast", command);
        if (status && !status.error && this.open && !(this.view && this.view.casting)) {
            this.message = `${spell.name} cast`;
            this.updateBar();
        }
    }

    // a click on the field while a spell waits for its target
    pickTarget(tile) {
        const casting = this.view.casting;
        if (casting.kind === "tile") {
            this.command("battle-target", { x: tile.x, y: tile.y });
            return;
        }
        if (casting.kind === "unit") {
            const unit = this.view.units.find((candidate) => candidate.x === tile.x && candidate.y === tile.y && (casting.units || []).includes(candidate.id));
            if (unit) {
                this.command("battle-target", { unit: unit.id });
            } else {
                this.message = `${casting.spell} can't target that`;
                this.updateBar();
            }
        }
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
        if (!view) {
            return;
        }
        const side = view["you-attack"] ? "you attack" : "you defend";
        this.root.querySelector(".battle-title").textContent = `Battle · turn ${view.turn} · ${side}`;
        const unit = this.selectedUnit();
        this.root.querySelector(".battle-unit").textContent = unit
            ? `${unit.name}: ${unit.health}/${unit["max-health"]} hp · ${unit["moves-left"]} moves${unit["ranged-attacks"] ? ` · ${unit["ranged-attacks"]} shots` : ""}`
            : "waiting…";
        const magic = view.magic;
        this.root.querySelector(".battle-mana").textContent = magic
            ? `skill ${magic["mana-pool"]} · mana ${magic.mana}${magic.range > 1 ? ` · range ×${magic.range}` : ""}${magic.casted ? " · cast" : ""}`
            : "";
        for (const button of this.root.querySelectorAll("button[data-command='battle-done'], button[data-command='battle-wait']")) {
            button.disabled = !unit;
        }
        const anySpells = (magic && magic.spells.length > 0) || view["unit-magic"];
        this.root.querySelector(".battle-spells-button").disabled = !unit || !anySpells;
        this.updateSpells();
        this.updateHint();
        this.updateLog();
        this.updateInfo();
    }

    updateHint() {
        const view = this.view;
        const hint = this.root.querySelector(".battle-hint");
        let text;
        if (view.casting) {
            const casting = view.casting;
            const none = (casting.kind === "unit" && !(casting.units || []).length) || (casting.kind === "tile" && !(casting.tiles || []).length);
            const what = none ? "nothing here it can target" : { unit: "click a highlighted unit", tile: "click a highlighted tile", choice: "pick one above" }[casting.kind] || "";
            text = `${view.casting.spell}: ${what} · Esc cancels`;
        } else {
            text = "click a highlighted tile to move · a red one to attack · S spells · D done · W wait · A auto";
        }
        hint.textContent = this.message ? `${this.message} — ${text}` : text;
        hint.classList.toggle("battle-message", !!this.message);
    }

    // the spell list, or the units a spell with a choice offers
    updateSpells() {
        const view = this.view;
        const panel = this.root.querySelector(".battle-spells");
        panel.innerHTML = "";
        const casting = view.casting;
        if (casting && casting.kind === "choice") {
            panel.hidden = false;
            this.addHeading(panel, casting.title || casting.spell);
            const row = document.createElement("div");
            row.className = "battle-spell-list";
            for (const choice of casting.choices || []) {
                const button = document.createElement("button");
                button.className = "battle-choice";
                button.dataset.choice = choice.id;
                const picture = document.createElement("canvas");
                if (this.assets.frameCount(choice.asset) > 0) {
                    const image = this.assets.bannerFrame(choice.asset, 0, choice.banner);
                    picture.width = image.width;
                    picture.height = image.height;
                    picture.getContext("2d").drawImage(image, 0, 0);
                }
                button.append(picture, document.createTextNode(choice.name));
                button.onclick = () => this.command("battle-target", { unit: choice.id });
                row.appendChild(button);
            }
            panel.appendChild(row);
            this.addButton(panel, "Cancel", () => this.command("battle-cancel"));
            return;
        }
        if (!this.spellsOpen || casting || view.selected === undefined) {
            panel.hidden = true;
            return;
        }
        panel.hidden = false;
        const magic = view.magic;
        if (magic) {
            const note = magic.casted ? "already cast this turn" : `up to ${magic.available} mana`;
            this.addHeading(panel, `Wizard · ${note}`);
            this.addSpellList(panel, magic.spells, undefined, "no combat spells");
        }
        const unitMagic = view["unit-magic"];
        if (unitMagic) {
            const caster = view.units.find((candidate) => candidate.id === unitMagic.caster);
            this.addHeading(panel, `${caster ? caster.name : "Unit"} · skill ${unitMagic["casting-skill"]}`);
            this.addSpellList(panel, unitMagic.spells, unitMagic.caster, "no spells");
        }
        this.addButton(panel, "Close", () => this.toggleSpells());
    }

    addHeading(panel, text) {
        const heading = document.createElement("h3");
        heading.textContent = text;
        panel.appendChild(heading);
    }

    addButton(panel, label, action) {
        const button = document.createElement("button");
        button.textContent = label;
        button.onclick = action;
        panel.appendChild(button);
    }

    addSpellList(panel, spells, caster, emptyText) {
        const list = document.createElement("div");
        list.className = "battle-spell-list";
        if (spells.length === 0) {
            list.innerHTML = `<span class="dim">${emptyText}</span>`;
        }
        for (const spell of spells) {
            const button = document.createElement("button");
            button.className = "battle-spell";
            button.dataset.spell = spell.name;
            button.disabled = !spell.castable;
            button.style.color = realmColors[spell.magic] || "";
            const cost = spell.charges ? `${spell.charges} charge${spell.charges === 1 ? "" : "s"}` : `${spell.cost}`;
            button.innerHTML = `<span></span><span class="dim"></span>`;
            button.children[0].textContent = spell.name;
            button.children[1].textContent = cost;
            button.title = `${spell.magic}${button.disabled ? " · not castable now" : ""}`;
            button.onclick = () => this.cast(spell, caster);
            list.appendChild(button);
        }
        panel.appendChild(list);
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
            try {
                this.draw(now);
            } catch (error) {
                // one bad frame mustn't stop the screen
                console.error(error);
            }
            requestAnimationFrame(step);
        };
        requestAnimationFrame(step);
    }

    diamond(x, y, stroke, fill, lineWidth = 1) {
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
            context.lineWidth = lineWidth;
            context.stroke();
        }
    }

    draw(now) {
        const view = this.view;
        if (!view) {
            return;
        }
        this.advanceReplay(now);
        const replaying = !!this.replay;
        const units = this.displayUnits();
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

        // where the selected unit can go and what it can hit, or what the waiting spell accepts
        const pulse = (Math.sin(now / 160) + 1) / 2;
        const casting = replaying ? null : view.casting;
        const spellColor = `rgba(190, 120, 255, ${0.55 + pulse * 0.45})`;
        if (casting && casting.kind === "unit") {
            const accepted = new Set(casting.units || []);
            for (const unit of units) {
                if (accepted.has(unit.id)) {
                    this.diamond(unit.x, unit.y, spellColor, `rgba(190, 120, 255, ${0.2 + pulse * 0.2})`, 2);
                }
            }
        } else if (casting && casting.kind === "tile") {
            for (const [x, y] of casting.tiles || []) {
                this.diamond(x, y, spellColor, `rgba(190, 120, 255, ${0.12 + pulse * 0.12})`, 2);
            }
        }
        const moves = casting || replaying ? [] : view.moves;
        for (const [x, y] of moves) {
            this.diamond(x, y, "rgba(255, 255, 255, 0.35)", "rgba(255, 255, 255, 0.08)");
        }
        const targets = new Map(casting || replaying ? [] : view.targets.map((target) => [target.id, target.kind]));
        for (const unit of units) {
            if (targets.has(unit.id)) {
                const color = targets.get(unit.id) === "ranged" ? `rgba(255, 170, 40, ${0.5 + pulse / 2})` : `rgba(255, 50, 40, ${0.5 + pulse / 2})`;
                this.diamond(unit.x, unit.y, color, "rgba(255, 60, 40, 0.15)");
            }
        }
        const selected = replaying ? null : this.selectedUnit();
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
        for (const unit of units) {
            const pose = this.pose(unit, now);
            things.push({ depth: this.center(pose[0], pose[1])[1], order: 1, draw: () => this.drawUnit(unit, unit === selected, pulse, pose, now) });
        }
        things.sort((a, b) => a.depth - b.depth || a.order - b.order);
        for (const thing of things) {
            thing.draw();
        }
        this.drawStepEffects(now);
        this.drawFloaters(now);
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

    // pose: where and how (see pose); flying units loop their movement frames, others stand still
    drawUnit(unit, isSelected, pulse, pose, now) {
        const [x, y, asset, frame] = pose;
        const frames = this.assets.frameCount(asset);
        if (frames === 0) {
            return;
        }
        const image = this.assets.bannerFrame(asset, Math.min(frame, frames - 1), unit.banner);
        const [cx, cy] = this.center(x, y);
        const context = this.context;
        const fading = unit.dying ? Math.max(0, 1 - (now - unit.dying) / fadeMilliseconds) : 1;
        context.globalAlpha = fading;
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
                context.globalAlpha = fading;
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
        context.globalAlpha = 1;
    }
}
