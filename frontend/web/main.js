// The web frontend's game screen: loads the core and the sprites, starts a game, and plays it
// through commands. The core owns the rules; this file only shows the views and asks the player.

import { loadCore } from "./core.js";
import { loadAssets } from "./assets.js";
import { MapRenderer } from "./map.js";
import { BattleScreen } from "./battle.js";
import { MagicScreen } from "./magic.js";
import { VaultScreen } from "./vault.js";

const element = (id) => document.getElementById(id);

const state = {
    core: null,
    map: null,
    view: null,
    plane: "Arcanus",
    // the selected stack is followed by its first unit's id (stacks move; ids don't)
    selectedUnit: null,
    // a spell waiting for its target on the map: {spell, target, start} (start: cast-spell from
    // the spellbook; otherwise a finished spell's cast-spell-target)
    spellTarget: null,
    questions: [],
    modalOpen: false,
    // the location of the city whose screen is open, or null
    cityOpen: null,
    battle: null,
    magic: null,
    vault: null,
    // the unit whose details the stack panel shows
    detailUnit: null,
    lastDate: null,
};

// ---- the log

function log(text, className) {
    const line = document.createElement("div");
    line.textContent = text;
    if (className) {
        line.className = className;
    }
    const panel = element("log");
    panel.prepend(line);
    while (panel.childElementCount > 200) {
        panel.lastChild.remove();
    }
}

const mine = (player) => player && player.human;

// a log line for an event, or null for events that don't concern the player
function describe(event) {
    const body = event.event || {};
    if (body.player && !body.player.human && event.type !== "random-event") {
        return null;
    }
    switch (event.type) {
        case "notice": return body.message;
        case "scroll": return body.title ? `${body.title}: ${body.text}` : body.text;
        case "random-event": return body.message;
        case "diplomacy": return `You meet ${body.enemy.wizard}.`;
        case "learned-spell": return `You learned ${body.spell}.`;
        case "new-building": return `${body.city.name} built ${body.building}.`;
        case "new-outpost": return `New outpost: ${body.city.name}.`;
        case "treasure": return `Treasure: ${body.items.map((item) => item.text).join(", ")}.`;
        case "vault": return `${body.artifact ? body.artifact.name : "An item"} went to your vault: place it in Items (I).`;
        case "hero-level-up": return `${body.hero.name} gained a level.`;
        case "banish": return `${body.defender.wizard} was banished by ${body.attacker.wizard}.`;
        case "battle-started":
            if (mine(body.defender) && !mine(body.attacker)) {
                return `${body.attacker ? body.attacker.wizard : "Monsters"} attack you at ${body.location.x},${body.location.y}!`;
            }
            return null;
        case "battle-ended": {
            if (!mine(body.attacker) && !mine(body.defender)) return null;
            const youAttacked = mine(body.attacker);
            const outcome = {
                "attacker-won": youAttacked ? "You won" : "You lost",
                "defender-won": youAttacked ? "You lost" : "You won",
                "attacker-fled": youAttacked ? "You fled" : "The enemy fled",
                "defender-fled": youAttacked ? "The enemy fled" : "You fled",
            }[body.result] || body.result;
            return `${outcome} the battle at ${body.location.x},${body.location.y}.`;
        }
    }
    return null;
}

// ---- dialogs

function ask(text, choices) {
    state.questions.push({ text, choices });
    showNextQuestion();
}

function showNextQuestion() {
    if (state.modalOpen || state.questions.length === 0) {
        return;
    }
    const { text, choices } = state.questions.shift();
    state.modalOpen = true;
    element("modalText").textContent = text;
    const box = element("modalChoices");
    box.innerHTML = "";
    for (const [label, action] of choices) {
        const button = document.createElement("button");
        button.textContent = label;
        button.onclick = () => {
            element("modal").classList.remove("open");
            state.modalOpen = false;
            action();
            showNextQuestion();
        };
        box.appendChild(button);
    }
    element("modal").classList.add("open");
}

