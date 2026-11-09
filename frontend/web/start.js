// The start screen: pick a premade wizard, the race of your first city, a banner colour, the
// number of opponents, difficulty, magic strength and land size, from the core's start options
// (core StartOptions). Resolves to the settings for newGame. The last choices are remembered in
// this browser when storage works. Saved games come first: Continue resolves to {saveSlot}, a
// save file to {saveText}.

const realmColors = {
    Life: "#f4f0d8", Death: "#b07ae0", Chaos: "#ff7a50", Nature: "#7ed06a", Sorcery: "#6ab4ff",
};

const storageKey = "civ-of-magic-start";

function escape(text) {
    return String(text).replace(/[&<>"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[character]);
}

function remembered() {
    try {
        return JSON.parse(localStorage.getItem(storageKey) || "{}") || {};
    } catch (error) {
        return {};
    }
}

function remember(settings) {
    try {
        localStorage.setItem(storageKey, JSON.stringify(settings));
    } catch (error) {
        // private windows and blocked storage: just don't remember
    }
}

import { pickSaveFile, describeSave, slotNames } from "./saves.js";

export function chooseStart(root, options, assets, saves = []) {
    const last = remembered();
    const choice = {
        wizard: options.wizards.some((wizard) => wizard.name === last.wizard) ? last.wizard : options.wizards[0].name,
        race: last.race || "",
        banner: options.banners.includes(last.banner) ? last.banner : "",
        opponents: Math.min(options["max-opponents"], Math.max(options["min-opponents"], Number(last.opponents) || 3)),
        difficulty: options.difficulties.includes(last.difficulty) ? last.difficulty : "average",
        magic: options.magic.includes(last.magic) ? last.magic : "normal",
        landSize: Number.isInteger(last.landSize) ? last.landSize : 1,
        wizardName: "",
    };

    const bannerColor = (banner) => {
        const colors = (assets.manifest.bannerColors || {})[banner];
        if (!colors || !colors.length) return banner;
        const [red, green, blue] = colors[Math.min(1, colors.length - 1)];
        return `rgb(${red}, ${green}, ${blue})`;
    };

    return new Promise((resolve) => {
        const render = () => {
            const wizard = options.wizards.find((candidate) => candidate.name === choice.wizard);
            const races = options.races.filter((race) => !race.myrran || wizard.myrran);
            if (choice.race && !races.some((race) => race.name === choice.race)) choice.race = "";

            root.innerHTML = `
                <div class="panel">
                    <div class="start-continue">
                        ${saves.map((save) => `<button data-continue="${escape(save.slot)}"><b>Continue</b> <span class="dim">${escape(slotNames[save.slot])}: ${escape(describeSave(save))}</span></button>`).join("")}
                        <button data-load-file>Load a save file…</button>
                    </div>
                    <h2>New game</h2>
                    <h3>Wizard</h3>
                    <div class="start-wizards">${options.wizards.map((candidate) => `
                        <button class="start-wizard${candidate.name === choice.wizard ? " chosen" : ""}" data-wizard="${escape(candidate.name)}">
                            <b>${escape(candidate.name)}</b>
                            <span>${Object.entries(candidate.books).map(([realm, count]) => `<span style="color: ${realmColors[realm] || ""}">${count} ${escape(realm)}</span>`).join(" ")}</span>
                            <span class="dim">${escape(candidate.retort || "")}</span>
                        </button>`).join("")}
                    </div>
                    <div class="start-grid">
                        <label>name <input type="text" maxlength="20" placeholder="${escape(choice.wizard)}" value="${escape(choice.wizardName)}" data-name></label>
                        <label>race <select data-race>
                            <option value="">random</option>
                            ${races.map((race) => `<option value="${escape(race.name)}"${race.name === choice.race ? " selected" : ""}>${escape(race.name)}${race.myrran ? " (Myrror)" : ""}</option>`).join("")}
                        </select></label>
                        <label>banner <span class="start-banners">
                            <button class="swatch${choice.banner === "" ? " chosen" : ""}" data-banner="" title="random">?</button>
                            ${options.banners.map((banner) => `<button class="swatch${banner === choice.banner ? " chosen" : ""}" data-banner="${escape(banner)}" title="${escape(banner)}" style="background: ${bannerColor(banner)}"></button>`).join("")}
                        </span></label>
                        <label>opponents <select data-opponents>${Array.from({ length: options["max-opponents"] - options["min-opponents"] + 1 }, (_, index) => options["min-opponents"] + index)
                            .map((count) => `<option${count === choice.opponents ? " selected" : ""}>${count}</option>`).join("")}</select></label>
                        <label>difficulty <select data-difficulty>${options.difficulties.map((name) => `<option${name === choice.difficulty ? " selected" : ""}>${escape(name)}</option>`).join("")}</select></label>
                        <label>magic <select data-magic>${options.magic.map((name) => `<option${name === choice.magic ? " selected" : ""}>${escape(name)}</option>`).join("")}</select></label>
                        <label>land <select data-land>${options["land-sizes"].map((name, index) => `<option value="${index}"${index === choice.landSize ? " selected" : ""}>${escape(name)}</option>`).join("")}</select></label>
                    </div>
                    <div class="row"><span class="dim">${wizard.myrran ? "Myrran retort: a Myrror race starts on Myrror" : ""}</span><button data-start>Start</button></div>
                </div>`;

            const finish = (result) => {
                root.innerHTML = "";
                root.hidden = true;
                resolve(result);
            };
            for (const button of root.querySelectorAll("[data-continue]")) {
                button.onclick = () => finish({ saveSlot: button.dataset.continue });
            }
            root.querySelector("[data-load-file]").onclick = async () => {
                const text = await pickSaveFile();
                if (text) finish({ saveText: text });
            };
            for (const button of root.querySelectorAll("[data-wizard]")) {
                button.onclick = () => { choice.wizard = button.dataset.wizard; render(); };
            }
            for (const button of root.querySelectorAll("[data-banner]")) {
                button.onclick = () => { choice.banner = button.dataset.banner; render(); };
            }
            root.querySelector("[data-name]").oninput = (event) => { choice.wizardName = event.target.value; };
            root.querySelector("[data-race]").onchange = (event) => { choice.race = event.target.value; };
            root.querySelector("[data-opponents]").onchange = (event) => { choice.opponents = Number(event.target.value); };
            root.querySelector("[data-difficulty]").onchange = (event) => { choice.difficulty = event.target.value; };
            root.querySelector("[data-magic]").onchange = (event) => { choice.magic = event.target.value; };
            root.querySelector("[data-land]").onchange = (event) => { choice.landSize = Number(event.target.value); };
            root.querySelector("[data-start]").onclick = () => {
                const settings = {
                    wizard: choice.wizard,
                    opponents: choice.opponents,
                    difficulty: choice.difficulty,
                    magic: choice.magic,
                    landSize: choice.landSize,
                    raiders: true,
                };
                if (choice.race) settings.race = choice.race;
                if (choice.banner) settings.banner = choice.banner;
                if (choice.wizardName.trim()) settings.wizardName = choice.wizardName.trim();
                remember({ ...choice, wizardName: undefined });
                finish(settings);
            };
        };
        root.hidden = false;
        render();
    });
}
