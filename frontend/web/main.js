// The web frontend's game screen: loads the core and the sprites, starts a game, and plays it
// through commands. The core owns the rules; this file only shows the views and asks the player.

import { loadCore } from "./core.js";
import { loadAssets } from "./assets.js";
import { MapRenderer } from "./map.js";
import { MapRenderer3D } from "./map3d.js";
import { BattleScreen } from "./battle.js";
import { MagicScreen } from "./magic.js";
import { VaultScreen } from "./vault.js";
import { DiplomacyScreen } from "./diplomacy.js";
import { ListScreen } from "./lists.js";
import { Minimap } from "./minimap.js";
import { chooseStart } from "./start.js";
import { storeSave, readSave, saveInfo, downloadSave, pickSaveFile, describeSave, slotNames, savedGames } from "./saves.js";

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
    // engineers picking where to build a road: {stack: location}; the map shows the path
    roadPick: null,
    questions: [],
    modalOpen: false,
    // the location of the city whose screen is open, or null
    cityOpen: null,
    battle: null,
    magic: null,
    vault: null,
    diplomacy: null,
    lists: null,
    minimap: null,
    // the unit whose details the stack panel shows
    detailUnit: null,
    lastDate: null,
    // the data tables' text, for loading a save
    gamedata: null,
    // autosave at the start of every turn (not in test scenarios)
    autosave: true,
    // the date of the last autosave
    autosaved: null,
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
const treatyNames = { pact: "a wizard's pact", alliance: "an alliance", peace: "peace" };