// ---- talking to the core

function execute(type, command = {}) {
    const status = state.core.execute(type, command);
    if (status.error) {
        log(status.error, "dim");
    }
    afterCommand(status);
    return status;
}

function afterCommand(status) {
    handleEvents();
    refresh();
    if (status.waiting !== "battle" && state.battle.open) {
        state.battle.hide();
    }
    handleWaiting(status);
    if (status.gameOver) {
        ask(status.gameOver === "won" ? "You have won the game!" : "You have been defeated.", [["OK", () => {}]]);
    }
}

function handleEvents() {
    for (const event of state.core.events()) {
        const text = describe(event);
        if (text) {
            log(text);
        }
        const body = event.event || {};
        switch (event.type) {
            case "choose-research":
                if (mine(body.player) && body.candidates.length > 0) {
                    ask("What should your sages research?", body.candidates.map((spell) => [spell, () => execute("set-research", { spell })]));
                }
                break;
            case "hire-hero":
                if (mine(body.player)) {
                    const cost = body.cost > 0 ? ` for ${body.cost} gold` : "";
                    ask(`${body.hero.name}, ${body.hero.title} (level ${body.hero.level}), offers to join you${cost}.`, [
                        ["Hire", () => execute("answer-offer", { accept: true })],
                        ["Reject", () => execute("answer-offer", { accept: false })],
                    ]);
                }
                break;
            case "hire-mercenaries":
                if (mine(body.player)) {
                    ask(`Mercenaries (${body.units.map((unit) => unit.name).join(", ")}) offer their service for ${body.cost} gold.`, [
                        ["Hire", () => execute("answer-offer", { accept: true })],
                        ["Reject", () => execute("answer-offer", { accept: false })],
                    ]);
                }
                break;
            case "merchant":
                if (mine(body.player)) {
                    ask(`A merchant offers ${body.artifact.name} for ${body.cost} gold.`, [
                        ["Buy", () => execute("answer-offer", { accept: true })],
                        ["Reject", () => execute("answer-offer", { accept: false })],
                    ]);
                }
                break;
            case "choose-spell-target":
                if (mine(body.player)) {
                    chooseSpellTarget(body.spell, body.target);
                }
                break;
            case "trade-offer":
                if (mine(body.player)) {
                    ask(`${body.wizard.wizard} would trade you ${body.receive} for ${body.give}.`, [
                        ["Yes", () => execute("answer-trade", { accept: true })],
                        ["Forget it", () => execute("answer-trade", { accept: false })],
                    ]);
                }
                break;
        }
    }
}

// a paused command waits for an answer: ask now
function handleWaiting(status) {
    switch (status.waiting) {
        case "confirm-encounter":
            ask("Your units approach a guarded place. Enter?", [
                ["Enter", () => execute("confirm-encounter", { enter: true })],
                ["Stay out", () => execute("confirm-encounter", { enter: false })],
            ]);
            break;
        case "raze-city":
            ask("The city is yours. Keep it or raze it?", [
                ["Keep", () => execute("raze-city", { raze: false })],
                ["Raze", () => execute("raze-city", { raze: true })],
            ]);
            break;
        case "battle": {
            const battle = state.core.battle();
            if (battle.error) {
                log(battle.error, "dim");
                break;
            }
            state.battle.show(battle);
            break;
        }
    }
}

const locationTargets = new Set(["friendly-city", "enemy-city", "any", "land", "empty-water", "change-terrain", "transmute", "raise-volcano", "enemy-melded-node", "disenchant"]);

// a spell cast with a choice (Spell Ward, Spell Binding, Disjunction, Resurrection): ask which of
// the choices the view lists, and send it with cast-spell (from the spellbook) or
// cast-spell-target (a spell finished over several turns)
function spellChoiceLabel(spell, choice) {
    if (choice.city) return `${choice.city}: ${choice.name}`;
    if (choice["wizard-name"]) return `${choice["wizard-name"]}'s ${choice.name}`;
    return choice.name;
}

