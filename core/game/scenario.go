package game

import (
    "fmt"
    "image"

    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/set"
)

// Scenarios set up a fresh game for tests, so they reach what they test at once instead of
// playing (and retrying) random games until it comes up. HostSettings.Scenario names one.
//
//   battle: a lair guarded by skeletons next to the human's capital (the only encounter next to
//           it), and a wizard who knows
//           Fire Bolt with mana and casting skill to spare: one move-stack starts a battle
func applyScenario(host *GameHost, name string) error {
    switch name {
        case "":
            return nil
        case "battle":
            return scenarioBattle(host)
    }
    return fmt.Errorf("unknown scenario %q", name)
}

func scenarioBattle(host *GameHost) error {
    model := host.Model
    human := host.Human
    capital := human.FindFortressCity()
    if capital == nil {
        return fmt.Errorf("battle scenario: the human has no capital")
    }
    mapUse := model.GetMap(capital.Plane)

    // the game's own encounters next to the capital would be found first (some are unguarded)
    for _, offset := range scenarioNeighbours {
        x, y := mapUse.WrapX(capital.X + offset.X), capital.Y + offset.Y
        if y >= 0 && y < mapUse.Height() && mapUse.GetEncounter(x, y) != nil {
            mapUse.RemoveEncounter(x, y)
        }
    }

    point, ok := scenarioFreeNeighbour(host, mapUse, capital.X, capital.Y)
    if !ok {
        return fmt.Errorf("battle scenario: no free land next to the capital")
    }
    mapUse.ExtraMap[point][maplib.ExtraKindEncounter] = &maplib.ExtraEncounter{
        Type: maplib.EncounterTypeLair,
        Units: []units.Unit{units.Skeleton, units.Skeleton, units.Skeleton},
        ExploredBy: set.MakeSet[maplib.Wizard](),
    }
    human.LiftFogSquare(capital.X, capital.Y, 3, capital.Plane)

    fireBolt := model.AllSpells.FindByName("Fire Bolt")
    if fireBolt.Valid() && !human.KnownSpells.Contains(fireBolt) {
        human.KnownSpells.AddSpell(fireBolt)
    }
    human.Mana = max(human.Mana, 300)
    human.CastingSkillPower = max(human.CastingSkillPower, 1000)
    return nil
}

var scenarioNeighbours = []image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}}

// a land tile next to (x, y) with nothing on it: no city, stack, encounter or other extra
func scenarioFreeNeighbour(host *GameHost, mapUse *maplib.Map, x int, y int) (image.Point, bool) {
    model := host.Model
    for _, offset := range scenarioNeighbours {
        tileX := mapUse.WrapX(x + offset.X)
        tileY := y + offset.Y
        if tileY < 0 || tileY >= mapUse.Height() {
            continue
        }
        tile := mapUse.GetTile(tileX, tileY)
        if !tile.Valid() || !tile.Tile.IsLand() || len(tile.Extras) > 0 || model.ContainsCity(tileX, tileY, mapUse.Plane) {
            continue
        }
        occupied := false
        for _, player := range model.Players {
            if player.FindStack(tileX, tileY, mapUse.Plane) != nil {
                occupied = true
            }
        }
        if occupied {
            continue
        }
        if mapUse.ExtraMap[image.Pt(tileX, tileY)] == nil {
            mapUse.ExtraMap[image.Pt(tileX, tileY)] = make(map[maplib.ExtraKind]maplib.ExtraTile)
        }
        return image.Pt(tileX, tileY), true
    }
    return image.Point{}, false
}
