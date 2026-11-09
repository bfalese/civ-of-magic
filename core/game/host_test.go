package game

import (
    "encoding/json"
    "io"
    "log"
    "os"
    "testing"

    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

func TestHostSettings(test *testing.T) {
    settings, err := DecodeHostSettings(nil)
    if err != nil || settings.Opponents != 3 || !settings.Raiders {
        test.Errorf("defaults: %+v %v", settings, err)
    }

    settings, err = DecodeHostSettings([]byte(`{"opponents": 2, "difficulty": "hard", "wizardName": "Tester"}`))
    if err != nil || settings.Opponents != 2 || settings.Difficulty != "hard" || settings.WizardName != "Tester" {
        test.Errorf("decoded: %+v %v", settings, err)
    }

    if _, err := parseDifficulty("nightmare"); err == nil {
        test.Errorf("unknown difficulty should be refused")
    }
}

// a human player through JSON commands for a few decades of AI turns. Needs the exported data
// tables: CIV_GAMEDATA=<dir with gamedata.json>; skipped otherwise.
func TestGameHost(test *testing.T) {
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

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 2, Raiders: true, WizardName: "Tester"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }

    if host.Human.Wizard.Name != "Tester" || len(host.Human.Cities) != 1 || len(host.Human.Stacks) == 0 {
        test.Fatalf("the human should start with a capital and units")
    }
    if host.Model.CurrentPlayer != 0 {
        test.Fatalf("the human plays first")
    }

    view, err := host.PlayerView()
    if err != nil {
        test.Fatalf("player view: %v", err)
    }
    var decoded map[string]any
    if err := json.Unmarshal(view, &decoded); err != nil {
        test.Fatalf("player view JSON: %v", err)
    }
    if _, err := host.MapView(data.PlaneArcanus); err != nil {
        test.Errorf("map view: %v", err)
    }
    if _, err := host.BattleView(); err == nil {
        test.Errorf("no battle yet")
    }

    if err := host.ExecuteJSON([]byte(`{"type": "set-tax-rate", "command": {"numerator": 1, "denominator": 1}}`)); err != nil {
        test.Errorf("tax: %v", err)
    }
    if err := host.ExecuteJSON([]byte(`{"type": "no-such-command", "command": {}}`)); err == nil {
        test.Errorf("unknown commands should be refused")
    }

    events := len(host.TakeEvents())
    start := host.Model.TurnNumber
    attacked := 0
    for range 30 {
        if host.GameOver() != "" {
            break
        }
        // answer anything left waiting (a finished spell's target is optional)
        if host.WaitingFor() != WaitingNothing {
            test.Fatalf("waiting for %v outside a command", host.WaitingFor())
        }
        if err := host.ExecuteJSON([]byte(`{"type": "end-turn", "command": {}}`)); err != nil {
            test.Fatalf("end turn at %v: %v", host.Model.TurnNumber, err)
        }
        // an AI player attacked: the AI turns wait for the battle
        for host.WaitingFor() == WaitingBattle {
            attacked += 1
            if err := host.Execute(&BattleAutoCommand{}); err != nil {
                test.Fatalf("auto battle: %v", err)
            }
        }
        if host.GameOver() == "" && host.Model.CurrentPlayer != 0 {
            test.Fatalf("after end-turn it should be the human's turn again, not player %v", host.Model.CurrentPlayer)
        }
        events += len(host.TakeEvents())
    }

    if host.Model.TurnNumber < start + 30 && host.GameOver() == "" {
        test.Errorf("expected 30 turns, played %v", host.Model.TurnNumber - start)
    }
    test.Logf("turn %v, %v events, game over %q, human cities %v, attacked %v times", host.Model.TurnNumber, events, host.GameOver(), len(host.Human.Cities), attacked)

    for _, err := range host.EventErrors {
        test.Errorf("event: %v", err)
    }
    if events == 0 {
        test.Errorf("expected some events")
    }
}

// an AI player's attack on the human during end-turn pauses the AI turns for the battle; the rest
// of the turn runs once the battle is over
func TestHostAiBattle(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    host := &GameHost{Model: model, Human: human, ai: &HeadlessGame{Model: model}}
    host.aiHandlers = &hostAiHandlers{HeadlessGame: host.ai, host: host}
    host.Session = MakeCommandSession(model, human)

    // the end of the turn: the enemy's spearmen attack the human's drakes, then the turn goes on
    finished := false
    var result combat.CombatState
    host.Session.EndTurn = func() {
        attackers := enemy.FindStack(4, 3, data.PlaneArcanus)
        defenders := human.FindStack(3, 3, data.PlaneArcanus)
        result = host.aiHandlers.DoCombat(enemy, attackers, human, defenders, combat.ZoneType{})
        finished = true
    }

    if err := host.Execute(&EndTurnCommand{}); err != nil {
        test.Fatalf("end turn: %v", err)
    }
    if host.WaitingFor() != WaitingBattle || finished {
        test.Fatalf("the attack should wait for the human's orders, waiting for %q", host.WaitingFor())
    }
    encoded, err := host.BattleView()
    if err != nil {
        test.Fatalf("battle view: %v", err)
    }
    var view BattleView
    if err := json.Unmarshal(encoded, &view); err != nil {
        test.Fatalf("battle view JSON: %v", err)
    }
    if view.YouAttack {
        test.Errorf("the human defends")
    }
    if err := host.Execute(&EndTurnCommand{}); err == nil {
        test.Errorf("other commands should wait for the battle")
    }

    // a few orders by hand, then the AI fights it out for the human
    for range 3 {
        if host.WaitingFor() != WaitingBattle {
            break
        }
        if err := host.Execute(&BattleDoneCommand{}); err != nil {
            test.Fatalf("done: %v", err)
        }
    }
    if host.WaitingFor() == WaitingBattle {
        if err := host.Execute(&BattleAutoCommand{}); err != nil {
            test.Fatalf("auto: %v", err)
        }
    }
    if host.WaitingFor() != WaitingNothing || !finished {
        test.Fatalf("after the battle the turn should finish, waiting for %q", host.WaitingFor())
    }
    if result != combat.CombatStateDefenderWin {
        test.Errorf("four great drakes should hold against spearmen, got %v", battleStateName(result))
    }
    if len(enemy.Stacks) != 0 || len(human.Stacks) == 0 {
        test.Errorf("the attackers should be gone, the drakes still there")
    }
}