function pickSpellChoice(spellName, commandType) {
    // the latest view: events are handled before the page refreshes its own
    const view = state.core.view();
    const spell = view.you ? view.you.spells.find((candidate) => candidate.name === spellName) : null;
    const choices = spell && spell.choices ? spell.choices : [];
    const question = { "Spell Ward": "which ward, on which city?", "Resurrection": "which hero?" }[spellName] || "which enchantment?";
    if (choices.length === 0) {
        log(`${spellName}: nothing to cast it on now.`, "dim");
        return;
    }
    ask(`${spellName}: ${question}`, [
        ...choices.map((choice) => [spellChoiceLabel(spellName, choice), () => {
            const target = { choice: choice.name };
            if (choice.location) target.location = choice.location;
            if (choice.wizard) target.wizard = choice.wizard;
            const command = commandType === "cast-spell" ? { spell: spellName, target } : { target };
            const status = execute(commandType, command);
            if (!status.error) log(`${spellName}: ${spellChoiceLabel(spellName, choice)}.`);
        }]),
        [commandType === "cast-spell" ? "Cancel" : "Not now", () => {}],
    ]);
}

function chooseSpellTarget(spell, target) {
    const you = state.core.view().you;
    const spellView = you ? you.spells.find((candidate) => candidate.name === spell) : null;
    if (spellView && spellView["choice-spell"]) {
        pickSpellChoice(spell, "cast-spell-target");
        return;
    }
    if (target === "wizard") {
        const wizards = state.view ? state.view["known-wizards"] : [];
        ask(`${spell}: on which wizard?`, [
            ...wizards.map((wizard) => [wizard.wizard, () => execute("cast-spell-target", { target: { wizard: wizard.banner } })]),
            ["Not now", () => {}],
        ]);
        return;
    }
    if (target === "friendly-unit" || locationTargets.has(target)) {
        state.spellTarget = { spell, target };
        log(`${spell}: click the target on the map (Esc to leave it for later).`);
        updateSide();
        return;
    }
    log(`${spell} needs a ${target} target; this screen can't pick one yet.`, "dim");
}

// a spell from the spellbook that can be cast now and needs a target
function pickSpellTarget(spell) {
    if (spell.target === "wizard") {
        const wizards = state.view ? state.view["known-wizards"] : [];
        ask(`${spell.name}: on which wizard?`, [
            ...wizards.map((wizard) => [wizard.wizard, () => execute("cast-spell", { spell: spell.name, target: { wizard: wizard.banner } })]),
            ["Cancel", () => {}],
        ]);
        return;
    }
    state.spellTarget = { spell: spell.name, target: spell.target, start: true };
    log(`${spell.name}: click the target on the map (Esc cancels).`);
    updateSide();
}

function openMagic() {
    if (state.view && !state.battle.open) {
        closeCity();
        state.vault.hide();
        state.magic.show(state.view);
    }
}

function openVault() {
    if (state.view && !state.battle.open) {
        closeCity();
        state.magic.hide();
        state.vault.show(state.view);
    }
}

// ---- views

function refresh() {
    state.view = state.core.view();
    const mapView = state.core.map(state.plane);
    state.map.setData(mapView, state.view);
    const stack = selectedStack();
    if (state.selectedUnit !== null && !stack) {
        state.selectedUnit = null;
    }
    state.map.setSelected(stack ? stack.location : null);
    updateHud();
    updateSide();
    if (state.magic) {
        state.magic.update(state.view);
    }
    if (state.vault) {
        state.vault.update(state.view);
    }
    if (state.cityOpen) {
        renderCityScreen();
    }
}

