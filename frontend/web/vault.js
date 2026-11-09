// The items screen: the vault, newly arrived items and the heroes' equipment, from the player
// view's you.items (core VaultView). Click an item to pick it up, then click another slot to
// swap them (move-item); Destroy turns the picked item into mana (destroy-item). An item that
// arrives at a hero away from where it was costs teleport-cost mana.

function escape(text) {
    return String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
}

const slotLabels = {
    "melee-weapon": "melee weapon", "ranged-weapon": "weapon or bow", "magic-weapon": "staff or wand",
    "any-weapon": "weapon or staff", "armor": "armor", "jewelry": "jewelry",
};

const sameSlot = (a, b) => JSON.stringify(a) === JSON.stringify(b);

export class VaultScreen {
    constructor(root, assets, { onCommand }) {
        this.root = root;
        this.assets = assets;
        this.onCommand = onCommand;
        this.view = null;
        // the slot of the item picked up, e.g. {vault: 0}
        this.picked = null;
        this.message = "";
        // the index of the hero whose name is being edited, or null
        this.renaming = null;
    }

    get open() {
        return this.root.classList.contains("open");
    }

    show(view) {
        this.view = view;
        this.picked = null;
        this.message = "";
        this.root.classList.add("open");
        this.render();
    }

    update(view) {
        this.view = view;
        if (this.open) {
            this.render();
        }
    }

    hide() {
        this.root.classList.remove("open");
        this.picked = null;
        this.renaming = null;
    }

    itemAt(slot) {
        const items = this.view.you.items;
        if (slot.vault !== undefined) return items.vault[slot.vault];
        if (slot.new !== undefined) return items.new[slot.new];
        const hero = items.heroes.find((hero) => hero.index === slot.hero);
        return hero ? hero.slots[slot.slot].item : null;
    }

    command(type, command) {
        const status = this.onCommand(type, command);
        this.message = status && status.error ? status.error : "";
        return status;
    }

    clickSlot(slot) {
        if (!this.picked) {
            if (this.itemAt(slot)) {
                this.picked = slot;
                this.message = "";
            }
        } else if (sameSlot(this.picked, slot)) {
            this.picked = null;
        } else {
            const name = this.itemAt(this.picked).name;
            const status = this.command("move-item", { from: this.picked, to: slot });
            if (!status.error) {
                this.message = `Moved ${name}.`;
                this.picked = null;
            }
        }
        this.render();
    }

    destroy() {
        const item = this.itemAt(this.picked);
        const status = this.command("destroy-item", { item: this.picked });
        if (!status.error) {
            this.message = `Destroyed ${item.name} for ${item.value} mana.`;
            this.picked = null;
        }
        this.render();
    }

    // the name editor of a hero: Enter or Save renames (rename-hero), Esc or Cancel leaves it
    bindRename(box, hero) {
        const input = box.querySelector("input");
        const close = () => { this.renaming = null; this.render(); };
        const save = () => {
            const name = input.value.trim();
            this.renaming = null;
            if (name && name !== hero.name) {
                const status = this.command("rename-hero", { hero: hero.index, name });
                if (!status.error) this.message = `${hero.name} is now ${name}.`;
            }
            this.render();
        };
        box.querySelector("[data-save]").onclick = save;
        box.querySelector("[data-cancel]").onclick = close;
        input.onkeydown = (event) => {
            event.stopPropagation();
            if (event.key === "Enter") save();
            if (event.key === "Escape") close();
        };
    }

    picture(asset) {
        const canvas = document.createElement("canvas");
        const [width, height] = this.assets.size(asset);
        canvas.width = Math.max(1, width);
        canvas.height = Math.max(1, height);
        canvas.style.width = `${canvas.width * 2}px`;
        canvas.style.height = `${canvas.height * 2}px`;
        this.assets.draw(canvas.getContext("2d"), asset, 0, 0);
        return canvas;
    }

