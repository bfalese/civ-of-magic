// The web frontend's game screen: loads the core and the sprites, starts a game, and plays it
// through commands. The core owns the rules; this file only shows the views and asks the player.

import { loadCore } from "./core.js";
import { loadAssets } from "./assets.js";
import { MapRenderer } from "./map.js";

const element = (id) => document.getElementById(id);

const state = {
    core: null,
    map: null,
    view: null,
    plane: "Arcanus",
    // the selected stack is followed by its first unit's id (stacks move; ids don't)
    selectedUnit: null,
    // a finished spell waiting for its target: {spell, target}
    spellTarget: null,
    questions: [],
    modalOpen: false,
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
        case "vault": return `${body.artifact ? body.artifact.name : "An item"} went to your vault.`;
        case "hero-level-up": return `${body.hero.name} gained a level.`;
        case "banish": return `${body.defender.wizard} was banished by ${body.attacker.wizard}.`;
        case "battle-ended":
            if (!mine(body.attacker) && !mine(body.defender)) return null;
            return `Battle at ${body.location.x},${body.location.y}: ${body.result}.`;
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
            const yours = (battle.units || []).filter((unit) => unit.yours).length;
            const theirs = (battle.units || []).length - yours;
            ask(`Battle! Your ${yours} unit(s) against ${theirs}. (Tactical battles come later.)`, [
                ["Fight (auto)", () => execute("battle-auto")],
                ["Flee", () => execute("battle-flee")],
            ]);
            break;
        }
    }
}

const locationTargets = new Set(["friendly-city", "enemy-city", "any", "land", "empty-water", "change-terrain", "transmute", "raise-volcano", "enemy-melded-node", "disenchant"]);

function chooseSpellTarget(spell, target) {
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
            add(`<div class="unit"><span>${escape(unit.name)}</span><span class="dim">${unit["moves-left"]} mv · ${unit.health}/${unit["max-health"]}</span></div>`);
        }
        add(`<div class="dim" style="margin-top:4px">click a tile to move</div>`);
    }

    if (extra) {
        add(extra);
    }
}

function cityInfo(city) {
    const escape = (text) => String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
    return `<h3>${escape(city.name)}</h3>
        <div>population ${city.population} (${city.citizens}/${city["maximum-size"]})</div>
        <div>farmers ${city.farmers} · workers ${city.workers} · rebels ${city.rebels}</div>
        <div>food ${city["food-surplus"]} · gold ${city["gold-surplus"]} · production ${city["production-per-turn"]}</div>
        <div>building <b>${escape(city.producing || "nothing")}</b> ${Math.floor(city.production)}/${city["production-cost"]}</div>
        <div class="dim">${city.buildings.map(escape).join(", ") || "no buildings"}</div>`;
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
        state.spellTarget = null;
        execute("cast-spell-target", { target });
        return;
    }

    const stackHere = view.stacks.find((stack) => sameTile(stack.location, tile) && stack.units.length > 0);
    const selected = selectedStack();

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

    const city = view.cities.find((city) => sameTile(city.location, tile));
    if (city) {
        updateSide(cityInfo(city));
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
    element("plane").onclick = () => {
        state.plane = state.plane === "Arcanus" ? "Myrror" : "Arcanus";
        refresh();
    };
    window.addEventListener("keydown", (event) => {
        if (state.modalOpen) {
            return;
        }
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
        const status = core.newGame(gamedata, settings);
        if (status.error) {
            throw new Error(status.error);
        }

        state.map = new MapRenderer(element("map"), assets, { onTileClick });
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
        window.civGame = state;
    } catch (error) {
        console.error(error);
        element("loading").textContent = `Error: ${error.message}`;
    }
}

main();