function updateHud() {
    const view = state.view;
    const you = view.you;
    const signed = (value) => (value >= 0 ? `+${value}` : `${value}`);
    element("date").textContent = view.date;
    element("gold").textContent = `${you.gold} (${signed(you["gold-per-turn"])})`;
    element("mana").textContent = `${you.mana} (${signed(you["mana-per-turn"])})`;
    element("food").textContent = signed(you["food-per-turn"]);
    element("research").textContent = you.researching ? `${you.researching} ${you["research-progress"]}/${you["research-cost"]}` : "-";
    element("plane").textContent = state.plane === "Arcanus" ? "Myrror" : "Arcanus";
    if (view.date !== state.lastDate) {
        state.lastDate = view.date;
        log(`— ${view.date} —`, "turn");
    }
}

function selectedStack() {
    if (!state.view || state.selectedUnit === null) {
        return null;
    }
    return state.view.stacks.find((stack) => stack.units.some((unit) => unit.id === state.selectedUnit)) || null;
}

const sameTile = (a, b) => a.plane === b.plane && a.x === b.x && a.y === b.y;

function updateSide(extra) {
    const side = element("side");
    side.innerHTML = "";
    const add = (html) => side.insertAdjacentHTML("beforeend", html);
    const escape = (text) => String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);

    if (state.spellTarget) {
        add(`<h3>${escape(state.spellTarget.spell)}</h3><div class="dim">click the target (${escape(state.spellTarget.target)}) · Esc: later</div>`);
        return;
    }

    const stack = selectedStack();
    if (stack) {
        add(`<h3>Stack at ${stack.location.x},${stack.location.y}</h3>`);
        for (const unit of stack.units) {
            add(`<div class="unit${unit.active ? "" : " inactive"}">
                <label title="moves with the stack (unchecked: stays behind)"><input type="checkbox" data-active="${unit.id}"${unit.active ? " checked" : ""}> ${escape(unit.name)}</label>
                <span class="dim">${unit["moves-left"]} mv · ${unit.health}/${unit["max-health"]}
                    <button class="tiny" data-details="${unit.id}" title="details">i</button><button class="tiny" data-disband="${unit.id}" title="disband">✕</button></span>
            </div>`);
            if (state.detailUnit === unit.id) {
                add(unitDetails(unit, escape));
            }
        }
        add(`<div class="dim" style="margin-top:4px">click a tile to move the checked units</div>`);
        for (const box of side.querySelectorAll("input[data-active]")) {
            box.onchange = () => execute("set-unit-active", { unit: Number(box.dataset.active), active: box.checked });
        }
        for (const button of side.querySelectorAll("button[data-details]")) {
            button.onclick = () => {
                const id = Number(button.dataset.details);
                state.detailUnit = state.detailUnit === id ? null : id;
                updateSide();
            };
        }
        for (const button of side.querySelectorAll("button[data-disband]")) {
            button.onclick = () => {
                const unit = stack.units.find((unit) => unit.id === Number(button.dataset.disband));
                ask(`Disband ${unit.name}?`, [
                    ["Disband", () => { if (!execute("disband-unit", { unit: unit.id }).error) log(`${unit.name} disbanded.`); }],
                    ["Keep", () => {}],
                ]);
            };
        }
    }

    if (extra) {
        add(extra);
    }
}

function unitDetails(unit, escape) {
    const details = unit.details;
    const ranged = details.ranged > 0 ? ` · ranged ${details.ranged}${details["ranged-type"] ? ` (${escape(details["ranged-type"])})` : ""}` : "";
    const upkeep = [["gold", details["upkeep-gold"]], ["food", details["upkeep-food"]], ["mana", details["upkeep-mana"]]]
        .filter(([, amount]) => amount).map(([name, amount]) => `${amount} ${name}`).join(", ");
    return `<div class="unit-details">
        ${details.level ? `<div>${escape(details.level)}</div>` : ""}
        <div>melee ${details.melee}${ranged} · defense ${details.defense} · resistance ${details.resistance}</div>
        <div>${details.figures} × ${details["hit-points"]} hp · ${details.moves} moves${upkeep ? ` · upkeep ${upkeep}` : ""}</div>
        ${details.abilities.length ? `<div class="dim">${details.abilities.map(escape).join(", ")}</div>` : ""}
        ${details.enchantments.length ? `<div class="enchanted">${details.enchantments.map(escape).join(", ")}</div>` : ""}
    </div>`;
}

