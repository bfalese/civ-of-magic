// The city and army lists (the desktop's citylistview and armyview), from the player view. Cities:
// size, what each builds and when it finishes, food / gold / production; click one to open its
// city screen. Armies: every stack with its leader, units, moves and standing orders, the heroes,
// and the upkeep; click one to select it and centre the map on it.

function escape(text) {
    return String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
}

const signed = (value) => (value > 0 ? `+${value}` : `${value}`);
const round = (value) => Math.round(value * 10) / 10;
const where = (location) => `${location.x},${location.y}${location.plane === "Myrror" ? " M" : ""}`;

// turns until the city's production is done: a number, "never", or "–" when it never ends
function turnsLeft(city) {
    // Housing and Trade Goods never finish
    if (!city.producing || city["production-cost"] <= 0) return "–";
    const remaining = Math.max(0, city["production-cost"] - city.production);
    if (remaining === 0) return "1";
    return city["production-per-turn"] > 0 ? String(Math.ceil(remaining / city["production-per-turn"])) : "never";
}

export class ListScreen {
    constructor(root, assets, { onOpenCity, onSelectStack }) {
        this.root = root;
        this.assets = assets;
        this.onOpenCity = onOpenCity;
        this.onSelectStack = onSelectStack;
        this.view = null;
        // "cities" or "armies"
        this.tab = "cities";
    }

    get open() {
        return this.root.classList.contains("open");
    }

    show(view, tab) {
        this.view = view;
        this.tab = tab;
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

    sprite(background, asset) {
        const canvas = document.createElement("canvas");
        const [width, height] = this.assets.size(background || asset);
        canvas.width = Math.max(1, width);
        canvas.height = Math.max(1, height);
        const context = canvas.getContext("2d");
        if (background) {
            this.assets.draw(context, background, 0, 0);
            this.assets.draw(context, asset, 1, 1);
        } else {
            this.assets.draw(context, asset, 0, 0);
        }
        return canvas;
    }

    render() {
        const cities = this.view.cities;
        const stacks = this.view.stacks.filter((stack) => stack.units.length > 0);
        this.root.innerHTML = `
            <div class="panel">
                <div class="row">
                    <span class="tabs">
                        <button class="small${this.tab === "cities" ? " chosen" : ""}" data-tab="cities" title="C">Cities (${cities.length})</button>
                        <button class="small${this.tab === "armies" ? " chosen" : ""}" data-tab="armies" title="A">Armies (${stacks.length})</button>
                    </span>
                    <button class="small" data-close title="Esc">Close</button>
                </div>
                <div class="list-body"></div>
            </div>`;
        const body = this.root.querySelector(".list-body");
        if (this.tab === "cities") {
            this.renderCities(body, cities);
        } else {
            this.renderArmies(body, stacks);
        }
        for (const button of this.root.querySelectorAll("[data-tab]")) {
            button.onclick = () => { this.tab = button.dataset.tab; this.render(); };
        }
        this.root.querySelector("[data-close]").onclick = () => this.hide();
        this.root.onclick = (event) => { if (event.target === this.root) this.hide(); };
    }

    renderCities(body, cities) {
        if (cities.length === 0) {
            body.innerHTML = `<div class="dim">no cities</div>`;
            return;
        }
        const total = (field) => round(cities.reduce((sum, city) => sum + city[field], 0));
        body.innerHTML = `<table class="list">
            <thead><tr><th>city</th><th>race</th><th class="number" title="citizens / maximum">size</th><th>producing</th><th class="number">turns</th>
                <th class="number">food</th><th class="number">gold</th><th class="number">prod.</th></tr></thead>
            <tbody>${cities.map((city, index) => `<tr data-city="${index}" title="open ${escape(city.name)}">
                <td>${escape(city.name)} <span class="dim">${where(city.location)}</span></td>
                <td class="dim">${escape(city.race)}</td>
                <td class="number">${city.outpost ? `<span class="dim">outpost</span>` : `${city.citizens}<span class="dim">/${city["maximum-size"]}</span>`}</td>
                <td>${escape(city.producing || "nothing")}</td>
                <td class="number">${turnsLeft(city)}</td>
                <td class="number${city["food-surplus"] < 0 ? " bad" : ""}">${signed(city["food-surplus"])}</td>
                <td class="number${city["gold-surplus"] < 0 ? " bad" : ""}">${signed(city["gold-surplus"])}</td>
                <td class="number">${round(city["production-per-turn"])}</td>
            </tr>`).join("")}</tbody>
            <tfoot><tr><td colspan="5" class="dim">total</td><td class="number">${signed(total("food-surplus"))}</td><td class="number">${signed(total("gold-surplus"))}</td><td class="number">${total("production-per-turn")}</td></tr></tfoot>
        </table>`;
        for (const row of body.querySelectorAll("tr[data-city]")) {
            row.onclick = () => {
                const city = cities[Number(row.dataset.city)];
                this.hide();
                this.onOpenCity(city.location);
            };
        }
    }

    renderArmies(body, stacks) {
        const units = stacks.flatMap((stack) => stack.units);
        const upkeep = (kind) => units.reduce((sum, unit) => sum + (unit.details[`upkeep-${kind}`] || 0), 0);
        const heroes = units.filter((unit) => unit.hero);
        body.innerHTML = `
            <div class="dim">upkeep: ${upkeep("gold")} gold · ${upkeep("food")} food · ${upkeep("mana")} mana · ${units.length} units</div>
            ${heroes.length ? `<div class="list-heroes">heroes: ${heroes.map((hero) => `<b>${escape(hero.name)}</b> <span class="dim">${escape(hero.details.level || "")}</span>`).join(" · ")}</div>` : ""}
            ${stacks.length ? `<table class="list">
                <thead><tr><th></th><th>where</th><th>units</th><th class="number">moves</th><th>orders</th></tr></thead>
                <tbody>${stacks.map((stack, index) => {
                    const moves = Math.max(...stack.units.map((unit) => unit["moves-left"]));
                    const busy = [...new Set(stack.units.map((unit) => unit.busy).filter(Boolean))].join(", ") || (stack.path && stack.path.length ? "walking" : "");
                    return `<tr data-stack="${index}" title="select and show on the map">
                        <td class="sprite"></td>
                        <td class="dim">${where(stack.location)}</td>
                        <td>${stack.units.map((unit) => `${unit.hero ? "<b>" : ""}${escape(unit.name)}${unit.hero ? "</b>" : ""}`).join(", ")}</td>
                        <td class="number">${round(moves)}</td>
                        <td class="dim">${escape(busy)}</td>
                    </tr>`;
                }).join("")}</tbody>
            </table>` : `<div class="dim">no armies</div>`}`;
        for (const row of body.querySelectorAll("tr[data-stack]")) {
            const stack = stacks[Number(row.dataset.stack)];
            row.querySelector(".sprite").appendChild(this.sprite(stack.background, stack.units[0].asset));
            row.onclick = () => {
                this.hide();
                this.onSelectStack(stack);
            };
        }
    }
}
