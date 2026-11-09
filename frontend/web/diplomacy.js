// The diplomacy screen: the wizards you know (player view "diplomacy", core DiplomacyView), how
// they feel about you, and what you can offer them: treaties (propose-treaty), tributes of gold or
// a spell (offer-tribute), and spell trades (trade-spells, answered with answer-trade).

function escape(text) {
    return String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
}

export class DiplomacyScreen {
    constructor(root, assets, { onCommand }) {
        this.root = root;
        this.assets = assets;
        this.onCommand = onCommand;
        this.view = null;
        this.message = "";
    }

    get open() {
        return this.root.classList.contains("open");
    }

    show(view) {
        this.view = view;
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
    }

    command(type, command, success) {
        const status = this.onCommand(type, command);
        this.message = status && status.error ? status.error : success;
        if (this.open) {
            this.render();
        }
        return status;
    }

    bannerColor(banner) {
        const colors = (this.assets.manifest.bannerColors || {})[banner];
        if (!colors || !colors.length) return "";
        // the brightest shade reads best on the dark panel
        const luminance = ([red, green, blue]) => 0.3 * red + 0.59 * green + 0.11 * blue;
        const [red, green, blue] = colors.reduce((best, color) => (luminance(color) > luminance(best) ? color : best));
        return `rgb(${red}, ${green}, ${blue})`;
    }

    render() {
        const wizards = this.view.diplomacy;
        this.root.innerHTML = `
            <div class="panel">
                <div class="row"><h3>Wizards</h3><button class="small" data-close>Close</button></div>
                <div class="wizards"></div>
                <div class="magic-message">${escape(this.message)}</div>
            </div>`;
        const list = this.root.querySelector(".wizards");
        if (wizards.length === 0) {
            list.innerHTML = `<div class="dim">you haven't met another wizard yet</div>`;
        }
        for (const wizard of wizards) {
            const banner = wizard.wizard.banner;
            const name = wizard.wizard.wizard;
            const card = document.createElement("div");
            card.className = "wizard";
            card.dataset.wizard = banner;
            const relation = Math.max(-100, Math.min(100, wizard.relation));
            const treaty = { none: "no treaty", pact: "wizard's pact", alliance: "alliance", war: "at war" }[wizard.treaty] || wizard.treaty;
            const spells = wizard.spells.map((spell) => `<option>${escape(spell)}</option>`).join("");
            card.innerHTML = `
                <div class="row">
                    <h3 style="color: ${this.bannerColor(banner)}">${escape(name)}${wizard.banished ? ` <span class="dim">(banished)</span>` : ""}</h3>
                    <span class="dim">${treaty}${wizard["peace-turns"] ? ` · peace ${wizard["peace-turns"]} turns` : ""} · fame ${wizard.fame} · ${wizard.cities} cities</span>
                </div>
                <div class="mood" title="how ${escape(name)} feels about you: ${relation}">
                    <span>${escape(wizard.mood)}</span>
                    <span class="meter"><span style="left: ${(relation + 100) / 2}%"></span></span>
                </div>
                ${wizard["global-enchantments"].length ? `<div class="dim">${wizard["global-enchantments"].map(escape).join(", ")}</div>` : ""}
                <div class="actions">
                    <span>propose</span>
                    <button class="small" data-treaty="pact"${wizard.treaty === "pact" || wizard.treaty === "alliance" ? " disabled" : ""}>pact</button>
                    <button class="small" data-treaty="alliance"${wizard.treaty === "alliance" ? " disabled" : ""}>alliance</button>
                    <button class="small" data-treaty="peace"${wizard["peace-turns"] ? " disabled" : ""} title="they won't attack you for 50 turns">peace</button>
                </div>
                <div class="actions">
                    <span>tribute</span>
                    <button class="small" data-tribute-gold${wizard["tribute-gold"] ? "" : " disabled"}>${wizard["tribute-gold"] ? `${wizard["tribute-gold"]} gold` : "not enough gold"}</button>
                    <select data-spell${spells ? "" : " disabled"}>${spells || "<option>no spell they lack</option>"}</select>
                    <button class="small" data-tribute-spell${spells ? "" : " disabled"}>give</button>
                    <button class="small" data-trade${spells && wizard["can-teach"] ? "" : " disabled"} title="${wizard["can-teach"] ? "offer it for a spell of theirs" : "they know no spell you don't"}">trade</button>
                </div>
                ${wizard["trade-offer"] ? `<div class="actions offer">
                    <span>offer: your <b>${escape(wizard["trade-offer"].give)}</b> for their <b>${escape(wizard["trade-offer"].receive)}</b></span>
                    <button class="small" data-answer="yes">accept</button><button class="small" data-answer="no">decline</button>
                </div>` : ""}`;

            const spell = () => card.querySelector("[data-spell]").value;
            for (const button of card.querySelectorAll("[data-treaty]")) {
                button.onclick = () => this.command("propose-treaty", { wizard: banner, treaty: button.dataset.treaty }, `${name} agrees to the ${button.dataset.treaty}.`);
            }
            card.querySelector("[data-tribute-gold]").onclick = () =>
                this.command("offer-tribute", { wizard: banner, gold: true }, `${name} thanks you for the ${wizard["tribute-gold"]} gold.`);
            card.querySelector("[data-tribute-spell]").onclick = () => {
                const gift = spell();
                this.command("offer-tribute", { wizard: banner, spell: gift }, `${name} thanks you for ${gift}.`);
            };
            card.querySelector("[data-trade]").onclick = () => {
                const offered = spell();
                this.command("trade-spells", { wizard: banner, spell: offered }, `You offer ${offered} to ${name}.`);
            };
            for (const button of card.querySelectorAll("[data-answer]")) {
                const offer = wizard["trade-offer"];
                const accept = button.dataset.answer === "yes";
                button.onclick = () => this.command("answer-trade", { accept }, accept ? `You learned ${offer.receive}.` : "You declined the trade.");
            }
            list.appendChild(card);
        }
        this.root.querySelector("[data-close]").onclick = () => this.hide();
    }
}