// ---- the city screen: production, buying, farmers, selling, the garrison

const escapeHtml = (text) => String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);

function openCity(location) {
    state.cityOpen = location;
    renderCityScreen();
}

function closeCity() {
    state.cityOpen = null;
    element("city").classList.remove("open");
}

// a command from the city screen; the screen redraws from the new view
function cityCommand(type, command) {
    execute(type, { city: state.cityOpen, ...command });
    renderCityScreen();
}

function renderCityScreen() {
    const screen = element("city");
    const city = state.cityOpen && state.view ? state.view.cities.find((city) => sameTile(city.location, state.cityOpen)) : null;
    if (!city) {
        closeCity();
        return;
    }
    screen.classList.add("open");
    const gold = state.view.you.gold;
    const perTurn = city["production-per-turn"];
    const remaining = Math.max(0, city["production-cost"] - city.production);
    const turns = city["production-cost"] === 0 ? "" : perTurn > 0 ? `${Math.ceil(remaining / perTurn)} turn(s)` : "never";
    const canBuy = city["buy-cost"] > 0 && city["buy-cost"] <= gold;
    const workers = city.citizens - city.farmers - city.rebels;
    const garrison = state.view.stacks.find((stack) => sameTile(stack.location, city.location));

    const options = city["can-build"].map((option) => {
        const current = option.name === city.producing;
        const kind = option.unit ? "unit" : "building";
        return `<button class="option${current ? " current" : ""}" data-kind="${kind}" data-name="${escapeHtml(option.name)}">${escapeHtml(option.name)}<span class="dim">${option.cost || ""}</span></button>`;
    }).join("");

    const buildings = city.buildings.map((name) => {
        const value = city["sell-values"][name];
        const sell = value === undefined ? "" : `<button class="small" data-sell="${escapeHtml(name)}" ${city["sold-building-this-turn"] ? "disabled" : ""}>sell ${value}</button>`;
        return `<div class="row"><span>${escapeHtml(name)}</span>${sell}</div>`;
    }).join("") || `<div class="dim">no buildings yet</div>`;

    screen.innerHTML = `<div class="panel">
        <div class="row"><h3>${escapeHtml(city.name)} <span class="dim">${escapeHtml(city.race)} · ${city.population.toLocaleString()} people</span></h3><button id="cityClose" title="Esc">Close</button></div>
        <div class="columns">
            <div>
                <div class="row"><span>food</span><b>${city["food-surplus"] >= 0 ? "+" : ""}${city["food-surplus"]}</b></div>
                <div class="row"><span>gold</span><b>${city["gold-surplus"] >= 0 ? "+" : ""}${city["gold-surplus"]}</b></div>
                <div class="row"><span>production</span><b>${Math.round(perTurn * 10) / 10}</b></div>
                <div class="row"><span>power · research</span><b>${city.power} · ${city.research}</b></div>
                <h3 class="gap">Citizens ${city.citizens}/${city["maximum-size"]}</h3>
                <div class="row"><span>farmers ${city.farmers} <span class="dim">(min ${city["subsistence-farmers"]})</span></span>
                    <span><button class="small" id="fewerFarmers" ${city.farmers <= Math.min(city["subsistence-farmers"], city.citizens - city.rebels) ? "disabled" : ""}>−</button>
                    <button class="small" id="moreFarmers" ${workers <= 0 ? "disabled" : ""}>+</button></span></div>
                <div class="row"><span>workers ${workers}</span><span class="dim">rebels ${city.rebels}</span></div>
                <h3 class="gap">Producing</h3>
                <div><b>${escapeHtml(city.producing || "nothing")}</b> <span class="dim">${Math.floor(city.production)}/${city["production-cost"]} ${turns}</span></div>
                <button id="buy" class="gap" ${canBuy ? "" : "disabled"}>${city["buy-cost"] > 0 ? `Buy for ${city["buy-cost"]} gold` : "Nothing to buy"}</button>
                <h3 class="gap">Garrison</h3>
                ${garrison ? `<div class="dim">${garrison.units.map((unit) => escapeHtml(unit.name)).join(", ")}</div><button id="selectGarrison" class="small gap">Select</button>` : `<div class="dim">none</div>`}
            </div>
            <div>
                <h3>Change production</h3>
                <div class="options">${options}</div>
                <h3 class="gap">Buildings</h3>
                ${buildings}
            </div>
        </div>
    </div>`;

    element("cityClose").onclick = closeCity;
    screen.onclick = (event) => {
        if (event.target === screen) {
            closeCity();
        }
    };
    element("fewerFarmers").onclick = () => cityCommand("set-city-farmers", { farmers: city.farmers - 1 });
    element("moreFarmers").onclick = () => cityCommand("set-city-farmers", { farmers: city.farmers + 1 });
    element("buy").onclick = () => cityCommand("buy-production", {});
    for (const button of screen.querySelectorAll("button.option")) {
        button.onclick = () => cityCommand("set-city-production", { [button.dataset.kind]: button.dataset.name });
    }
    for (const button of screen.querySelectorAll("button[data-sell]")) {
        button.onclick = () => cityCommand("sell-building", { building: button.dataset.sell });
    }
    if (garrison) {
        element("selectGarrison").onclick = () => {
            state.selectedUnit = garrison.units[0].id;
            closeCity();
            refresh();
        };
    }
}

