// The magic screen: how power is shared between mana, research and casting skill
// (set-power-distribution), alchemy, the spell being cast, and the spellbook of overland spells
// (cast-spell). Everything comes from the player view's "you" (core WizardView).
//
// Picking a spell: one that needs no target is cast (now, or started over the next turns); one
// that needs a target and can be cast now asks the page to pick it (onPickTarget), one that can't
// be cast now is started without a target: the core asks for it when it's finished
// (choose-spell-target).
//
// Spells cast with a choice (Spell Ward, Spell Binding, Disjunction, Resurrection) list what they
// can be cast on now in the view (spell.choices); the page asks which (onPickChoice). Create
// Artifact and Enchant Item open the item designer (items.js) in place of the spellbook.

import { ItemDesigner } from "./items.js";

const realmColors = {
    Life: "#f4f0d8", Death: "#b07ae0", Chaos: "#ff7a50", Nature: "#7ed06a", Sorcery: "#6ab4ff", Arcane: "#c8c0b0",
};

// targets the page can pick: a tile on the map (or a stack on it), or a wizard
const pickableTargets = new Set([
    "friendly-city", "enemy-city", "any", "land", "empty-water", "change-terrain", "transmute", "raise-volcano",
    "enemy-melded-node", "disenchant", "friendly-unit", "enemy-unit", "wizard",
]);

function escape(text) {
    return String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
}

export class MagicScreen {
    constructor(root, assets, { onCommand, onPickTarget, onPickChoice }) {
        this.root = root;
        this.assets = assets;
        this.onCommand = onCommand;
        this.onPickTarget = onPickTarget;
        this.onPickChoice = onPickChoice;
        this.view = null;
        this.message = "";
        // the item designer while it's open
        this.designer = null;
    }

    get open() {
        return this.root.classList.contains("open");
    }

    show(view) {
        this.view = view;
        this.root.classList.add("open");
        this.render();
    }

    // redraw from a new view while open
    update(view) {
        this.view = view;
        if (this.open) {
            this.render();
        }
    }

    hide() {
        this.root.classList.remove("open");
        this.message = "";
        this.designer = null;
    }

    command(type, command) {
        const status = this.onCommand(type, command);
        this.message = status && status.error ? status.error : "";
        if (this.open) {
            this.render();
        }
        return status;
    }

    // keep the three shares adding up to 1: the changed one takes its value, the other two share
    // the rest in their old proportion
    setShare(changed, value) {
        const current = { ...this.view.you["power-distribution"] };
        const others = ["mana", "research", "skill"].filter((name) => name !== changed);
        const rest = Math.max(0, 1 - value);
        const otherTotal = current[others[0]] + current[others[1]];
        current[changed] = value;
        if (otherTotal > 0) {
            current[others[0]] = rest * current[others[0]] / otherTotal;
            current[others[1]] = rest - current[others[0]];
        } else {
            current[others[0]] = rest / 2;
            current[others[1]] = rest / 2;
        }
        this.command("set-power-distribution", current);
    }

    cast(spell) {
        if (spell["needs-design"]) {
            this.designer = new ItemDesigner(this.assets, spell, this.view.you["item-design"]);
            this.message = "";
            this.render();
            return;
        }
        if (spell["choice-spell"] && spell.now) {
            this.hide();
            this.onPickChoice(spell);
            return;
        }
        if (spell.target && spell.now && !spell["choice-spell"]) {
            this.hide();
            this.onPickTarget(spell);
            return;
        }
        const status = this.command("cast-spell", { spell: spell.name });
        if (status && !status.error) {
            this.message = spell.now ? `${spell.name} cast.` : `Casting ${spell.name} over the next turns.`;
        }
        this.render();
    }

    createItem(design) {
        const spell = this.designer.spell;
        const now = this.designer.cost() <= this.view.you.mana && this.designer.cost() <= this.view.you["remaining-casting-skill"];
        const status = this.onCommand("cast-spell", { spell: spell.name, artifact: design });
        if (status && status.error) {
            this.message = status.error;
        } else {
            this.designer = null;
            this.message = now ? `${spell.name}: the item is made.` : `Casting ${spell.name} over the next turns.`;
        }
        if (this.open) {
            this.render();
        }
    }

