package game

import (
    "image"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/set"
)

func TestMakeTileInfo(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    at := func(x int, y int) Location {
        return Location{Plane: data.PlaneArcanus, X: x, Y: y}
    }

    if _, err := model.MakeTileInfo(player, at(1, 1)); err == nil {
        test.Errorf("an unexplored tile should be refused")
    }
    player.LiftFogSquare(4, 4, 4, data.PlaneArcanus)

    // next to the city at 5,5: too close for another one
    info, err := model.MakeTileInfo(player, at(6, 6))
    if err != nil {
        test.Fatalf("tile info: %v", err)
    }
    if info.Site.CanBuild || info.Site.Reason == "" {
        test.Errorf("a tile next to a city shouldn't be a city site: %+v", info.Site)
    }

    // the city's own tile: its resources, no new city
    info, _ = model.MakeTileInfo(player, at(5, 5))
    if info.City == "" || info.CityOwner == nil || info.Site.CanBuild || info.Site.MaximumPopulation == 0 {
        test.Errorf("the city tile should name the city and its resources: %+v", info)
    }

    // far enough from the city, with a bonus
    model.ArcanusMap.SetBonus(1, 1, data.BonusGoldOre)
    info, _ = model.MakeTileInfo(player, at(1, 1))
    if !info.Site.CanBuild || info.Site.MaximumPopulation == 0 {
        test.Errorf("open land 4 tiles away should be a city site: %+v", info.Site)
    }
    if info.Bonus == nil || info.Bonus.Name != data.BonusGoldOre.String() || info.Bonus.Gold != 3 {
        test.Errorf("the gold ore should show: %+v", info.Bonus)
    }

    // a lair: its guards stay unknown until explored, and no city can go there
    model.ArcanusMap.ExtraMap[image.Pt(1, 2)][maplib.ExtraKindEncounter] = &maplib.ExtraEncounter{
        Type: maplib.EncounterTypeLair,
        Units: []units.Unit{units.Skeleton},
        ExploredBy: set.MakeSet[maplib.Wizard](),
    }
    info, _ = model.MakeTileInfo(player, at(1, 2))
    if info.Encounter == nil || info.Encounter.Guards != "Unexplored" || info.Site.CanBuild {
        test.Errorf("an unexplored lair: %+v %+v", info.Encounter, info.Site)
    }
    model.ArcanusMap.GetEncounter(1, 2).ExploredBy.Insert(player)
    info, _ = model.MakeTileInfo(player, at(1, 2))
    if info.Encounter.Guards != units.Skeleton.Name {
        test.Errorf("an explored lair names its guards, got %q", info.Encounter.Guards)
    }

    model.ArcanusMap.SetCorruption(2, 1)
    info, _ = model.MakeTileInfo(player, at(2, 1))
    if !info.Corrupted || info.Site.CanBuild {
        test.Errorf("corrupted land isn't a city site: %+v", info)
    }
}

// the mirror: your books, retorts and casting skill, a rival's books and retorts
func TestMirrorView(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.Wizard.Books = []data.WizardBook{{Magic: data.LifeMagic, Count: 6}, {Magic: data.NatureMagic, Count: 3}}
    player.Wizard.Retorts = []data.Retort{data.RetortAlchemy}
    other := model.Players[1]
    other.Wizard.Books = []data.WizardBook{{Magic: data.ChaosMagic, Count: 9}}
    player.AwarePlayer(other)

    view := model.MakePlayerView(player)
    mirror := view.You.Mirror
    if mirror.Books["Life"] != 6 || mirror.Books["Nature"] != 3 || len(mirror.Retorts) != 1 || mirror.Retorts[0] != data.RetortAlchemy.String() {
        test.Errorf("your mirror: %+v", mirror)
    }
    if mirror.CastingSkill != view.You.CastingSkill {
        test.Errorf("your mirror should show your casting skill")
    }
    if len(view.Diplomacy) != 1 || view.Diplomacy[0].Mirror.Books["Chaos"] != 9 || view.Diplomacy[0].Mirror.CastingSkill != 0 {
        test.Errorf("the rival's mirror: %+v", view.Diplomacy)
    }
}
