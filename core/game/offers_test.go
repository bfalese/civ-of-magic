package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/units"
)

func TestHireMercenaries(test *testing.T) {
    model, player, _ := makeBattleTestModel()
    player.Gold = 500

    mercenaries := []*units.OverworldUnit{
        units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 7, 7, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()),
        units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 7, 7, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()),
    }
    model.HireMercenaries(player, mercenaries, 200)

    if player.Gold != 300 {
        test.Errorf("mercenaries should cost 200 gold, gold is %v", player.Gold)
    }
    stack := player.FindStack(7, 7, data.PlaneArcanus)
    if stack == nil || len(stack.Units()) != 2 {
        test.Errorf("both mercenaries should be in one stack at 7,7")
    }
}

func TestHireHero(test *testing.T) {
    model, player, _ := makeBattleTestModel()
    player.Gold = 500

    hero := herolib.MakeHero(units.MakeOverworldUnitFromUnit(units.HeroRakir, 0, 0, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()), herolib.HeroRakir, "Rakir")
    if !model.HireHero(player, hero, 100, false, data.PlanePoint{X: 8, Y: 8, Plane: data.PlaneArcanus}) {
        test.Fatalf("hero should be hired")
    }
    if player.Gold != 400 || hero.Status != herolib.StatusEmployed {
        test.Errorf("gold=%v status=%v", player.Gold, hero.Status)
    }
    if player.FindStack(8, 8, data.PlaneArcanus) == nil {
        test.Errorf("hero should stand at 8,8")
    }
}

func TestArtifactManaValue(test *testing.T) {
    _, player, _ := makeBattleTestModel()
    item := &artifact.Artifact{Name: "Sword", Cost: 300}

    if player.ArtifactManaValue(item) != 150 {
        test.Errorf("destroying an item gives half its cost, got %v", player.ArtifactManaValue(item))
    }
    player.Wizard.EnableRetort(data.RetortArtificer)
    if player.ArtifactManaValue(item) != 300 {
        test.Errorf("artificers get the full cost, got %v", player.ArtifactManaValue(item))
    }
}

// an offer answered outside AnswerOffer (the desktop's screens, an AI's decision) is closed; a
// newer offer to the same player stays open
func TestCloseOffer(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    first := &GameEventMerchant{Player: player, Cost: 10}
    model.SendOffer(first)
    model.CloseOffer(first)
    if err := model.AnswerOffer(player, false); err == nil {
        test.Errorf("a closed offer can't be answered")
    }

    second := &GameEventMerchant{Player: player, Cost: 20}
    model.SendOffer(first)
    model.SendOffer(second)
    model.CloseOffer(first)
    if open, ok := model.PendingOffers[player]; !ok || open != second {
        test.Errorf("closing a replaced offer shouldn't close the newer one")
    }
}
