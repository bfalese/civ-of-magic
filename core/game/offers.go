package game

import (
    "errors"

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

// send an offer event (GameEventHireHero, GameEventHireMercenaries or GameEventMerchant) and keep
// it as the player's open offer, for frontends that answer with the answer-offer command
func (model *GameModel) SendOffer(offer GameEvent) {
    var player *playerlib.Player
    switch offer.(type) {
        case *GameEventHireHero: player = offer.(*GameEventHireHero).Player
        case *GameEventHireMercenaries: player = offer.(*GameEventHireMercenaries).Player
        case *GameEventMerchant: player = offer.(*GameEventMerchant).Player
        default: return
    }

    if model.PendingOffers == nil {
        model.PendingOffers = make(map[*playerlib.Player]GameEvent)
    }
    model.PendingOffers[player] = offer

    select {
        case model.Events <- offer:
        default:
    }
}

// the offer was answered elsewhere (the desktop's hire and merchant screens, an AI player's own
// decision): it isn't open any more. Does nothing if another offer has replaced it.
func (model *GameModel) CloseOffer(offer GameEvent) {
    for player, open := range model.PendingOffers {
        if open == offer {
            delete(model.PendingOffers, player)
        }
    }
}

// accept or decline the player's open offer (see SendOffer)
func (model *GameModel) AnswerOffer(player *playerlib.Player, accept bool) error {
    offer, ok := model.PendingOffers[player]
    if !ok {
        return errors.New("no open offer")
    }

    if !accept {
        delete(model.PendingOffers, player)
        if hire, ok := offer.(*GameEventHireHero); ok {
            model.DeclineHero(hire.Hero)
        }
        return nil
    }

    switch offer.(type) {
        case *GameEventHireHero:
            hire := offer.(*GameEventHireHero)
            if hire.Cost > player.Gold {
                return errors.New("not enough gold")
            }
            if !model.HireHero(player, hire.Hero, hire.Cost, true, data.PlanePoint{}) {
                return errors.New("no room for another hero")
            }
        case *GameEventHireMercenaries:
            hire := offer.(*GameEventHireMercenaries)
            if hire.Cost > player.Gold {
                return errors.New("not enough gold")
            }
            model.HireMercenaries(player, hire.Units, hire.Cost)
        case *GameEventMerchant:
            merchant := offer.(*GameEventMerchant)
            if merchant.Cost > player.Gold {
                return errors.New("not enough gold")
            }
            model.BuyMerchantItem(player, merchant.Artifact, merchant.Cost)
            model.SendVaultItem(&GameEventVault{CreatedArtifact: merchant.Artifact, Player: player})
    }

    delete(model.PendingOffers, player)
    return nil
}
