// Plays a game through the wasm core under Node: a check of the JS <-> Go round trip without a
// browser.
//
//   make core-wasm
//   node frontend/wasm/smoke.cjs frontend/wasm/core.wasm ~/.local/share/civ-of-magic/gamedata/gamedata.json [turns]
"use strict";

const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

if (process.argv.length < 4) {
    console.error("usage: node smoke.cjs <core.wasm> <gamedata.json> [turns]");
    process.exit(1);
}

const wasmPath = process.argv[2];
const gamedataPath = process.argv[3];
const turns = parseInt(process.argv[4] || "20", 10);

globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto");
const goRoot = execSync("go env GOROOT").toString().trim();
require(path.join(goRoot, "lib", "wasm", "wasm_exec.js"));

function call(name, ...args) {
    const result = JSON.parse(globalThis.civ[name](...args));
    if (result && result.error) {
        throw new Error(`${name}: ${result.error}`);
    }
    return result;
}

function run() {
    const started = Date.now();
    let status = call("newGame", fs.readFileSync(gamedataPath, "utf8"), JSON.stringify({ opponents: 3, wizardName: "Smoke" }));
    console.log(`newGame: ${JSON.stringify(status)} (${Date.now() - started} ms)`);

    const view = call("view");
    console.log(`you: ${JSON.stringify(view.player || view.you || Object.keys(view))}`);
    const map = call("map", "Arcanus");
    console.log(`map view keys: ${Object.keys(map).join(", ")}`);

    const counts = {};
    const countEvents = () => {
        for (const event of call("events")) {
            counts[event.type] = (counts[event.type] || 0) + 1;
        }
    };
    countEvents();

    call("execute", JSON.stringify({ type: "set-tax-rate", command: { numerator: 3, denominator: 2 } }));

    const refused = JSON.parse(globalThis.civ.execute(JSON.stringify({ type: "no-such-command", command: {} })));
    if (!refused.error) {
        throw new Error("an unknown command should be refused");
    }

    for (let turn = 0; turn < turns; turn++) {
        status = call("execute", JSON.stringify({ type: "end-turn", command: {} }));
        countEvents();
        if (status.gameOver) {
            break;
        }
    }

    const after = call("view");
    console.log(`after ${turns} turns: ${JSON.stringify(status)}, events ${JSON.stringify(counts)}`);
    console.log(`view keys: ${Object.keys(after).join(", ")}`);
    console.log(`total ${Date.now() - started} ms`);
    process.exit(0);
}

globalThis.onCivReady = () => {
    try {
        run();
    } catch (error) {
        console.error(error);
        process.exit(1);
    }
};

const go = new Go();
WebAssembly.instantiate(fs.readFileSync(wasmPath), go.importObject).then((result) => go.run(result.instance)).catch((error) => {
    console.error(error);
    process.exit(1);
});