    render() {
        if (this.designer) {
            this.root.innerHTML = `<div class="panel designer"><div class="design"></div><div class="magic-message">${escape(this.message)}</div></div>`;
            this.designer.render(this.root.querySelector(".design"), this.view.you, {
                onCreate: (design) => this.createItem(design),
                onBack: () => { this.designer = null; this.message = ""; this.render(); },
            });
            return;
        }
        const you = this.view.you;
        const share = you["power-distribution"];
        const percent = (value) => Math.round(value * 100);
        const casting = you.casting
            ? `${escape(you.casting)} · ${you["casting-progress"]}/${you["casting-cost"]}`
            : "nothing";

        this.root.innerHTML = `
            <div class="panel">
                <div class="row"><h3>Magic</h3><button class="small" data-close>Close</button></div>
                <div class="columns">
                    <div>
                        <div class="row"><span>power</span><b>${you.power} per turn</b></div>
                        ${[["mana", "mana", you["mana-per-turn"]], ["research", "research", you["research-per-turn"]], ["skill", "casting skill", you["skill-per-turn"]]].map(([name, label, perTurn]) => `
                            <label class="share">
                                <span>${label} <b>${percent(share[name])}%</b></span>
                                <input type="range" min="0" max="100" step="5" value="${percent(share[name])}" data-share="${name}">
                                <span class="dim">+${perTurn}/turn</span>
                            </label>`).join("")}
                        <div class="gap row"><span>mana</span><b>${you.mana}</b></div>
                        <div class="row"><span>casting skill</span><b>${you["remaining-casting-skill"]} of ${you["casting-skill"]} left this turn</b></div>
                        <div class="row"><span>casting</span><b>${casting}</b></div>
                        <h3 class="gap">Alchemy</h3>
                        <div class="dim">${you["alchemy-full-rate"] ? "full rate (Alchemy retort)" : "half rate: 2 for 1"}</div>
                        <div class="row alchemy">
                            <input type="number" min="1" value="10" data-alchemy-amount>
                            <button class="small" data-alchemy="gold-to-mana">gold → mana</button>
                            <button class="small" data-alchemy="mana-to-gold">mana → gold</button>
                        </div>
                    </div>
                    <div class="spellbook"></div>
                </div>
                <div class="magic-message">${escape(this.message)}</div>
            </div>`;

        const book = this.root.querySelector(".spellbook");
        const sections = new Map();
        for (const spell of you.spells) {
            if (!sections.has(spell.section)) {
                sections.set(spell.section, []);
            }
            sections.get(spell.section).push(spell);
        }
        if (you.spells.length === 0) {
            book.innerHTML = `<h3>Spellbook</h3><div class="dim">no overland spells</div>`;
        }
        for (const [section, spells] of sections) {
            const heading = document.createElement("h3");
            heading.textContent = section;
            book.appendChild(heading);
            const list = document.createElement("div");
            list.className = "spell-list";
            for (const spell of spells) {
                const button = document.createElement("button");
                button.className = "spell";
                button.dataset.spell = spell.name;
                button.style.color = realmColors[spell.magic] || "";
                const blocked = spell["needs-design"] && !you["item-design"] ? "the item powers aren't available"
                    : spell["choice-spell"] ? (spell.choices && spell.choices.length ? "" : "nothing to cast it on now")
                    : spell.target && !pickableTargets.has(spell.target) ? `needs a ${spell.target} target (not in this screen yet)` : "";
                button.disabled = !!blocked;
                const note = spell.now ? `${spell.cost}` : `${spell.cost} · over turns`;
                button.title = blocked || `${spell.magic}${spell.target ? ` · target: ${spell.target}` : ""}${spell.now ? " · cast now" : " · not enough mana or skill this turn: cast over the next turns"}`;
                button.innerHTML = `<span></span><span class="dim"></span>`;
                button.children[0].textContent = spell.name;
                button.children[1].textContent = note;
                button.onclick = () => this.cast(spell);
                list.appendChild(button);
            }
            book.appendChild(list);
        }

        this.root.querySelector("[data-close]").onclick = () => this.hide();
        for (const input of this.root.querySelectorAll("input[data-share]")) {
            input.onchange = () => this.setShare(input.dataset.share, Number(input.value) / 100);
        }
        for (const button of this.root.querySelectorAll("button[data-alchemy]")) {
            button.onclick = () => {
                const amount = Number(this.root.querySelector("[data-alchemy-amount]").value);
                const status = this.command("alchemy", { direction: button.dataset.alchemy, amount });
                if (status && !status.error) {
                    this.message = `Converted ${amount}.`;
                }
                this.render();
            };
        }
    }
}
