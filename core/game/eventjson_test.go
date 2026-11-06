package game

import (
    "encoding/json"
    "testing"

    "github.com/bfalese/civ-of-magic/core/artifact"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// decode an encoded event into its type and body
func decodeTestEvent(test *testing.T, model *GameModel, event GameEvent) (string, map[string]any) {
    encoded, ok, err := model.EncodeEvent(event)
    if err != nil || !ok {
        test.Fatalf("encode %T: ok=%v err=%v", event, ok, err)
    }
    var envelope struct {
        Type string `json:"type"`
        Event map[string]any `json:"event"`
    }
    if err := json.Unmarshal(encoded, &envelope); err != nil {
        test.Fatalf("not valid JSON: %s", encoded)
    }
    return envelope.Type, envelope.Event
}

func TestEncodeEvents(test *testing.T) {
    model, player, city := makeCommandTestModel()
    enemy := model.Players[1]

    eventType, body := decodeTestEvent(test, model, &GameEventNewBuilding{City: city, Building: buildinglib.BuildingBarracks, Player: player})
    if eventType != "new-building" || body["building"] != "Barracks" {
        test.Errorf("new-building: %v %v", eventType, body)
    }
    cityRef := body["city"].(map[string]any)
    if cityRef["name"] != "Testville" || cityRef["location"].(map[string]any)["x"] != float64(5) {
        test.Errorf("city ref: %v", cityRef)
    }
    if body["player"].(map[string]any)["banner"] != player.GetBanner().String() {
        test.Errorf("player ref: %v", body["player"])
    }

    eventType, body = decodeTestEvent(test, model, &GameEventDiplomacy{Player: player, Enemy: enemy})
    if eventType != "diplomacy" || body["enemy"].(map[string]any)["wizard"] != enemy.Wizard.Name {
        test.Errorf("diplomacy: %v %v", eventType, body)
    }

    player.ResearchCandidateSpells.AddSpell(spellbook.Spell{Name: "Fireball"})
    eventType, body = decodeTestEvent(test, model, &GameEventResearchSpell{Player: player})
    if eventType != "choose-research" || len(body["candidates"].([]any)) == 0 {
        test.Errorf("choose-research: %v %v", eventType, body)
    }

    treasure := Treasure{
        Point: data.PlanePoint{X: 2, Y: 3, Plane: data.PlaneArcanus},
        Treasures: []TreasureItem{
            &TreasureGold{Amount: 100},
            &TreasureMagicalItem{Artifact: &artifact.Artifact{Name: "Sword", Cost: 100}},
            &TreasureSpellbook{Magic: data.ChaosMagic, Count: 1},
        },
    }
    eventType, body = decodeTestEvent(test, model, &GameEventTreasure{Treasure: treasure, Player: player})
    items := body["items"].([]any)
    if eventType != "treasure" || len(items) != 3 {
        test.Fatalf("treasure: %v %v", eventType, body)
    }
    gold := items[0].(map[string]any)
    item := items[1].(map[string]any)
    if gold["kind"] != "gold" || gold["amount"] != float64(100) || item["artifact"].(map[string]any)["name"] != "Sword" {
        test.Errorf("treasure items: %v", items)
    }

    event := MakeDisjunctionEvent(5)
    eventType, body = decodeTestEvent(test, model, &GameEventShowRandomEvent{Event: event, Starting: true})
    if eventType != "random-event" || body["event"] != "disjunction" {
        test.Errorf("random-event: %v %v", eventType, body)
    }

    eventType, body = decodeTestEvent(test, model, &GameEventBattle{Attacker: player, Defender: enemy, Location: Location{X: 4, Y: 3}, State: "attacker-won"})
    if eventType != "battle-ended" || body["result"] != "attacker-won" {
        test.Errorf("battle-ended: %v %v", eventType, body)
    }
    eventType, body = decodeTestEvent(test, model, &GameEventChooseRaze{Player: player, City: city})
    if eventType != "choose-raze" || body["city"].(map[string]any)["name"] != "Testville" {
        test.Errorf("choose-raze: %v %v", eventType, body)
    }
    eventType, _ = decodeTestEvent(test, model, &GameEventChooseEncounter{Player: player, Encounter: "Monster Lair", Guards: 3})
    if eventType != "choose-encounter" {
        test.Errorf("choose-encounter: %v", eventType)
    }

    // events that aren't the model's are not encoded
    if _, ok, _ := model.EncodeEvent(struct{}{}); ok {
        test.Errorf("unknown events should not be encoded")
    }
}
