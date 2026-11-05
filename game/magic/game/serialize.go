package game

import (
    gamecore "github.com/bfalese/civ-of-magic/core/game"
    settingslib "github.com/bfalese/civ-of-magic/game/magic/settings"
)

// a save file: the core game state plus this frontend's preferences. Embedding keeps the JSON
// layout of older saves (encoding/json flattens embedded structs).
type SerializedGame struct {
    gamecore.SerializedGame

    // remake UI prefs (end-of-turn wait, keybindings, ...). Omitted in
    // saves written before this field existed; json "settings" is the
    // new-game difficulty/land/magic struct, not this.
    Preferences *settingslib.SerializedSettings `json:"preferences,omitempty"`
}

func SerializeModel(model *gamecore.GameModel, saveName string, prefs *settingslib.Settings) SerializedGame {
    return SerializedGame{
        SerializedGame: gamecore.SerializeModel(model, saveName),
        Preferences: settingslib.SerializeSettings(prefs),
    }
}
