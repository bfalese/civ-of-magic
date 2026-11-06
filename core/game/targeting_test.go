package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

func TestCheckSpellTarget(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := makeSpellTestModel(10, 10)
    model.Players = []*playerlib.Player{human, enemy}
    spell := spellbook.Spell{Name: "Test"}

    check := func(locationType LocationType, x int, y int) bool {
        _, ok, _ := model.CheckSpellTarget(human, spell, locationType, model.ComputeCityStackInfo(), data.PlaneArcanus, x, y)
        return ok
    }

    if !check(LocationTypeAny, 4, 4) {
        test.Errorf("any tile should be valid for LocationTypeAny")
    }
    if check(LocationTypeFriendlyUnit, 3, 3) || check(LocationTypeFriendlyCity, 3, 3) || check(LocationTypeEnemyCity, 3, 3) {
        test.Errorf("empty tile: unit/city targets must be refused")
    }

    unit := units.MakeOverworldUnitFromUnit(units.HighElfSpearmen, 3, 3, data.PlaneArcanus, human.Wizard.Banner, human.MakeExperienceInfo(), human.MakeUnitEnchantmentProvider())
    human.AddUnit(unit)
    if !check(LocationTypeFriendlyUnit, 3, 3) {
        test.Errorf("own stack should be a valid friendly-unit target")
    }

    // tiles the player hasn't explored are never valid map targets
    if check(LocationTypeLand, 8, 8) {
        test.Errorf("unexplored land should not be a valid target")
    }
}