    // a slot button: the item's picture, or the kind of slot when empty
    slotButton(slot, item, emptyLabel, fits = true) {
        const button = document.createElement("button");
        button.className = "item-slot";
        button.dataset.slot = JSON.stringify(slot);
        if (this.picked && sameSlot(this.picked, slot)) button.classList.add("chosen");
        if (this.picked && !fits) button.classList.add("unfit");
        if (item) {
            button.appendChild(this.picture(item.asset));
            const label = document.createElement("span");
            label.textContent = item.name;
            button.appendChild(label);
            button.title = `${item.name} (${item.type})\n${item.powers.join(", ") || "no powers"}\ncost ${item.cost} · destroy for ${item.value} mana`;
        } else {
            button.classList.add("empty");
            button.textContent = emptyLabel;
        }
        button.onclick = () => this.clickSlot(slot);
        return button;
    }

    render() {
        const items = this.view.you.items;
        const picked = this.picked ? this.itemAt(this.picked) : null;
        this.root.innerHTML = `
            <div class="panel">
                <div class="row"><h3>Items</h3><button class="small" data-close>Close</button></div>
                <div class="dim">click an item, then a slot to move it there (swapping) · ${items["teleport-cost"]} mana for each item sent to a hero away from it</div>
                <div class="vault-new"></div>
                <h3 class="gap">Vault</h3>
                <div class="vault-slots vault-main"></div>
                <h3 class="gap">Heroes</h3>
                <div class="vault-heroes"></div>
                <div class="row gap">
                    <span class="picked">${picked ? `holding <b>${escape(picked.name)}</b>: ${escape(picked.powers.join(", ") || "no powers")}` : ""}</span>
                    <button data-destroy${picked ? "" : " disabled"}>Destroy${picked ? ` (+${picked.value} mana)` : ""}</button>
                </div>
                <div class="magic-message">${escape(this.message)}</div>
            </div>`;

        if (items.new.length) {
            const box = this.root.querySelector(".vault-new");
            box.innerHTML = `<h3 class="gap">New items</h3><div class="dim">place each in an empty slot, or destroy it</div>`;
            const row = document.createElement("div");
            row.className = "vault-slots";
            items.new.forEach((item, index) => row.appendChild(this.slotButton({ new: index }, item, "")));
            box.appendChild(row);
        }

        const vault = this.root.querySelector(".vault-main");
        items.vault.forEach((item, index) => vault.appendChild(this.slotButton({ vault: index }, item, "empty")));

        const heroes = this.root.querySelector(".vault-heroes");
        if (items.heroes.length === 0) {
            heroes.innerHTML = `<div class="dim">no heroes</div>`;
        }
        for (const hero of items.heroes) {
            const row = document.createElement("div");
            row.className = "vault-hero";
            const name = document.createElement("div");
            const about = `<span class="dim">the ${escape(hero.title)} · ${escape(hero.level)} · ${hero.location.x},${hero.location.y}${items.fortress && JSON.stringify(items.fortress) !== JSON.stringify(hero.location) ? " (away)" : ""}</span>`;
            if (this.renaming === hero.index) {
                name.className = "rename";
                name.innerHTML = `<input maxlength="20" value="${escape(hero.name)}"><button class="small" data-save>Save</button><button class="small" data-cancel>Cancel</button> ${about}`;
                this.bindRename(name, hero);
            } else {
                name.innerHTML = `<b>${escape(hero.name)}</b> <button class="tiny" data-rename title="rename">✎</button> ${about}`;
                name.querySelector("[data-rename]").onclick = () => { this.renaming = hero.index; this.render(); };
            }
            row.appendChild(name);
            const slots = document.createElement("div");
            slots.className = "vault-slots";
            hero.slots.forEach((slot, index) => {
                const fits = !picked || slot.takes.includes(picked.type);
                slots.appendChild(this.slotButton({ hero: hero.index, slot: index }, slot.item, slotLabels[slot.kind] || slot.kind, fits));
            });
            row.appendChild(slots);
            heroes.appendChild(row);
        }

        this.root.querySelector("[data-close]").onclick = () => this.hide();
        const input = this.root.querySelector(".rename input");
        if (input) {
            input.focus();
            input.select();
        }
        this.root.querySelector("[data-destroy]").onclick = () => this.destroy();
    }
}
