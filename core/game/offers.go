package game

import (
    "github.com/bfalese/civ-of-magic/core/units"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/artifact"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/data"
)

// Accepting the offers a player gets (hero for hire, mercenaries, merchant). The frontend shows
// the offer and calls these when the player accepts; AI players go through their AIBehavior.

// hire the hero at the fortress or at point. returns false if there was no room (nothing is paid)
func (model *GameModel) HireHero(player *playerlib.Player, hero *herolib.Hero, cost int, atFortress bool, point data.PlanePoint) bool {
    added := false
    if atFortress {
        added = player.AddHeroToFortress(hero)
    } else {
        added = player.AddHero(hero, point.X, point.Y, point.Plane)
    }

    if added {
        player.Gold -= cost
        hero.SetStatus(herolib.StatusEmployed)

        model.ResolveStackAt(hero.GetX(), hero.GetY(), hero.GetPlane())

        if player.SelectedStack == nil {
            player.SelectedStack = player.FindStack(hero.GetX(), hero.GetY(), hero.GetPlane())
        }

        model.RefreshUI()
    }

    return added
}

// the player turned the hero down
func (model *GameModel) DeclineHero(hero *herolib.Hero) {
    hero.GainLevel(units.ExperienceChampionHero)
}

func (model *GameModel) HireMercenaries(player *playerlib.Player, mercenaries []*units.OverworldUnit, cost int) {
    for _, unit := range mercenaries {
        player.AddUnit(unit)
        model.ResolveStackAt(unit.GetX(), unit.GetY(), unit.GetPlane())
    }
    player.Gold -= cost
    model.RefreshUI()
}

// buy the merchant's item; the frontend then puts it in the vault
func (model *GameModel) BuyMerchantItem(player *playerlib.Player, item *artifact.Artifact, cost int) {
    model.ArtifactPool.Award(item)
    player.Gold -= cost
}
