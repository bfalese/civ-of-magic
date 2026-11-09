package game

import (
    "slices"
    "image"
    "strings"
    "time"
    "testing"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/set"
)

// a human player with four drakes at 3,3 next to an AI spearman at 4,3
func makeSessionTestModel() (*GameModel, *playerlib.Player, *playerlib.Player) {
    names := make(map[herolib.HeroType]string)
    human := playerlib.MakePlayer(setup.WizardCustom{Name: "Merlin", Banner: data.BannerRed}, true, 10, 10, names, nil)
    enemy := playerlib.MakePlayer(setup.WizardCustom{Name: "Oberic", Banner: data.BannerBlue}, false, 10, 10, names, nil)

    model := makeSpellTestModel(10, 10)
    model.MyrrorMap = model.ArcanusMap
    model.Players = []*playerlib.Player{human, enemy}
    model.Events = make(chan GameEvent, 1000)
    human.GlobalEnchantmentsProvider = model
    enemy.GlobalEnchantmentsProvider = model
    model.CurrentPlayer = 0

    for range 4 {
        human.AddUnit(units.MakeOverworldUnitFromUnit(units.GreatDrake, 3, 3, data.PlaneArcanus, human.Wizard.Banner, human.MakeExperienceInfo(), human.MakeUnitEnchantmentProvider()))
    }
    enemy.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 4, 3, data.PlaneArcanus, enemy.Wizard.Banner, enemy.MakeExperienceInfo(), enemy.MakeUnitEnchantmentProvider()))
    human.LiftFogSquare(3, 3, 4, data.PlaneArcanus)
    for _, unit := range human.FindStack(3, 3, data.PlaneArcanus).Units() {
        human.FindStack(3, 3, data.PlaneArcanus).SetActive(unit, true)
    }

    return model, human, enemy
}

func TestSessionBattle(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    session := MakeCommandSession(model, human)
    here := Location{Plane: data.PlaneArcanus, X: 3, Y: 3}

    if err := session.Execute(&MoveStackCommand{From: here, To: Location{Plane: data.PlaneArcanus, X: 4, Y: 3}}); err != nil {
        test.Fatalf("move: %v", err)
    }
    if session.WaitingFor() != WaitingBattle || session.Battle == nil {
        test.Fatalf("the move should pause for the battle, waiting for %q", session.WaitingFor())
    }

    view := session.Battle.View()
    if view.Selected == nil || !view.YouAttack || len(view.Units) != 5 {
        test.Fatalf("battle view: %+v", view)
    }

    if encoded, err := session.Battle.EncodeView(); err != nil || len(encoded) == 0 {
        test.Errorf("battle view JSON: %v", err)
    }

    // what a battle screen draws: the field, sprites, and where the selected unit can go
    if len(view.Tiles) != view.Height || len(view.Tiles[0]) != view.Width || !strings.HasPrefix(view.Tiles[0][0].Asset, "battle-tile/") {
        test.Errorf("battle tiles: %vx%v, first %+v", view.Width, view.Height, view.Tiles[0][0])
    }
    for _, unit := range view.Units {
        if !strings.HasPrefix(unit.Asset, "battle-unit/") || unit.Banner == "" || unit.Facing == "" || len(unit.Figures) == 0 {
            test.Errorf("battle unit: %+v", unit)
        }
    }
    if len(view.Moves) == 0 {
        test.Errorf("the selected drake should be able to move")
    }
    for _, target := range view.Targets {
        if target.Kind != "melee" && target.Kind != "ranged" {
            test.Errorf("target kind %q", target.Kind)
        }
    }
    timer := time.Now()
    for range 20 {
        session.Battle.View()
    }
    test.Logf("battle view: %v each, %v moves, %v targets", time.Since(timer) / 20, len(view.Moves), len(view.Targets))

    if err := session.Execute(&SetTaxRateCommand{Numerator: 1, Denominator: 1}); err == nil {
        test.Errorf("other commands should wait until the battle is over")
    }

    // one unit waits, then the AI finishes the battle
    if err := session.Execute(&BattleWaitCommand{}); err != nil {
        test.Errorf("wait: %v", err)
    }
    if err := session.Execute(&BattleAutoCommand{}); err != nil {
        test.Fatalf("auto: %v", err)
    }

    if session.WaitingFor() != WaitingNothing {
        test.Fatalf("the battle should be over, waiting for %q", session.WaitingFor())
    }
    if len(enemy.Stacks) != 0 {
        test.Errorf("the spearman should be dead")
    }

    started, ended := false, false
    for len(model.Events) > 0 {
        if battle, ok := (<-model.Events).(*GameEventBattle); ok {
            if battle.Started {
                started = true
            } else if battle.State == "attacker-won" {
                ended = true
            }
        }
    }
    if !started || !ended {
        test.Errorf("battle started/ended events: %v %v", started, ended)
    }
}