// the battle scenario: a guarded lair next to the capital and Fire Bolt to cast there (10 games,
// since the maps are random)
func TestHostBattleScenario(test *testing.T) {
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

    if _, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "nonsense"}); err == nil {
        test.Errorf("an unknown scenario should be refused")
    }

    for range 10 {
        host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "battle"})
        if err != nil {
            test.Fatalf("setup: %v", err)
        }
        capital := host.Human.FindFortressCity()
        mapUse := host.Model.GetMap(capital.Plane)
        var lair *Location
        for _, offset := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
            x, y := mapUse.WrapX(capital.X + offset[0]), capital.Y + offset[1]
            if mapUse.GetEncounter(x, y) != nil {
                lair = &Location{Plane: capital.Plane, X: x, Y: y}
            }
        }
        if lair == nil {
            test.Fatalf("no lair next to the capital")
        }

        stack := host.Human.FindStack(capital.X, capital.Y, capital.Plane)
        if err := host.Execute(&MoveStackCommand{From: Location{Plane: capital.Plane, X: capital.X, Y: capital.Y}, To: *lair}); err != nil {
            test.Fatalf("move: %v", err)
        }
        if host.WaitingFor() != WaitingEncounter || stack == nil {
            test.Fatalf("walking into the lair should ask first, waiting for %q", host.WaitingFor())
        }
        if err := host.Execute(&ConfirmEncounterCommand{Enter: true}); err != nil {
            test.Fatalf("enter: %v", err)
        }
        if host.WaitingFor() != WaitingBattle {
            test.Fatalf("entering should start a battle, waiting for %q", host.WaitingFor())
        }
        view := host.Session.Battle.View()
        castable := false
        for _, spell := range view.Magic.Spells {
            if spell.Name == "Fire Bolt" && spell.Castable {
                castable = true
            }
        }
        if !castable {
            test.Fatalf("fire bolt should be castable: %+v", view.Magic)
        }
        if err := host.Execute(&BattleAutoCommand{}); err != nil {
            test.Fatalf("auto: %v", err)
        }
    }
}

// the siege scenario: a human army next to an AI capital with walls, a wall of fire and darkness
func TestHostSiegeScenario(test *testing.T) {
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

    for range 10 {
        host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "siege"})
        if err != nil {
            test.Fatalf("setup: %v", err)
        }
        // the scenario's army: a stack of the human's next to an enemy city
        var army *playerlib.UnitStack
        var target *Location
        for _, stack := range host.Human.Stacks {
            for _, player := range host.Model.Players {
                if player == host.Human {
                    continue
                }
                for _, city := range player.Cities {
                    // the map wraps east-west
                    width := host.Model.GetMap(city.Plane).Width()
                    dx := min(abs(city.X - stack.X()), width - abs(city.X - stack.X()))
                    if city.Plane == stack.Plane() && max(dx, abs(city.Y - stack.Y())) <= 1 {
                        army, target = stack, &Location{Plane: city.Plane, X: city.X, Y: city.Y}
                    }
                }
            }
        }
        if army == nil {
            test.Fatalf("no army of the human's next to an enemy city")
        }

        if err := host.Execute(&MoveStackCommand{From: Location{Plane: army.Plane(), X: army.X(), Y: army.Y()}, To: *target}); err != nil {
            test.Fatalf("move: %v", err)
        }
        if host.WaitingFor() != WaitingBattle {
            test.Fatalf("attacking the city should start a battle, waiting for %q", host.WaitingFor())
        }
        view := host.Session.Battle.View()
        stone, animated := 0, 0
        for _, row := range view.Tiles {
            for _, tile := range row {
                for _, wall := range tile.Walls {
                    if wall.Animated {
                        animated += 1
                    } else {
                        stone += 1
                    }
                }
            }
        }
        if stone == 0 || animated == 0 || view.Fortress == "" {
            test.Fatalf("walls %v, fire/darkness %v, fortress %q", stone, animated, view.Fortress)
        }
        if err := host.Execute(&BattleFleeCommand{}); err != nil {
            test.Fatalf("flee: %v", err)
        }
        if host.WaitingFor() == WaitingBattle {
            test.Fatalf("fleeing should end the battle")
        }
    }
}
