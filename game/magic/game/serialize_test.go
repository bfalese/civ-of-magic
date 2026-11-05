package game

import (
    "encoding/json"
    "testing"

    "github.com/bfalese/civ-of-magic/game/magic/keybinds"
    settingslib "github.com/bfalese/civ-of-magic/game/magic/settings"
    "github.com/bfalese/civ-of-magic/core/setup"
    gamecore "github.com/bfalese/civ-of-magic/core/game"

    "github.com/hajimehoshi/ebiten/v2"
)

func TestSerializedGameOldJSONLeavesPreferencesNil(test *testing.T) {
    var serialized SerializedGame
    err := json.Unmarshal([]byte(`{"metadata":{"version":1,"name":"Gabe"},"settings":{"Difficulty":2,"Opponents":2,"LandSize":2,"Magic":1},"turn":1}`), &serialized)
    if err != nil {
        test.Fatalf("unmarshal: %v", err)
    }
    if serialized.Preferences != nil {
        test.Errorf("old save must not invent a preferences object")
    }
    if serialized.Settings.Difficulty != 2 {
        test.Errorf("new-game settings should still decode")
    }
}

func TestSerializedGamePreferencesRoundTrip(test *testing.T) {
    prefs := settingslib.MakeSettings(nil)
    prefs.EndOfTurnWait = false
    prefs.StrategicCombatOnly = true
    prefs.Keybindings.Set(keybinds.ActionNextTurn, ebiten.KeySpace)

    original := SerializedGame{
        SerializedGame: gamecore.SerializedGame{
            Settings: setup.NewGameSettings{Opponents: 3},
            Turn: 12,
        },
        Preferences: settingslib.SerializeSettings(prefs),
    }

    raw, err := json.Marshal(original)
    if err != nil {
        test.Fatalf("marshal: %v", err)
    }

    var loaded SerializedGame
    if err := json.Unmarshal(raw, &loaded); err != nil {
        test.Fatalf("unmarshal: %v", err)
    }
    if loaded.Turn != 12 || loaded.Settings.Opponents != 3 {
        test.Errorf("game fields: turn=%d opponents=%d", loaded.Turn, loaded.Settings.Opponents)
    }
    if loaded.Preferences == nil {
        test.Fatalf("preferences missing after round trip")
    }

    restored := settingslib.MakeSettings(nil)
    loaded.Preferences.Apply(restored)
    if restored.EndOfTurnWait || !restored.StrategicCombatOnly {
        test.Errorf("bools: wait=%v strategic=%v", restored.EndOfTurnWait, restored.StrategicCombatOnly)
    }
    if restored.Keybindings.Get(keybinds.ActionNextTurn) != ebiten.KeySpace {
        test.Errorf("keybindings did not round trip")
    }
}

// the core state is embedded; the save file must stay flat (same layout as before the split)
func TestSerializedGameJSONIsFlat(test *testing.T) {
    original := SerializedGame{
        SerializedGame: gamecore.SerializedGame{Turn: 7},
        Preferences: settingslib.SerializeSettings(settingslib.MakeSettings(nil)),
    }

    raw, err := json.Marshal(original)
    if err != nil {
        test.Fatalf("marshal: %v", err)
    }

    var keys map[string]json.RawMessage
    if err := json.Unmarshal(raw, &keys); err != nil {
        test.Fatalf("unmarshal: %v", err)
    }
    for _, key := range []string{"metadata", "settings", "turn", "players", "preferences"} {
        if _, ok := keys[key]; !ok {
            test.Errorf("top-level key %q missing", key)
        }
    }
    if _, ok := keys["SerializedGame"]; ok {
        test.Errorf("core state must not be nested under SerializedGame")
    }
}