func TestSessionEncounterDeclined(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    enemy.Stacks = nil
    model.ArcanusMap.ExtraMap[image.Pt(3, 5)][maplib.ExtraKindEncounter] = &maplib.ExtraEncounter{Type: maplib.EncounterTypeLair, Units: []units.Unit{units.HighMenSpearmen}, ExploredBy: set.MakeSet[maplib.Wizard]()}
    session := MakeCommandSession(model, human)

    if err := session.Execute(&MoveStackCommand{From: Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, To: Location{Plane: data.PlaneArcanus, X: 3, Y: 5}}); err != nil {
        test.Fatalf("move: %v", err)
    }
    if session.WaitingFor() != WaitingEncounter {
        test.Fatalf("walking into a lair should ask first, waiting for %q", session.WaitingFor())
    }
    if err := session.Execute(&BattleDoneCommand{}); err == nil {
        test.Errorf("a battle command isn't an answer to the lair question")
    }
    if err := session.Execute(&ConfirmEncounterCommand{Enter: false}); err != nil {
        test.Fatalf("decline: %v", err)
    }
    if session.WaitingFor() != WaitingNothing {
        test.Errorf("declining should finish the move, waiting for %q", session.WaitingFor())
    }
    if model.ArcanusMap.GetEncounter(3, 5) == nil {
        test.Errorf("the lair should still be there")
    }
}

func TestSessionBattleActAndFlee(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    session := MakeCommandSession(model, human)

    if err := session.Execute(&MoveStackCommand{From: Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, To: Location{Plane: data.PlaneArcanus, X: 4, Y: 3}}); err != nil {
        test.Fatalf("move: %v", err)
    }
    if session.WaitingFor() != WaitingBattle {
        test.Fatalf("expected a battle, waiting for %q", session.WaitingFor())
    }

    view := session.Battle.View()
    var selected BattleUnitView
    for _, unit := range view.Units {
        if view.Selected != nil && unit.Id == *view.Selected {
            selected = unit
        }
    }

    // a tile off the battlefield can't be acted on
    if err := session.Execute(&BattleActCommand{X: -5, Y: -5}); err == nil {
        test.Errorf("acting off the field should be refused")
    }

    // move to a free neighbouring tile
    combatModel := session.Battle.Battle.Combat
    unit := combatModel.SelectedUnit
    var target *BattleActCommand
    for dx := -1; dx <= 1 && target == nil; dx++ {
        for dy := -1; dy <= 1 && target == nil; dy++ {
            x, y := selected.X + dx, selected.Y + dy
            if (dx != 0 || dy != 0) && combatModel.TileIsEmpty(x, y) && combatModel.CanMoveTo(unit, x, y, false) {
                target = &BattleActCommand{X: x, Y: y}
            }
        }
    }
    if target == nil {
        test.Fatalf("no free tile next to the selected unit")
    }
    if err := session.Execute(target); err != nil {
        test.Fatalf("battle move: %v", err)
    }
    if unit.X != target.X || unit.Y != target.Y {
        test.Errorf("the unit should be at %v,%v, is at %v,%v", target.X, target.Y, unit.X, unit.Y)
    }

    if err := session.Execute(&BattleFleeCommand{}); err != nil {
        test.Fatalf("flee: %v", err)
    }
    if session.WaitingFor() != WaitingNothing {
        test.Fatalf("fleeing should end the battle, waiting for %q", session.WaitingFor())
    }
    // fleeing units may die (50% each), so all of them can be lost; the survivors step back
    if human.FindStack(4, 3, data.PlaneArcanus) != nil {
        test.Errorf("no stack should stay on the enemy's tile")
    }
    if len(human.Stacks) > 0 && human.FindStack(3, 3, data.PlaneArcanus) == nil {
        test.Errorf("the survivors should be back at 3,3")
    }
    fled := false
    for len(model.Events) > 0 {
        if battle, ok := (<-model.Events).(*GameEventBattle); ok && battle.State == "attacker-fled" {
            fled = true
        }
    }
    if !fled {
        test.Errorf("the battle should end with the attacker fleeing")
    }
    if len(enemy.Stacks) != 1 {
        test.Errorf("the spearman should still be there")
    }
}

