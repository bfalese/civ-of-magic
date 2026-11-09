package game

import (
    "fmt"
    "image"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
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
//   siege:  an AI wizard's capital with city walls, a wall of fire and a wall of darkness, and a
//           human army next to it: one move-stack starts a battle at the walls
func applyScenario(host *GameHost, name string) error {
    switch name {
        case "":
            return nil
        case "battle":
            return scenarioBattle(host)
        case "siege":
            return scenarioSiege(host)
    }
    return fmt.Errorf("unknown scenario %q", name)
}

// siege: an AI wizard's capital (with its fortress) gets city walls, a wall of fire and a wall
//        of darkness, and a human army of swordsmen stands next to it: one move-stack starts a
//        battle at the walls
func scenarioSiege(host *GameHost) error {
    model := host.Model
    human := host.Human
    var capital *citylib.City
    var enemy *playerlib.Player
    for _, player := range model.Players {
        if player != human && player.AIBehavior != nil && !player.IsNeutral() {
            if city := player.FindFortressCity(); city != nil {
                capital, enemy = city, player
                break
            }
        }
    }
    if capital == nil {
        return fmt.Errorf("siege scenario: no AI wizard with a capital")
    }

    capital.Buildings.Insert(buildinglib.BuildingCityWalls)
    capital.AddEnchantment(data.CityEnchantmentWallOfFire, enemy.GetBanner())
    capital.AddEnchantment(data.CityEnchantmentWallOfDarkness, enemy.GetBanner())
    // someone to fight at the walls
    enemy.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, capital.X, capital.Y, capital.Plane, enemy.Wizard.Banner, enemy.MakeExperienceInfo(), enemy.MakeUnitEnchantmentProvider()))

    mapUse := model.GetMap(capital.Plane)
    point, ok := scenarioFreeNeighbour(host, mapUse, capital.X, capital.Y)
    if !ok {
        return fmt.Errorf("siege scenario: no free land next to the enemy capital")
    }
    for range 4 {
        human.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSwordsmen, point.X, point.Y, capital.Plane, human.Wizard.Banner, human.MakeExperienceInfo(), human.MakeUnitEnchantmentProvider()))
    }
    stack := human.FindStack(point.X, point.Y, capital.Plane)
    for _, unit := range stack.Units() {
        stack.SetActive(unit, true)
    }
    human.LiftFogSquare(point.X, point.Y, 3, capital.Plane)
    return nil
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
