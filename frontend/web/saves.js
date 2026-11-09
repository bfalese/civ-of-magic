// Saved games in this browser: an autosave at the start of every turn and one save slot, gzipped
// into localStorage (a large map's save is ~2 MB as JSON), plus save files to download and load.
// Storage can fail (private windows, blocked or full storage): every call copes and says so.

const prefix = "civ-of-magic-save-";
export const slots = ["autosave", "save"];
export const slotNames = { autosave: "Autosave", save: "Saved game" };

async function gzip(text) {
    const stream = new Blob([text]).stream().pipeThrough(new CompressionStream("gzip"));
    const bytes = new Uint8Array(await new Response(stream).arrayBuffer());
    let binary = "";
    for (let index = 0; index < bytes.length; index += 0x8000) {
        binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000));
    }
    return btoa(binary);
}

async function gunzip(base64) {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index++) {
        bytes[index] = binary.charCodeAt(index);
    }
    const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream("gzip"));
    return new Response(stream).text();
}

// what the start screen and the game menu show of a slot: {slot, wizard, date, saved}, or null
export function saveInfo(slot) {
    try {
        const info = JSON.parse(localStorage.getItem(`${prefix}${slot}-info`) || "null");
        return info && localStorage.getItem(`${prefix}${slot}`) ? { slot, ...info } : null;
    } catch (error) {
        return null;
    }
}

export function savedGames() {
    return slots.map(saveInfo).filter(Boolean).sort((a, b) => b.saved - a.saved);
}

// keep a save (the core's text) in a slot; info is {wizard, date}. Resolves to an error message, or
// null when it was kept.
export async function storeSave(slot, text, info) {
    try {
        const packed = await gzip(text);
        localStorage.setItem(`${prefix}${slot}`, packed);
        localStorage.setItem(`${prefix}${slot}-info`, JSON.stringify({ ...info, saved: Date.now() }));
        return null;
    } catch (error) {
        return `couldn't keep the save in this browser (${error.name || error.message})`;
    }
}

// the save text of a slot, or throws
export async function readSave(slot) {
    const packed = localStorage.getItem(`${prefix}${slot}`);
    if (!packed) {
        throw new Error(`no ${slotNames[slot] || slot}`);
    }
    return gunzip(packed);
}

export function downloadSave(text, name) {
    const link = document.createElement("a");
    link.href = URL.createObjectURL(new Blob([text], { type: "application/json" }));
    link.download = `${name.replace(/[^\w -]+/g, "").trim() || "civ-of-magic"}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}

// asks for a save file; resolves to its text, or null when the player picks nothing
export function pickSaveFile() {
    return new Promise((resolve) => {
        const input = document.createElement("input");
        input.type = "file";
        input.accept = ".json,application/json";
        input.onchange = () => {
            const file = input.files && input.files[0];
            resolve(file ? file.text() : null);
        };
        input.oncancel = () => resolve(null);
        input.click();
    });
}

export function describeSave(info) {
    const when = new Date(info.saved).toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" });
    return `${info.wizard}, ${info.date} · ${when}`;
}