// ---- input

function onTileClick(tile) {
    const view = state.view;
    if (!view || state.modalOpen) {
        return;
    }

    if (state.spellTarget) {
        const target = { location: tile };
        if (state.spellTarget.target === "friendly-unit") {
            const stack = view.stacks.find((stack) => sameTile(stack.location, tile));
            if (!stack) {
                log("Choose one of your stacks.", "dim");
                return;
            }
            target.unit = stack.units[0].id;
        }
        const { spell, start } = state.spellTarget;
        state.spellTarget = null;
        if (start) {
            execute("cast-spell", { spell, target });
        } else {
            execute("cast-spell-target", { target });
        }
        return;
    }

    const stackHere = view.stacks.find((stack) => sameTile(stack.location, tile) && stack.units.length > 0);
    const selected = selectedStack();

    // your city opens its screen, unless a stack is selected (then the click moves it there)
    const city = view.cities.find((city) => sameTile(city.location, tile));
    if (city && !selected) {
        openCity(city.location);
        return;
    }

    if (stackHere && (!selected || !sameTile(selected.location, tile))) {
        state.selectedUnit = stackHere.units[0].id;
        refresh();
        return;
    }
    if (selected && sameTile(selected.location, tile)) {
        state.selectedUnit = null;
        refresh();
        return;
    }
    if (selected) {
        execute("move-stack", { from: selected.location, to: tile });
        return;
    }

    const foreign = view["foreign-cities"].find((city) => sameTile(city.location, tile));
    if (foreign) {
        updateSide(`<h3>${foreign.name}</h3><div class="dim">${foreign.owner.wizard} · ${foreign.size} citizens</div>`);
        return;
    }
    updateSide();
}

