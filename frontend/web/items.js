// The item designer for Create Artifact and Enchant Item: pick the item type and picture, a name,
// powers (one per group; abilities: one each from Ability 1 and 2, any from Ability 3) and, for
// staffs and wands, a spell with charges. The options and prices come from the player view's
// "item-design" (core ItemDesignView); the core checks the design again on cast-spell.

function escape(text) {
    return String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
}

const singleGroups = ["Attack", "To Hit", "Defense", "Movement", "Resistance", "Spell Skill", "Spell Save", "Ability 1", "Ability 2"];

export class ItemDesigner {
    // spell: the SpellView of Create Artifact or Enchant Item; options: you["item-design"]
    constructor(assets, spell, options) {
        this.assets = assets;
        this.spell = spell;
        this.options = options;
        this.enchantItem = spell.name === "Enchant Item";
        this.type = options.types[0].name;
        this.image = options.types[0].images[0].image;
        this.name = "";
        // group -> power name, for the single groups; a set for Ability 3
        this.single = new Map();
        this.many = new Set();
        this.chargeSpell = "";
        this.charges = 1;
    }

    typeView() {
        return this.options.types.find((kind) => kind.name === this.type);
    }

    // why a power can't go on this item now, or ""
    blocked(power) {
        if (!power.types.includes(this.type)) {
            return `a ${this.type} can't have it`;
        }
        if (!power.allowed) {
            return `needs ${power.books} ${power.magic} spellbooks`;
        }
        if (this.enchantItem && !power.ability && power.cost > this.options["enchant-item-limit"]) {
            return "too strong for Enchant Item";
        }
        return "";
    }

    powers() {
        return [...this.single.values(), ...this.many];
    }

    holdsSpells() {
        return this.type === "Staff" || this.type === "Wand";
    }

    setType(name) {
        this.type = name;
        this.image = this.typeView().images[0].image;
        // drop what the new type can't have
        for (const [group, powerName] of this.single) {
            const power = this.options.powers.find((candidate) => candidate.name === powerName);
            if (!power || this.blocked(power)) this.single.delete(group);
        }
        for (const powerName of [...this.many]) {
            const power = this.options.powers.find((candidate) => candidate.name === powerName);
            if (!power || this.blocked(power)) this.many.delete(powerName);
        }
        if (!this.holdsSpells()) this.chargeSpell = "";
    }

    // the item's cost before the spell's cost factor, as core artifact.CalculateCost
    itemCost() {
        const kind = this.typeView();
        let powerCost = 0;
        for (const name of this.powers()) {
            const power = this.options.powers.find((candidate) => candidate.name === name);
            if (power) powerCost += power.cost;
        }
        let spellCost = 0;
        if (this.chargeSpell) {
            const spell = this.options.spells.find((candidate) => candidate.name === this.chargeSpell);
            if (spell) spellCost = spell["charge-cost"] * this.charges;
        }
        return kind["base-cost"] + powerCost * kind["power-factor"] + spellCost;
    }

    cost() {
        return Math.floor(this.itemCost() * (this.spell["cost-factor"] || 1));
    }

    design() {
        const design = { type: this.type, image: this.image, powers: this.powers() };
        if (this.name.trim()) design.name = this.name.trim();
        if (this.chargeSpell) {
            design.spell = this.chargeSpell;
            design.charges = this.charges;
        }
        return design;
    }

    picture(imageId, scale = 2) {
        const canvas = document.createElement("canvas");
        const [width, height] = this.assets.size(imageId);
        canvas.width = Math.max(1, width);
        canvas.height = Math.max(1, height);
        canvas.style.width = `${canvas.width * scale}px`;
        canvas.style.height = `${canvas.height * scale}px`;
        canvas.style.imageRendering = "pixelated";
        this.assets.draw(canvas.getContext("2d"), imageId, 0, 0);
        return canvas;
    }

