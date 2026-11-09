package game

import (
    "encoding/json"
    "io"
    "log"
    "os"
    "reflect"
    "strings"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
)

// play turns through the host, answering battles automatically and the research question
func playHostTurns(test *testing.T, host *GameHost, turns int) {
    for range turns {
        if host.GameOver() != "" {
            return
        }
        if candidates := host.Model.MakePlayerView(host.Human).You.ResearchCandidates; host.Human.ResearchingSpell.Name == "" && len(candidates) > 0 {
            if err := host.Execute(&SetResearchCommand{Spell: candidates[0]}); err != nil {
                test.Fatalf("research: %v", err)
            }
        }
        if err := host.Execute(&EndTurnCommand{}); err != nil {
            test.Fatalf("end turn at %v: %v", host.Model.TurnNumber, err)
        }
        for host.WaitingFor() == WaitingBattle {
            if err := host.Execute(&BattleAutoCommand{}); err != nil {
                test.Fatalf("auto battle: %v", err)
            }
        }
        host.TakeEvents()
    }
}

// the player view without unit ids (reassigned on load) and the save's own fields, as JSON
func comparableView(test *testing.T, host *GameHost) map[string]any {
    encoded, err := host.PlayerView()
    if err != nil {
        test.Fatalf("player view: %v", err)
    }
    var view map[string]any
    if err := json.Unmarshal(encoded, &view); err != nil {
        test.Fatalf("player view JSON: %v", err)
    }
    var strip func(value any)
    strip = func(value any) {
        switch value := value.(type) {
            case map[string]any:
                delete(value, "id")
                delete(value, "unit")
                for _, inner := range value {
                    strip(inner)
                }
            case []any:
                for _, inner := range value {
                    strip(inner)
                }
        }
    }
    strip(view)
    return view
}

// save after a few turns, load with the same data: the views match, and the loaded game plays on
func TestHostSaveLoad(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 2, Raiders: true, Scenario: "items"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    // the scenario's shield waits in the vault: the turn can't end until it's dealt with
    if err := host.Execute(&EndTurnCommand{}); err != ErrNewItemsWaiting {
        test.Fatalf("end-turn with a new item waiting should be refused, got %v", err)
    }
    shield := host.Model.PendingItems[host.Human][0]
    zero := 0
    if err := host.Execute(&DestroyItemCommand{Item: ItemSlot{New: &zero}}); err != nil {
        test.Fatalf("destroy the shield: %v", err)
    }
    playHostTurns(test, host, 8)
    if host.GameOver() != "" {
        test.Skip("the game ended early")
    }
    // an item waiting to be placed is kept by the save
    host.Model.recordVaultItem(&GameEventVault{CreatedArtifact: shield, Player: host.Human})

    saved, err := host.Save("test")
    if err != nil {
        test.Fatalf("save: %v", err)
    }
    loaded, err := LoadGameHost(gameData, saved)
    if err != nil {
        test.Fatalf("load: %v", err)
    }

    before, after := comparableView(test, host), comparableView(test, loaded)
    for _, key := range []string{"turn", "date", "you", "cities", "stacks", "known-wizards", "diplomacy", "foreign-cities", "foreign-stacks"} {
        if !reflect.DeepEqual(before[key], after[key]) {
            beforeJSON, _ := json.Marshal(before[key])
            afterJSON, _ := json.Marshal(after[key])
            test.Errorf("%v differs after loading:\n  before %.600s\n  after  %.600s", key, beforeJSON, afterJSON)
        }
    }
    for _, plane := range []data.Plane{data.PlaneArcanus, data.PlaneMyrror} {
        mapBefore, _ := host.MapView(plane)
        mapAfter, _ := loaded.MapView(plane)
        if string(mapBefore) != string(mapAfter) {
            test.Errorf("the %v map differs after loading", plane)
        }
    }
    if len(loaded.Model.PendingItems[loaded.Human]) != 1 {
        test.Errorf("the scenario's new item should still wait in the vault, got %v", len(loaded.Model.PendingItems[loaded.Human]))
    }

    // the loaded game plays on, and its units have ids for commands
    view := loaded.Model.MakePlayerView(loaded.Human)
    if len(view.Stacks) == 0 || view.Stacks[0].Units[0].Id == 0 {
        test.Fatalf("the loaded units should have ids")
    }
    if err := loaded.Execute(&DestroyItemCommand{Item: ItemSlot{New: &zero}}); err != nil {
        test.Fatalf("destroying the loaded new item: %v", err)
    }
    turn := loaded.Model.TurnNumber
    playHostTurns(test, loaded, 5)
    if loaded.GameOver() == "" && loaded.Model.TurnNumber != turn + 5 {
        test.Errorf("the loaded game should play on: turn %v -> %v", turn, loaded.Model.TurnNumber)
    }

    if _, err := LoadGameHost(gameData, []byte(`{"version": 99}`)); err == nil || !strings.Contains(err.Error(), "version") {
        test.Errorf("another save version should be refused: %v", err)
    }
}

// TakeEvents passes on only what concerns the human: no AI player's research, offers or battles
func TestHostEventsConcernHuman(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 4, Raiders: true})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    type ref struct {
        Human bool `json:"human"`
    }
    count := 0
    check := func() {
        for _, encoded := range host.TakeEvents() {
            count += 1
            var event struct {
                Type string `json:"type"`
                Event struct {
                    Player *ref `json:"player"`
                    Attacker *ref `json:"attacker"`
                    Defender *ref `json:"defender"`
                } `json:"event"`
            }
            if err := json.Unmarshal(encoded, &event); err != nil {
                test.Fatalf("event JSON: %v", err)
            }
            body := event.Event
            if body.Player != nil && !body.Player.Human {
                test.Errorf("an event for another player reached the frontend: %s", encoded)
            }
            if strings.HasPrefix(event.Type, "battle-") && !(body.Attacker != nil && body.Attacker.Human) && !(body.Defender != nil && body.Defender.Human) {
                test.Errorf("a battle without the human reached the frontend: %s", encoded)
            }
        }
    }
    check()
    for range 25 {
        if host.GameOver() != "" {
            break
        }
        if candidates := host.Model.MakePlayerView(host.Human).You.ResearchCandidates; host.Human.ResearchingSpell.Name == "" && len(candidates) > 0 {
            host.Execute(&SetResearchCommand{Spell: candidates[0]})
        }
        if err := host.Execute(&EndTurnCommand{}); err != nil {
            test.Fatalf("end turn: %v", err)
        }
        for host.WaitingFor() == WaitingBattle {
            host.Execute(&BattleAutoCommand{})
        }
        check()
    }
    if count == 0 {
        test.Errorf("the human should have had some events")
    }
}