function nextUnit() {
    const view = state.view;
    const ready = view.stacks.filter((stack) => stack.units.some((unit) => unit["moves-left"] > 0) && !(stack.path && stack.path.length));
    if (ready.length === 0) {
        log("No units can move.", "dim");
        return;
    }
    const current = ready.findIndex((stack) => stack.units.some((unit) => unit.id === state.selectedUnit));
    const next = ready[(current + 1) % ready.length];
    state.selectedUnit = next.units[0].id;
    if (next.location.plane !== state.plane) {
        state.plane = next.location.plane;
    }
    refresh();
    state.map.centerOn(next.location.x, next.location.y);
}

function endTurn() {
    if (state.modalOpen) {
        return;
    }
    state.selectedUnit = null;
    execute("end-turn");
}

function bindControls() {
    element("endTurn").onclick = endTurn;
    element("nextUnit").onclick = nextUnit;
    element("magicButton").onclick = openMagic;
    element("itemsButton").onclick = openVault;
    element("plane").onclick = () => {
        state.plane = state.plane === "Arcanus" ? "Myrror" : "Arcanus";
        refresh();
    };
    window.addEventListener("keydown", (event) => {
        if (state.modalOpen) {
            return;
        }
        if (state.battle.open) {
            state.battle.key(event.key);
            return;
        }
        if (state.magic.open) {
            if (event.key === "Escape" || event.key === "m" || event.key === "M") state.magic.hide();
            return;
        }
        if (state.vault.open) {
            if (event.key === "Escape" || event.key === "i" || event.key === "I") state.vault.hide();
            return;
        }
        if (state.cityOpen) {
            if (event.key === "Escape") closeCity();
            return;
        }
        if (event.key === "m" || event.key === "M") openMagic();
        if (event.key === "i" || event.key === "I") openVault();
        if (event.key === "Enter") endTurn();
        if (event.key === "n" || event.key === "N") nextUnit();
        if (event.key === "Escape") {
            state.spellTarget = null;
            state.selectedUnit = null;
            refresh();
        }
    });
}

// ---- start

async function main() {
    const params = new URLSearchParams(location.search);
    try {
        const [core, assets, gamedata] = await Promise.all([
            loadCore("core.wasm"),
            loadAssets("assets"),
            fetch("gamedata.json").then((response) => {
                if (!response.ok) throw new Error(`gamedata.json: ${response.status}`);
                return response.text();
            }),
        ]);
        state.core = core;
        if (params.has("log")) {
            core.setLogging(true);
        }

        const settings = { opponents: parseInt(params.get("opponents") || "3", 10), raiders: true, difficulty: params.get("difficulty") || "average" };
        // a setup for tests (core/game/scenario.go), e.g. ?scenario=battle
        if (params.has("scenario")) {
            settings.scenario = params.get("scenario");
        }
        const status = core.newGame(gamedata, settings);
        if (status.error) {
            throw new Error(status.error);
        }

        state.map = new MapRenderer(element("map"), assets, { onTileClick });
        state.magic = new MagicScreen(element("magic"), assets, {
            onCommand: (type, command) => execute(type, command),
            onPickTarget: pickSpellTarget,
            onPickChoice: (spell) => pickSpellChoice(spell.name, "cast-spell"),
        });
        state.vault = new VaultScreen(element("vault"), assets, {
            onCommand: (type, command) => execute(type, command),
        });
        state.battle = new BattleScreen(element("battle"), assets, {
            onCommand: (type, command) => execute(type, command),
        });
        element("loading").remove();
        for (const id of ["top", "log", "hint"]) {
            element(id).hidden = false;
        }
        bindControls();

        log(`Welcome, ${core.view().you.player.wizard}.`);
        afterCommand(status);
        const capital = state.view.cities[0];
        if (capital) {
            state.plane = capital.location.plane;
            refresh();
            state.map.centerOn(capital.location.x, capital.location.y);
        }
        state.map.start();
        // for tests and the console: the state, and commands through the same path as the UI
        window.civGame = state;
        state.execute = execute;
    } catch (error) {
        console.error(error);
        element("loading").textContent = `Error: ${error.message}`;
    }
}

main();
