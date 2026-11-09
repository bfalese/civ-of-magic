// The game core (Go, compiled to wasm) behind a small JS API. See docs/frontend-protocol.md for
// the commands, events and views. Every call returns parsed JSON.

export async function loadCore(wasmUrl) {
    const go = new Go();
    const ready = new Promise((resolve) => {
        window.onCivReady = resolve;
    });
    const result = await WebAssembly.instantiateStreaming(fetch(wasmUrl), go.importObject);
    go.run(result.instance);
    await ready;
    return new Core();
}

export class Core {
    call(name, ...args) {
        return JSON.parse(globalThis.civ[name](...args));
    }

    newGame(gamedataText, settings) {
        return this.call("newGame", gamedataText, JSON.stringify(settings));
    }

    // the game as a JSON string (to keep), or throws with the core's reason
    save(name) {
        const text = globalThis.civ.save(name);
        if (text.startsWith('{"error"')) {
            throw new Error(JSON.parse(text).error);
        }
        return text;
    }

    // a saved game (the text save() gave) replaces the current one; returns a status
    load(gamedataText, saveText) {
        return this.call("load", gamedataText, saveText);
    }

    // a command {type, command}; returns {error?, waiting, gameOver?}
    execute(type, command = {}) {
        return this.call("execute", JSON.stringify({ type, command }));
    }

    events() {
        return this.call("events");
    }

    view() {
        return this.call("view");
    }

    map(plane) {
        return this.call("map", plane);
    }

    battle() {
        return this.call("battle");
    }

    // what the surveyor tells about an explored tile (core TileInfoView), or {error}
    tileInfo(location) {
        return this.call("tileInfo", JSON.stringify(location));
    }

    // the road build-road would make from the stack to a tile: a list of locations, or {error}
    roadPath(stack, to) {
        return this.call("roadPath", JSON.stringify({ stack, to }));
    }

    // the choices for a new game: {wizards, races, banners, difficulties, magic, land-sizes, ...}
    startOptions() {
        return this.call("startOptions");
    }

    // the custom wizard builder's choices so far: {picks-left, error?, retorts: {name: why not}, ready}
    checkWizard(custom) {
        return this.call("checkWizard", JSON.stringify(custom));
    }

    setLogging(on) {
        this.call("setLogging", on);
    }
}