func TestSessionCaptureCity(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    infos := make(buildinglib.BuildingInfos, 40)
    model.BuildingInfo = infos
    city := citylib.MakeCity("Enemyville", 4, 3, data.RaceHighMen, infos, model.ArcanusMap, model, enemy)
    city.Plane = data.PlaneArcanus
    city.Population = 3000
    city.Farmers = 3
    city.ResetCitizens()
    enemy.AddCity(city)
    session := MakeCommandSession(model, human)

    if err := session.Execute(&MoveStackCommand{From: Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, To: Location{Plane: data.PlaneArcanus, X: 4, Y: 3}}); err != nil {
        test.Fatalf("move: %v", err)
    }
    if err := session.Execute(&BattleAutoCommand{}); err != nil {
        test.Fatalf("auto: %v", err)
    }
    if session.WaitingFor() != WaitingRaze {
        test.Fatalf("winning at the city should ask raze or keep, waiting for %q", session.WaitingFor())
    }
    if err := session.Execute(&RazeCityCommand{Raze: false}); err != nil {
        test.Fatalf("keep the city: %v", err)
    }
    if session.WaitingFor() != WaitingNothing {
        test.Fatalf("the move should be finished, waiting for %q", session.WaitingFor())
    }
    if human.FindCity(4, 3, data.PlaneArcanus) == nil {
        test.Errorf("the city should belong to the human now")
    }
}

func TestSessionBattleCast(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    fireBolt := spellbook.Spell{Name: "Fire Bolt", Magic: data.ChaosMagic, CastCost: 5, Section: spellbook.SectionCombatSpell}
    human.KnownSpells.AddSpell(fireBolt)
    human.Mana = 500
    human.CastingSkillPower = 1000
    session := MakeCommandSession(model, human)

    if err := session.Execute(&MoveStackCommand{From: Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, To: Location{Plane: data.PlaneArcanus, X: 4, Y: 3}}); err != nil {
        test.Fatalf("move: %v", err)
    }
    if session.WaitingFor() != WaitingBattle {
        test.Fatalf("expected a battle, waiting for %q", session.WaitingFor())
    }

    var spearman *BattleUnitView
    for _, unit := range session.Battle.View().Units {
        if !unit.Yours {
            spearman = &unit
        }
    }
    if spearman == nil {
        test.Fatalf("no enemy unit in the battle")
    }
    manaBefore := human.Mana

    view := session.Battle.View()
    if view.Magic == nil || len(view.Magic.Spells) != 1 || view.Magic.Spells[0].Name != "Fire Bolt" || !view.Magic.Spells[0].Castable || view.Magic.Spells[0].Magic != "Chaos" {
        test.Fatalf("the wizard's spells: %+v", view.Magic)
    }

    // without a target the spell waits for one: the view lists the units it accepts
    if err := session.Execute(&BattleCastCommand{Spell: "Fire Bolt"}); err != nil {
        test.Fatalf("fire bolt without a target: %v", err)
    }
    view = session.Battle.View()
    if view.Casting == nil || view.Casting.Kind != "unit" || view.Casting.Spell != "Fire Bolt" || !slices.Equal(view.Casting.Units, []uint64{spearman.Id}) {
        test.Fatalf("casting: %+v", view.Casting)
    }
    if human.Mana != manaBefore || session.Battle.army().Casted {
        test.Errorf("a spell waiting for its target shouldn't cost anything yet")
    }
    ownId := *view.Selected
    if err := session.Execute(&BattleTargetCommand{BattleTarget{Unit: &ownId}}); err == nil {
        test.Errorf("fire bolt on your own unit should be refused")
    }
    if session.Battle.View().Casting == nil {
        test.Errorf("a refused target should leave the spell waiting")
    }
    if err := session.Execute(&BattleCancelCommand{}); err != nil {
        test.Errorf("cancel: %v", err)
    }
    if session.Battle.View().Casting != nil || human.Mana != manaBefore || session.Battle.army().Casted {
        test.Errorf("a cancelled spell should cost nothing and wait no more")
    }
    if err := session.Execute(&BattleCancelCommand{}); err == nil {
        test.Errorf("cancel with nothing waiting should be refused")
    }

    // a battle-cast with its target is still cast in one step
    id := spearman.Id
    if err := session.Execute(&BattleCastCommand{Spell: "Fire Bolt", Target: &BattleTarget{Unit: &id}}); err != nil {
        test.Fatalf("fire bolt: %v", err)
    }
    if human.Mana >= manaBefore {
        test.Errorf("the cast should cost mana")
    }

    // the bolt's damage depends on defense rolls; what's certain is that the spell was cast
    if session.WaitingFor() == WaitingBattle && !session.Battle.army().Casted {
        test.Errorf("the army should have used its spell for this turn")
    }

    if session.WaitingFor() == WaitingBattle {
        if err := session.Execute(&BattleCastCommand{Spell: "Fire Bolt", Target: &BattleTarget{Unit: &id}}); err == nil {
            test.Errorf("a second wizard spell in the same turn should be refused")
        }
        session.Execute(&BattleAutoCommand{})
    }
    if session.WaitingFor() != WaitingNothing || len(enemy.Stacks) != 0 {
        test.Errorf("the battle should be won")
    }
}