    // draw into container; you is the player view's "you" (mana and skill for "cast now")
    render(container, you, { onCreate, onBack }) {
        const count = this.powers().length;
        const full = count >= this.options["max-powers"];
        const cost = this.cost();
        const now = cost <= you.mana && cost <= you["remaining-casting-skill"];
        const kind = this.typeView();

        const groups = new Map();
        for (const power of this.options.powers) {
            if (!groups.has(power.group)) groups.set(power.group, []);
            groups.get(power.group).push(power);
        }
        const order = [...singleGroups.filter((group) => groups.has(group)), ...[...groups.keys()].filter((group) => !singleGroups.includes(group))];

        container.innerHTML = `
            <div class="row"><h3>${escape(this.spell.name)}</h3><button class="small" data-back>Back</button></div>
            <div class="item-types">${this.options.types.map((type) => `<button class="small${type.name === this.type ? " chosen" : ""}" data-type="${escape(type.name)}">${escape(type.name)}</button>`).join("")}</div>
            <div class="item-images"></div>
            <label class="row item-name"><span>name</span><input type="text" maxlength="30" placeholder="(generated)" value="${escape(this.name)}" data-name></label>
            <div class="dim">powers ${count}/${this.options["max-powers"]}${this.enchantItem ? ` · Enchant Item: powers up to ${this.options["enchant-item-limit"]}` : ""}${kind["power-factor"] > 1 ? ` · jewelry: powers cost ${kind["power-factor"]}×` : ""}</div>
            <div class="item-powers"></div>
            <div class="item-charges"></div>
            <div class="row gap">
                <b>cost ${cost}</b>
                <span class="dim">${now ? "cast now" : "over the next turns"}</span>
                <button data-create>${now ? "Create" : "Start casting"}</button>
            </div>`;

        const images = container.querySelector(".item-images");
        for (const image of kind.images) {
            const button = document.createElement("button");
            button.className = `item-image${image.image === this.image ? " chosen" : ""}`;
            button.dataset.image = image.image;
            button.title = `picture ${image.image}`;
            button.appendChild(this.picture(image.asset));
            button.onclick = () => { this.image = image.image; this.render(container, you, { onCreate, onBack }); };
            images.appendChild(button);
        }

        const powersBox = container.querySelector(".item-powers");
        for (const group of order) {
            const powers = groups.get(group).filter((power) => power.types.includes(this.type));
            if (powers.length === 0) continue;
            const row = document.createElement("div");
            row.className = "item-group";
            const label = document.createElement("span");
            label.textContent = group === "Ability 3" ? "Abilities" : group;
            row.appendChild(label);
            if (group === "Ability 3") {
                const list = document.createElement("div");
                list.className = "item-many";
                for (const power of powers) {
                    const reason = this.blocked(power);
                    const checked = this.many.has(power.name);
                    const option = document.createElement("label");
                    option.title = reason || `${power.magic} · ${power.books} books`;
                    option.innerHTML = `<input type="checkbox" data-many="${escape(power.name)}"${checked ? " checked" : ""}${reason || (full && !checked) ? " disabled" : ""}> ${escape(power.name)} <span class="dim">${power.cost}</span>`;
                    list.appendChild(option);
                }
                row.appendChild(list);
            } else {
                const select = document.createElement("select");
                select.dataset.group = group;
                const current = this.single.get(group) || "";
                select.innerHTML = `<option value="">—</option>` + powers.map((power) => {
                    const reason = this.blocked(power);
                    const disabled = reason || (full && !current);
                    return `<option value="${escape(power.name)}"${power.name === current ? " selected" : ""}${disabled ? " disabled" : ""}>${escape(power.name)} (${power.cost})${reason ? ` – ${escape(reason)}` : ""}</option>`;
                }).join("");
                row.appendChild(select);
            }
            powersBox.appendChild(row);
        }

        const charges = container.querySelector(".item-charges");
        if (this.holdsSpells()) {
            charges.innerHTML = `
                <div class="item-group"><span>spell</span>
                    <select data-charge-spell><option value="">—</option>${this.options.spells.map((spell) => `<option value="${escape(spell.name)}"${spell.name === this.chargeSpell ? " selected" : ""}>${escape(spell.name)} (${spell["charge-cost"]} each)</option>`).join("")}</select>
                    <select data-charges${this.chargeSpell ? "" : " disabled"}>${[1, 2, 3, 4].map((amount) => `<option value="${amount}"${amount === this.charges ? " selected" : ""}>${amount} charge${amount > 1 ? "s" : ""}</option>`).join("")}</select>
                </div>`;
        }

        const redraw = () => this.render(container, you, { onCreate, onBack });
        container.querySelector("[data-back]").onclick = onBack;
        container.querySelector("[data-create]").onclick = () => onCreate(this.design());
        container.querySelector("[data-name]").oninput = (event) => { this.name = event.target.value; };
        for (const button of container.querySelectorAll("button[data-type]")) {
            button.onclick = () => { this.setType(button.dataset.type); redraw(); };
        }
        for (const select of container.querySelectorAll("select[data-group]")) {
            select.onchange = () => {
                if (select.value) this.single.set(select.dataset.group, select.value);
                else this.single.delete(select.dataset.group);
                redraw();
            };
        }
        for (const box of container.querySelectorAll("input[data-many]")) {
            box.onchange = () => {
                if (box.checked) this.many.add(box.dataset.many);
                else this.many.delete(box.dataset.many);
                redraw();
            };
        }
        const chargeSpell = container.querySelector("[data-charge-spell]");
        if (chargeSpell) {
            chargeSpell.onchange = () => { this.chargeSpell = chargeSpell.value; redraw(); };
            container.querySelector("[data-charges]").onchange = (event) => { this.charges = Number(event.target.value); redraw(); };
        }
    }
}