function describe(event) {
    const body = event.event || {};
    if (event.type === "spell-of-mastery") {
        const who = mine(body.player) ? "You" : body.player.wizard;
        return body.started
            ? (mine(body.player) ? "You begin casting the Spell of Mastery." : `${who} has begun casting the Spell of Mastery!`)
            : `${who} cast the Spell of Mastery.`;
    }
    if (event.type === "war-declared") {
        if (mine(body.wizard)) return `You broke your treaty with ${body.player.wizard} by attacking: you are at war.`;
        return body.attacked ? `${body.wizard.wizard} broke your treaty and attacked you: you are at war.` : `${body.wizard.wizard} declares war on you!`;
    }
    if (event.type === "treaty-proposal") {
        return `${body.wizard.wizard} proposes ${treatyNames[body.treaty] || body.treaty}.`;
    }
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
    // a new turn (after an AI attack, once its battle is over): keep it
    if (state.autosave && !status.waiting && !status.gameOver && state.view.date !== state.autosaved) {
        state.autosaved = state.view.date;
        saveTo("autosave").then((error) => { if (error) log(`Autosave: ${error}.`, "dim"); });
    }
    if (status.waiting !== "battle" && state.battle.open) {
        state.battle.hide();
    }
    handleWaiting(status);
    if (status.gameOver && !state.ended) {
        state.ended = true;
        showEnding(status.gameOver);
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
            case "spell-of-mastery":
                if (!body.started) {
                    state.masteryWinner = body.player;
                } else if (!mine(body.player)) {
                    ask(`${body.player.wizard} has begun casting the Spell of Mastery! If the casting finishes, ${body.player.wizard} wins the game.`, [["OK", () => {}]]);
                }
                break;
            case "treaty-proposal":
                // the wizards screen shows the proposal itself
                if (mine(body.player) && !state.diplomacy.open) {
                    ask(`${body.wizard.wizard} proposes ${treatyNames[body.treaty] || body.treaty}.`, [
                        ["Accept", () => { if (!execute("answer-proposal", { accept: true }).error) log(`You agreed to ${treatyNames[body.treaty] || body.treaty} with ${body.wizard.wizard}.`); }],
                        ["Decline", () => execute("answer-proposal", { accept: false })],
                    ]);
                }
                break;
            case "war-declared":
                if (mine(body.player) && !body.attacked) {
                    ask(`${body.wizard.wizard} declares war on you!`, [["OK", () => {}]]);
                }
                break;
            case "trade-offer":
                // the diplomacy screen shows the offer itself
                if (mine(body.player) && !state.diplomacy.open) {
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

// the full-screen panels: one open at a time
function hideScreens() {
    closeCity();
    closeGameMenu();
    for (const screen of [state.magic, state.vault, state.diplomacy, state.lists]) {
        screen.hide();
    }
}

function openMagic() {
    if (state.view && !state.battle.open) {
        hideScreens();
        state.magic.show(state.view);
    }
}

function openVault() {
    if (state.view && !state.battle.open) {
        hideScreens();
        state.vault.show(state.view);
    }
}

function openDiplomacy() {
    if (state.view && !state.battle.open) {
        hideScreens();
        state.diplomacy.show(state.view);
    }
}

function openList(tab) {
    if (state.view && !state.battle.open) {
        hideScreens();
        state.lists.show(state.view, tab);
    }
}

// select a stack (from the army list) and show it on the map
function selectStack(stack) {
    state.selectedUnit = stack.units[0].id;
    state.plane = stack.location.plane;
    refresh();
    state.map.centerOn(stack.location.x, stack.location.y);
}

// ---- the end of the game

// the winner's portrait asset, when known: yours, or a wizard you met
function portraitOf(player) {
    if (!player || !state.view) return null;
    if (player.human) return state.view.you.mirror.portrait;
    const known = state.view.diplomacy.find((wizard) => wizard.wizard.banner === player.banner);
    return known ? known.mirror.portrait : null;
}

function showEnding(result) {
    hideScreens();
    const root = element("ending");
    const winner = state.masteryWinner;
    let title;
    let text;
    if (winner && result === "won") {
        title = "The Spell of Mastery";
        text = "Your Spell of Mastery is cast. The other wizards are banished from the world, and it is yours to rule.";
    } else if (winner) {
        title = "The Spell of Mastery";
        text = `${winner.wizard} has cast the Spell of Mastery. You are banished from the world.`;
    } else if (result === "won") {
        title = "Victory";
        text = "Every rival wizard has been defeated. The world is yours.";
    } else {
        title = "Defeat";
        text = "You have been defeated.";
    }
    root.innerHTML = `<div class="panel">
        <div class="ending-portrait"></div>
        <h2>${escapeHtml(title)}</h2>
        <p>${escapeHtml(text)}</p>
        <p class="dim">${escapeHtml(state.view.date)}</p>
        <div class="choices"><button data-new>New game</button><button data-look>Look at the map</button></div>
    </div>`;
    const portrait = state.assets.picture(portraitOf(winner || (result === "won" ? state.view.you.player : null)), 1);
    if (portrait) root.querySelector(".ending-portrait").appendChild(portrait);
    root.querySelector("[data-new]").onclick = () => { location.href = location.pathname; };
    root.querySelector("[data-look]").onclick = () => root.classList.remove("open");
    root.classList.add("open");
}

// ---- saving and loading

const saveTitle = () => `${state.view.you.player.wizard}, ${state.view.date}`;

// keep the game in a slot; resolves to an error message or null
async function saveTo(slot) {
    let text;
    try {
        text = state.core.save(saveTitle());
    } catch (error) {
        return error.message;
    }
    return storeSave(slot, text, { wizard: state.view.you.player.wizard, date: state.view.date });
}

// a saved game replaces the one being played
function loadGame(text) {
    const status = state.core.load(state.gamedata, text);
    if (status.error) {
        return status.error;
    }
    hideScreens();
    state.selectedUnit = null;
    state.detailUnit = null;
    state.spellTarget = null;
    if (state.roadPick) endRoadPick();
    state.questions = [];
    state.ended = false;
    state.masteryWinner = null;
    state.view = state.core.view();
    state.autosaved = state.view.date;
    log(`Loaded ${saveTitle()}.`);
    afterCommand(status);
    centerOnHome();
    return null;
}

function centerOnHome() {
    const home = state.view.cities[0] || state.view.stacks[0];
    if (home) {
        state.plane = home.location.plane;
        refresh();
        state.map.centerOn(home.location.x, home.location.y);
    }
}

function openGameMenu(message = "") {
    if (!state.view || state.battle.open) {
        return;
    }
    hideScreens();
    const root = element("game");
    const saved = saveInfo("save");
    const autosave = saveInfo("autosave");
    root.innerHTML = `<div class="panel">
        <div class="row"><h3>Game</h3><button class="small" data-close title="Esc">Close</button></div>
        <div class="game-actions">
            <button data-save>Save${saved ? ` <span class="dim">(replaces ${escapeHtml(describeSave(saved))})</span>` : ""}</button>
            <button data-download>Download a save file</button>
            ${saved ? `<button data-load="save">Load ${escapeHtml(slotNames.save)} <span class="dim">${escapeHtml(describeSave(saved))}</span></button>` : ""}
            ${autosave ? `<button data-load="autosave">Load ${escapeHtml(slotNames.autosave)} <span class="dim">${escapeHtml(describeSave(autosave))}</span></button>` : ""}
            <button data-load-file>Load a save file…</button>
            <button data-new>New game</button>
        </div>
        <div class="dim">loading replaces this game: save it first to keep it · saves stay in this browser</div>
        <div class="message">${escapeHtml(message)}</div>
    </div>`;
    root.classList.add("open");
    const say = (text) => { root.querySelector(".message").textContent = text; };
    root.querySelector("[data-close]").onclick = closeGameMenu;
    root.onclick = (event) => { if (event.target === root) closeGameMenu(); };
    root.querySelector("[data-save]").onclick = async () => {
        say("saving…");
        const error = await saveTo("save");
        openGameMenu(error ? `Not saved: ${error}.` : `Saved: ${saveTitle()}.`);
    };
    root.querySelector("[data-download]").onclick = () => {
        try {
            downloadSave(state.core.save(saveTitle()), saveTitle());
            say("Downloaded.");
        } catch (error) {
            say(`Not saved: ${error.message}.`);
        }
    };
    for (const button of root.querySelectorAll("[data-load]")) {
        button.onclick = async () => {
            say("loading…");
            try {
                const error = loadGame(await readSave(button.dataset.load));
                if (error) say(`Couldn't load: ${error}.`); else closeGameMenu();
            } catch (error) {
                say(`Couldn't load: ${error.message}.`);
            }
        };
    }
    root.querySelector("[data-load-file]").onclick = async () => {
        const text = await pickSaveFile();
        if (!text) return;
        const error = loadGame(text);
        if (error) say(`Couldn't load the file: ${error}.`); else closeGameMenu();
    };
    root.querySelector("[data-new]").onclick = () => {
        location.href = location.pathname;
    };
}

function closeGameMenu() {
    element("game").classList.remove("open");
}

const gameMenuOpen = () => element("game").classList.contains("open");

// ---- views

function refresh() {
    state.view = state.core.view();
    const mapView = state.core.map(state.plane);
    state.map.setData(mapView, state.view);
    state.minimap.setData(mapView, state.view);
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
    if (state.diplomacy) {
        state.diplomacy.update(state.view);
    }
    if (state.lists) {
        state.lists.update(state.view);
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
    const tax = element("tax");
    const rates = you["tax-rates"];
    const current = rates.findIndex((rate) => `${rate.numerator}/${rate.denominator}` === you["tax-rate"] || (rate.numerator === 0 && you["tax-rate"].startsWith("0/")));
    tax.innerHTML = rates.map((rate, index) => `<option value="${index}"${index === current ? " selected" : ""} title="${rate.label} gold per citizen, ${Math.round(rate.unrest * 100)}% unrest">${rate.label} (${Math.round(rate.unrest * 100)}%)</option>`).join("");
    tax.onchange = () => {
        const rate = rates[Number(tax.value)];
        execute("set-tax-rate", { numerator: rate.numerator, denominator: rate.denominator });
    };
    element("research").parentElement.title = you.researching || "";
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
    if (state.roadPick) {
        add(`<h3>Build a road</h3><div class="dim">click the destination: the engineers build the road over the next turns · Esc cancels</div>`);
        return;
    }

    const stack = selectedStack();
    if (stack) {
        add(`<h3>Stack at ${stack.location.x},${stack.location.y}</h3>`);
        const orders = stackOrders(stack);
        if (orders.length) {
            add(`<div class="orders">${orders.map((order) => `<button class="small" data-order="${order.id}" title="${escape(order.title)}">${escape(order.label)}</button>`).join("")}</div>`);
        }
        for (const unit of stack.units) {
            add(`<div class="unit${unit.active ? "" : " inactive"}">
                <label title="moves with the stack (unchecked: stays behind)"><input type="checkbox" data-active="${unit.id}"${unit.active ? " checked" : ""}> ${escape(unit.name)}</label>
                <span class="dim">${unit.busy ? `<span class="busy">${escape(unit.busy)}</span> · ` : ""}${unit["moves-left"]} mv · ${unit.health}/${unit["max-health"]}
                    <button class="tiny" data-details="${unit.id}" title="details">i</button><button class="tiny" data-disband="${unit.id}" title="disband">✕</button></span>
            </div>`);
            if (state.detailUnit === unit.id) {
                add(unitDetails(unit, escape));
            }
        }
        add(`<div class="dim" style="margin-top:4px">click a tile to move the checked units</div>`);
        for (const button of side.querySelectorAll("button[data-order]")) {
            button.onclick = () => runOrder(button.dataset.order);
        }
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

// the orders the selected stack can take now: the build action the view names, a road for
// engineers, planar travel where the view allows it, and patrol
const buildLabels = {
    settle: ["Settle (B)", "found an outpost here"],
    meld: ["Meld (B)", "meld with the node: its power goes to you"],
    purify: ["Purify (B)", "the priests cleanse the corruption over the next turns"],
};

function stackOrders(stack) {
    const orders = [];
    const build = buildLabels[stack.build];
    if (build) {
        orders.push({ id: "build", label: build[0], title: build[1] });
    }
    if (stack.road) {
        orders.push({ id: "road", label: stack.build === "road" ? "Road (B)" : "Road (R)", title: "the engineers build a road to a tile you pick" });
    }
    if (stack["planar-travel"]) {
        orders.push({ id: "plane", label: "Plane (T)", title: `go to ${stack.location.plane === "Arcanus" ? "Myrror" : "Arcanus"}` });
    }
    if (stack.units.some((unit) => unit.active)) {
        orders.push({ id: "patrol", label: "Patrol (P)", title: "the checked units guard this tile and are skipped by Next unit" });
    }
    return orders;
}

function runOrder(id) {
    const stack = selectedStack();
    if (!stack || state.modalOpen) {
        return;
    }
    if (!stackOrders(stack).some((order) => order.id === id)) {
        if (id === "build") log("Nothing to build here.", "dim");
        if (id === "plane") log("This stack can't travel to the other plane here.", "dim");
        return;
    }
    switch (id) {
        case "build":
            if (!execute("build-action", { stack: stack.location }).error && stack.build === "meld") {
                log(`${stack.units.find((unit) => unit.active).name} melded with the node.`);
            }
            break;
        case "road":
            state.roadPick = { stack: stack.location };
            log("Road: click the destination (Esc cancels).");
            updateSide();
            break;
        case "plane": {
            const status = execute("planar-travel", { stack: stack.location });
            if (!status.error) {
                const moved = selectedStack();
                if (moved && moved.location.plane !== state.plane) {
                    state.plane = moved.location.plane;
                    refresh();
                    state.map.centerOn(moved.location.x, moved.location.y);
                }
                log(`The stack travels to ${moved ? moved.location.plane : "the other plane"}.`);
            }
            break;
        }
        case "patrol":
            if (!execute("patrol", { stack: stack.location }).error) {
                state.selectedUnit = null;
                refresh();
            }
            break;
    }
}

function endRoadPick() {
    state.roadPick = null;
    state.map.setPreviewPath(null);
    updateSide();
}

// the surveyor's view of the tile under the pointer (core TileInfoView), in the corner
function showTileInfo(tile) {
    const panel = element("tileinfo");
    const info = tile ? state.core.tileInfo(tile) : null;
    if (!info || info.error) {
        panel.hidden = true;
        return;
    }
    const lines = [];
    const add = (html) => lines.push(`<div>${html}</div>`);
    const percent = (value) => `+${value}%`;
    add(`<div class="row"><b>${escapeHtml(info.terrain)}</b><span class="dim">${info.location.x},${info.location.y}</span></div>`);
    const terrain = [info.corrupted ? "corrupted" : "", info["food-text"] ? `${info["food-text"]} food` : "", info["production-bonus"] ? `${percent(info["production-bonus"])} production` : "", info["gold-bonus"] ? `${percent(info["gold-bonus"])} gold` : ""].filter(Boolean);
    if (terrain.length) add(`<span class="dim">${terrain.join(" · ")}</span>`);
    if (info.bonus) {
        const bonus = info.bonus;
        const gives = [bonus.food ? `+${bonus.food} food` : "", bonus.gold ? `+${bonus.gold} gold` : "", bonus.power ? `+${bonus.power} power` : "", bonus["unit-cost-reduction"] ? `units −${bonus["unit-cost-reduction"]}%` : ""].filter(Boolean);
        add(`${escapeHtml(bonus.name)} <span class="dim">${gives.join(" · ")}</span>`);
    }
    if (info.node) {
        const node = info.node;
        const meld = node.warped ? "warped" : node["melded-by"] ? `${escapeHtml(node.spirit)} of ${escapeHtml(node["melded-by"].wizard)}` : "";
        add(`${escapeHtml(node.kind)}${meld ? ` <span class="dim">${meld}</span>` : ""}`);
    }
    if (info.encounter) {
        add(`${info.node ? "guards" : escapeHtml(info.encounter.type)}: <span class="dim">${escapeHtml(info.encounter.guards)}</span>`);
    }
    if (info.city) {
        add(`${escapeHtml(info.city)}${info["city-owner"] && !info["city-owner"].human ? ` <span class="dim">(${escapeHtml(info["city-owner"].wizard)})</span>` : ""}`);
    }
    const site = info.site;
    if (site["maximum-population"]) {
        add(`<div class="site">${site["can-build"] ? "city site" : info.city ? "city resources" : `<span class="dim">${escapeHtml(site.reason || "")}</span>`}
            <div class="row"><span class="dim">max pop ${site["maximum-population"]}</span><span class="dim">prod ${percent(site["production-bonus"])} · gold ${percent(site["gold-bonus"])}</span></div></div>`);
    } else if (site.reason) {
        add(`<div class="site dim">${escapeHtml(site.reason)}</div>`);
    }
    panel.innerHTML = lines.join("");
    panel.hidden = false;
}

function onTileHover(tile) {
    showTileInfo(tile);
    if (!state.roadPick) {
        return;
    }
    if (!tile || tile.plane !== state.roadPick.stack.plane) {
        state.map.setPreviewPath(null);
        return;
    }
    const path = state.core.roadPath(state.roadPick.stack, tile);
    state.map.setPreviewPath(Array.isArray(path) ? [state.roadPick.stack, ...path] : null);
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
    state.cityRenaming = false;
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

    const title = state.cityRenaming
        ? `<span class="rename"><input id="cityName" maxlength="20" value="${escapeHtml(city.name)}"><button class="small" id="cityNameSave">Save</button><button class="small" id="cityNameCancel">Cancel</button></span>`
        : `<h3>${escapeHtml(city.name)} <button class="tiny" id="cityRename" title="rename">✎</button> <span class="dim">${escapeHtml(city.race)} · ${city.population.toLocaleString()} people</span></h3>`;
    const enchantments = city.enchantments.map((enchantment) => `<div class="row">
            <span class="${enchantment.yours ? "" : "curse"}">${escapeHtml(enchantment.name)}${enchantment.yours ? "" : ` <span class="dim">(${escapeHtml(enchantment.owner)})</span>`}</span>
            <span>${enchantment.yours && enchantment.upkeep ? `<span class="dim">${enchantment.upkeep} mana</span> ` : ""}${enchantment.yours ? `<button class="small" data-cancel-enchantment="${escapeHtml(enchantment.name)}">cancel</button>` : ""}</span>
        </div>`).join("") || `<div class="dim">none</div>`;

    screen.innerHTML = `<div class="panel">
        <div class="row">${title}<button id="cityClose" title="Esc">Close</button></div>
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
                <h3 class="gap">Enchantments</h3>
                ${enchantments}
            </div>
        </div>
    </div>`;

    element("cityClose").onclick = closeCity;
    if (state.cityRenaming) {
        const input = element("cityName");
        input.focus();
        input.select();
        const save = () => {
            state.cityRenaming = false;
            const name = input.value.trim();
            if (name && name !== city.name) {
                cityCommand("rename-city", { name });
            } else {
                renderCityScreen();
            }
        };
        element("cityNameSave").onclick = save;
        element("cityNameCancel").onclick = () => { state.cityRenaming = false; renderCityScreen(); };
        input.onkeydown = (event) => {
            if (event.key === "Enter") save();
            if (event.key === "Escape") { event.stopPropagation(); state.cityRenaming = false; renderCityScreen(); }
        };
    } else {
        element("cityRename").onclick = () => { state.cityRenaming = true; renderCityScreen(); };
    }
    for (const button of screen.querySelectorAll("button[data-cancel-enchantment]")) {
        const name = button.dataset.cancelEnchantment;
        button.onclick = () => ask(`Cancel your ${name} on ${city.name}?`, [
            ["Cancel it", () => cityCommand("cancel-enchantment", { enchantment: name })],
            ["Keep it", () => {}],
        ]);
    }
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

    if (state.roadPick) {
        const stack = state.roadPick.stack;
        endRoadPick();
        if (!execute("build-road", { stack, to: tile }).error) {
            log(`The engineers start a road to ${tile.x},${tile.y}.`);
        }
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
    // units on patrol, building a road or purifying have their orders: skip them
    const ready = view.stacks.filter((stack) => stack.units.some((unit) => unit["moves-left"] > 0 && !unit.busy) && !(stack.path && stack.path.length));
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
    if (state.roadPick) {
        endRoadPick();
    }
    const status = execute("end-turn");
    // new items in the vault must be placed or destroyed first: show them
    if (status.error && state.view.you.items.new.length > 0) {
        openVault();
        return;
    }
    if (!status.error) {
        state.selectedUnit = null;
        refresh();
    }
}

function bindControls() {
    element("endTurn").onclick = endTurn;
    element("nextUnit").onclick = nextUnit;
    element("magicButton").onclick = openMagic;
    element("itemsButton").onclick = openVault;
    element("wizardsButton").onclick = openDiplomacy;
    element("gameButton").onclick = () => openGameMenu();
    element("listsButton").onclick = () => openList(state.lists.tab);
    element("plane").onclick = () => {
        state.plane = state.plane === "Arcanus" ? "Myrror" : "Arcanus";
        refresh();
    };
    window.addEventListener("keydown", (event) => {
        // typing a name: the field handles its own keys
        if (state.modalOpen || event.target instanceof HTMLInputElement) {
            return;
        }
        if (state.battle.open) {
            state.battle.key(event.key);
            return;
        }
        if (gameMenuOpen()) {
            if (event.key === "Escape" || event.key === "g" || event.key === "G") closeGameMenu();
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
        if (state.diplomacy.open) {
            if (event.key === "Escape" || event.key === "w" || event.key === "W") state.diplomacy.hide();
            return;
        }
        if (state.lists.open) {
            const key = event.key.toLowerCase();
            const tab = { c: "cities", a: "armies" }[key];
            if (event.key === "Escape" || tab === state.lists.tab) state.lists.hide();
            else if (tab) state.lists.show(state.view, tab);
            return;
        }
        if (state.cityOpen) {
            if (event.key === "Escape") closeCity();
            return;
        }
        if (event.key === "m" || event.key === "M") openMagic();
        if (event.key === "i" || event.key === "I") openVault();
        if (event.key === "w" || event.key === "W") openDiplomacy();
        if (event.key === "g" || event.key === "G") openGameMenu();
        // the 3D map: flatten the ground or give it back its height
        if ((event.key === "h" || event.key === "H") && state.map.toggleHeight) state.map.toggleHeight();
        if (event.key === "c" || event.key === "C") openList("cities");
        if (event.key === "a" || event.key === "A") openList("armies");
        if (event.key === "Enter") endTurn();
        if (event.key === "n" || event.key === "N") nextUnit();
        if (selectedStack() && !state.spellTarget && !state.roadPick) {
            const key = event.key.toLowerCase();
            if (key === "b") {
                runOrder(selectedStack().build === "road" ? "road" : "build");
            }
            if (key === "r") runOrder("road");
            if (key === "t") runOrder("plane");
            if (key === "p") runOrder("patrol");
        }
        if (event.key === "Escape") {
            if (state.roadPick) {
                endRoadPick();
                return;
            }
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

        // URL parameters skip the start screen (tests, bookmarks): ?opponents=2&difficulty=hard,
        // ?wizard=Merlin&race=High Elf&banner=red, or a test setup (core/game/scenario.go) ?scenario=battle
        state.gamedata = gamedata;
        let settings;
        let saveText = null;
        if (params.has("load")) {
            // ?load=autosave or ?load=save: continue a game kept in this browser
            saveText = await readSave(params.get("load"));
        } else if (["opponents", "difficulty", "wizard", "scenario"].some((name) => params.has(name))) {
            settings = { opponents: parseInt(params.get("opponents") || "3", 10), raiders: true, difficulty: params.get("difficulty") || "average" };
            for (const name of ["scenario", "wizard", "race", "banner", "magic"]) {
                if (params.has(name)) settings[name] = params.get(name);
            }
        } else {
            element("loading").hidden = true;
            const choice = await chooseStart(element("start"), core.startOptions(), assets, savedGames(), (custom) => core.checkWizard(custom));
            element("loading").hidden = false;
            if (choice.saveSlot) {
                saveText = await readSave(choice.saveSlot);
            } else if (choice.saveText) {
                saveText = choice.saveText;
            } else {
                settings = choice;
            }
            element("loading").textContent = saveText ? "loading the game…" : "making the world…";
            // let the page paint before the core builds the world
            await new Promise((resolve) => setTimeout(resolve, 30));
        }
        const status = saveText ? core.load(gamedata, saveText) : core.newGame(gamedata, settings);
        if (status.error) {
            throw new Error(status.error);
        }
        state.autosave = !(settings && settings.scenario);

        state.assets = assets;
        // the isometric 3D map; ?view=2d for the flat one
        const Renderer = params.get("view") === "2d" ? MapRenderer : MapRenderer3D;
        state.map = new Renderer(element("map"), assets, { onTileClick, onTileHover, onFrame: () => state.minimap.update() });
        state.minimap = new Minimap(element("minimap").querySelector("canvas"), state.map, assets);
        state.magic = new MagicScreen(element("magic"), assets, {
            confirm: (text, yes) => ask(text, [["Yes", yes], ["No", () => {}]]),
            onCommand: (type, command) => execute(type, command),
            onPickTarget: pickSpellTarget,
            onPickChoice: (spell) => pickSpellChoice(spell.name, "cast-spell"),
        });
        state.diplomacy = new DiplomacyScreen(element("diplomacy"), assets, {
            onCommand: (type, command) => execute(type, command),
        });
        state.vault = new VaultScreen(element("vault"), assets, {
            onCommand: (type, command) => execute(type, command),
        });
        state.lists = new ListScreen(element("lists"), assets, {
            onOpenCity: openCity,
            onSelectStack: selectStack,
        });
        state.battle = new BattleScreen(element("battle"), assets, {
            onCommand: (type, command) => execute(type, command),
        });
        element("loading").remove();
        for (const id of ["top", "log", "hint", "corner"]) {
            element(id).hidden = false;
        }
        bindControls();

        log(saveText ? `Welcome back, ${core.view().you.player.wizard}.` : `Welcome, ${core.view().you.player.wizard}.`);
        afterCommand(status);
        centerOnHome();
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