// a spell with a choice of units (Animate Dead) offers killed units, picked by id
func TestSessionBattleCastChoice(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    for range 2 {
        enemy.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 4, 3, data.PlaneArcanus, enemy.Wizard.Banner, enemy.MakeExperienceInfo(), enemy.MakeUnitEnchantmentProvider()))
    }
    human.KnownSpells.AddSpell(spellbook.Spell{Name: "Animate Dead", Magic: data.DeathMagic, CastCost: 30, Section: spellbook.SectionCombatSpell})
    human.Mana = 500
    human.CastingSkillPower = 1000
    session := MakeCommandSession(model, human)

    if err := session.Execute(&MoveStackCommand{From: Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, To: Location{Plane: data.PlaneArcanus, X: 4, Y: 3}}); err != nil {
        test.Fatalf("move: %v", err)
    }
    if session.WaitingFor() != WaitingBattle {
        test.Fatalf("expected a battle, waiting for %q", session.WaitingFor())
    }

    combatModel := session.Battle.Battle.Combat
    defenders := combatModel.DefendingArmy.GetUnits()
    if len(defenders) != 3 {
        test.Fatalf("expected 3 spearmen, got %v", len(defenders))
    }
    first, second := defenders[0], defenders[1]
    killed := []uint64{first.Id, second.Id}
    combatModel.KillUnit(first)
    combatModel.KillUnit(second)

    if err := session.Execute(&BattleCastCommand{Spell: "Animate Dead"}); err != nil {
        test.Fatalf("animate dead: %v", err)
    }
    casting := session.Battle.View().Casting
    if casting == nil || casting.Kind != "choice" || len(casting.Choices) != 2 || casting.Title == "" {
        test.Fatalf("casting: %+v", casting)
    }
    for _, choice := range casting.Choices {
        if !slices.Contains(killed, choice.Id) || !strings.HasPrefix(choice.Asset, "battle-unit/") {
            test.Errorf("choice: %+v", choice)
        }
    }

    pick := casting.Choices[0].Id
    if err := session.Execute(&BattleTargetCommand{BattleTarget{Unit: &pick}}); err != nil {
        test.Fatalf("pick a unit to animate: %v", err)
    }
    animated := combatModel.GetUnitById(pick)
    if animated == nil || animated.GetHealth() <= 0 || combatModel.GetArmyForPlayer(human) != combatModel.AttackingArmy || !slices.Contains(combatModel.AttackingArmy.GetUnits(), animated) {
        test.Errorf("the picked unit should fight for the human now")
    }
    if !session.Battle.army().Casted {
        test.Errorf("the army should have used its spell for this turn")
    }
}
