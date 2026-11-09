// The start screen: pick a premade wizard, the race of your first city, a banner colour, the
// number of opponents, difficulty, magic strength and land size, from the core's start options
// (core StartOptions), or a custom wizard: a portrait, spellbooks and retorts for the 11 picks,
// checked by the core as you choose (checkWizard). Resolves to the settings for newGame. The last choices are remembered in
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

const CUSTOM = "Custom";

export function chooseStart(root, options, assets, saves = [], checkWizard = null) {
    const last = remembered();
    const builder = options.custom && checkWizard ? options.custom : null;
    // the premade wizards, and the builder's card
    const wizards = builder ? [...options.wizards, { name: CUSTOM, books: {}, retort: "your own books and retorts", custom: true }] : options.wizards;
    const lastCustom = last.custom && typeof last.custom === "object" ? last.custom : {};
    const choice = {
        wizard: wizards.some((wizard) => wizard.name === last.wizard) ? last.wizard : options.wizards[0].name,
        custom: {
            portrait: Number.isInteger(lastCustom.portrait) ? lastCustom.portrait : 0,
            books: lastCustom.books && typeof lastCustom.books === "object" ? { ...lastCustom.books } : {},
            retorts: Array.isArray(lastCustom.retorts) ? [...lastCustom.retorts] : [],
        },
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

    // the custom wizard's portrait, books and retorts
    const builderHtml = (check) => {
        const left = check["picks-left"];
        const level = (realm) => choice.custom.books[realm] || 0;
        const blocked = (realm) => (realm === "Life" && level("Death") > 0) || (realm === "Death" && level("Life") > 0);
        return `<div class="start-builder">
            <div class="row"><h3>Custom wizard</h3><span class="${left === 0 ? "" : "dim"}">${left} of ${builder.picks} picks left</span></div>
            <div class="start-portraits">${builder.portraits.map((portrait) => `<button class="start-portrait-choice${portrait.portrait === choice.custom.portrait ? " chosen" : ""}" data-portrait="${portrait.portrait}" title="portrait ${portrait.portrait + 1}"></button>`).join("")}</div>
            <div class="start-books">${builder.realms.map((realm) => `<span class="start-book">
                <span style="color: ${realmColors[realm] || ""}">${escape(realm)}</span>
                <button class="small" data-book="${escape(realm)}" data-step="-1"${level(realm) === 0 ? " disabled" : ""}>−</button>
                <b>${level(realm)}</b>
                <button class="small" data-book="${escape(realm)}" data-step="1"${left <= 0 || level(realm) >= 11 || blocked(realm) ? " disabled" : ""} title="${blocked(realm) ? "Life and Death don't mix" : ""}">+</button>
            </span>`).join("")}</div>
            <div class="start-retorts">${builder.retorts.map((retort) => {
                const chosen = choice.custom.retorts.includes(retort.name);
                const why = check.retorts[retort.name] || "";
                return `<button class="small${chosen ? " chosen" : ""}" data-retort="${escape(retort.name)}"${!chosen && why ? " disabled" : ""} title="${escape(why || retort.requires || "")}">${escape(retort.name)} <span class="dim">${retort.picks}</span></button>`;
            }).join("")}</div>
            ${check.error ? `<div class="start-error">${escape(check.error)}</div>` : ""}
        </div>`;
    };

    return new Promise((resolve) => {
        let render;
        const bindBuilder = () => {
            for (const button of root.querySelectorAll("[data-portrait]")) {
                const asset = builder.portraits.find((portrait) => portrait.portrait === Number(button.dataset.portrait)).asset;
                const picture = assets.picture(asset, 0.4);
                if (picture) button.appendChild(picture); else button.textContent = Number(button.dataset.portrait) + 1;
                button.onclick = () => { choice.custom.portrait = Number(button.dataset.portrait); render(); };
            }
            for (const button of root.querySelectorAll("[data-book]")) {
                button.onclick = () => {
                    const realm = button.dataset.book;
                    choice.custom.books[realm] = Math.max(0, Math.min(11, (choice.custom.books[realm] || 0) + Number(button.dataset.step)));
                    // retorts whose books are gone go too
                    const check = checkWizard({ ...choice.custom, name: CUSTOM });
                    choice.custom.retorts = choice.custom.retorts.filter((name) => !check.retorts[name]);
                    render();
                };
            }
            for (const button of root.querySelectorAll("[data-retort]")) {
                button.onclick = () => {
                    const name = button.dataset.retort;
                    choice.custom.retorts = choice.custom.retorts.includes(name) ? choice.custom.retorts.filter((other) => other !== name) : [...choice.custom.retorts, name];
                    render();
                };
            }
        };
        render = () => {
            const wizard = wizards.find((candidate) => candidate.name === choice.wizard);
            const custom = wizard.custom;
            const customSettings = () => ({ ...choice.custom, name: choice.wizardName.trim() || CUSTOM });
            const check = custom ? checkWizard(customSettings()) : null;
            const myrran = custom ? choice.custom.retorts.includes("Myrran") : wizard.myrran;
            const books = custom ? Object.fromEntries(Object.entries(choice.custom.books).filter(([, count]) => count > 0)) : wizard.books;
            const retortText = custom ? choice.custom.retorts.join(", ") : (wizard.retort || "");
            const portraitAsset = custom ? (builder.portraits.find((portrait) => portrait.portrait === choice.custom.portrait) || builder.portraits[0]).asset : wizard.portrait;
            const races = options.races.filter((race) => !race.myrran || myrran);
            if (choice.race && !races.some((race) => race.name === choice.race)) choice.race = "";

            root.innerHTML = `
                <div class="panel">
                    <div class="start-continue">
                        ${saves.map((save) => `<button data-continue="${escape(save.slot)}"><b>Continue</b> <span class="dim">${escape(slotNames[save.slot])}: ${escape(describeSave(save))}</span></button>`).join("")}
                        <button data-load-file>Load a save file…</button>
                    </div>
                    <div class="start-head">
                        <span class="start-portrait"></span>
                        <div>
                            <h2>New game</h2>
                            <div><b>${escape(custom ? (choice.wizardName.trim() || CUSTOM) : wizard.name)}</b></div>
                            <div>${Object.entries(books).map(([realm, count]) => `<span style="color: ${realmColors[realm] || ""}">${count} ${escape(realm)}</span>`).join(" ") || `<span class="dim">no spellbooks yet</span>`}</div>
                            <div class="dim">${escape(retortText)}</div>
                        </div>
                    </div>
                    <h3>Wizard</h3>
                    <div class="start-wizards">${wizards.map((candidate) => `
                        <button class="start-wizard${candidate.name === choice.wizard ? " chosen" : ""}" data-wizard="${escape(candidate.name)}">
                            <b>${escape(candidate.name)}</b>
                            <span>${Object.entries(candidate.books).map(([realm, count]) => `<span style="color: ${realmColors[realm] || ""}">${count} ${escape(realm)}</span>`).join(" ")}</span>
                            <span class="dim">${escape(candidate.retort || "")}</span>
                        </button>`).join("")}
                    </div>
                    ${custom ? builderHtml(check) : ""}
                    <div class="start-grid">
                        <label>name <input type="text" maxlength="20" placeholder="${escape(custom ? CUSTOM : choice.wizard)}" value="${escape(choice.wizardName)}" data-name></label>
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
                    <div class="row"><span class="dim">${myrran ? "Myrran retort: a Myrror race starts on Myrror" : ""}</span><button data-start${check && !check.ready ? ` disabled title="${escape(check.error || `${check["picks-left"]} picks left`)}"` : ""}>Start</button></div>
                </div>`;

            const portrait = assets.picture(portraitAsset, 1);
            if (portrait) root.querySelector(".start-portrait").appendChild(portrait);
            if (custom) bindBuilder(check);
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
            root.querySelector("[data-name]").oninput = (event) => {
                choice.wizardName = event.target.value;
                // the custom wizard's name shows in the heading; keep the focus in the field
                if (custom) {
                    root.querySelector(".start-head b").textContent = choice.wizardName.trim() || CUSTOM;
                    const start = root.querySelector("[data-start]");
                    const ready = checkWizard(customSettings());
                    start.disabled = !ready.ready;
                }
            };
            root.querySelector("[data-race]").onchange = (event) => { choice.race = event.target.value; };
            root.querySelector("[data-opponents]").onchange = (event) => { choice.opponents = Number(event.target.value); };
            root.querySelector("[data-difficulty]").onchange = (event) => { choice.difficulty = event.target.value; };
            root.querySelector("[data-magic]").onchange = (event) => { choice.magic = event.target.value; };
            root.querySelector("[data-land]").onchange = (event) => { choice.landSize = Number(event.target.value); };
            root.querySelector("[data-start]").onclick = () => {
                const settings = {
                    opponents: choice.opponents,
                    difficulty: choice.difficulty,
                    magic: choice.magic,
                    landSize: choice.landSize,
                    raiders: true,
                };
                if (custom) {
                    settings.custom = customSettings();
                } else {
                    settings.wizard = choice.wizard;
                }
                if (choice.race) settings.race = choice.race;
                if (choice.banner) settings.banner = choice.banner;
                if (choice.wizardName.trim()) settings.wizardName = choice.wizardName.trim();
                remember({ ...choice, wizardName: undefined, custom: choice.custom });
                finish(settings);
            };
        };
        root.hidden = false;
        render();
    });
}
