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

    // the road build-road would make from the stack to a tile: a list of locations, or {error}
    roadPath(stack, to) {
        return this.call("roadPath", JSON.stringify({ stack, to }));
    }

    // the choices for a new game: {wizards, races, banners, difficulties, magic, land-sizes, ...}
    startOptions() {
        return this.call("startOptions");
    }

    setLogging(on) {
        this.call("setLogging", on);
    }
}
