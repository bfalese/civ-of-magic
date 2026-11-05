package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// a stack of count units of the given kind for the player at 3,3 on Arcanus
func makeTestStack(player *playerlib.Player, kind units.Unit, count int) *playerlib.UnitStack {
    for i := 0; i < count; i++ {
        unit := units.MakeOverworldUnitFromUnit(kind, 3, 3, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
        player.AddUnit(unit)
    }
    return player.FindStack(3, 3, data.PlaneArcanus)
}

func TestApplyStasis(test *testing.T) {
    _, enemy := makeContactPlayers()
    stack := makeTestStack(enemy, units.HighElfSpearmen, 2)
    if stack == nil || len(stack.Units()) != 2 {
        test.Fatalf("test stack not created")
    }

    model := &GameModel{}
    model.ApplyStasis(stack)
    for _, unit := range stack.Units() {
        if unit.GetBusy() != units.BusyStatusStasis {
            test.Errorf("unit not in stasis: %v", unit.GetBusy())
        }
    }
}

func TestApplyFireStormDamagesOrKills(test *testing.T) {
    _, enemy := makeContactPlayers()
    stack := makeTestStack(enemy, units.HighElfSpearmen, 4)

    before := 0
    for _, unit := range stack.Units() {
        before += unit.GetHealth()
    }

    model := &GameModel{}
    model.ApplyFireStorm(stack, enemy)

    after := 0
    remaining := enemy.FindStack(3, 3, data.PlaneArcanus)
    if remaining != nil {
        for _, unit := range remaining.Units() {
            if unit.GetHealth() <= 0 {
                test.Errorf("dead units must be removed")
            }
            after += unit.GetHealth()
        }
    }
    if after >= before {
        test.Errorf("fire storm should hurt the stack: health %v -> %v", before, after)
    }
}
