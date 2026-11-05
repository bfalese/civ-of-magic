package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/lib/set"
)

func TestApplySpellOfReturn(test *testing.T) {
    human, _ := makeContactPlayers()
    human.Banished = true
    city := &citylib.City{Buildings: set.MakeSet[building.Building](), Race: data.RaceDwarf}

    model := &GameModel{}
    model.ApplySpellOfReturn(human, city)

    if human.Banished {
        test.Errorf("wizard should no longer be banished")
    }
    if !city.Buildings.Contains(building.BuildingFortress) || !city.Buildings.Contains(building.BuildingSummoningCircle) {
        test.Errorf("city should get the fortress and summoning circle")
    }
    if human.Wizard.Race != data.RaceDwarf {
        test.Errorf("wizard race should become the city's race, got %v", human.Wizard.Race)
    }
}

func TestIsGlobalEnchantmentActive(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := &GameModel{Players: []*playerlib.Player{human, enemy}}

    if model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        test.Errorf("nobody has Planar Seal yet")
    }
    enemy.GlobalEnchantments.Insert(data.EnchantmentPlanarSeal)
    if !model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        test.Errorf("enemy's Planar Seal should count")
    }
}
