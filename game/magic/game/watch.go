package game

import (
    gamecore "github.com/bfalese/civ-of-magic/core/game"
)

// In watch mode the "human" player is only an observer (Admin, Banished, Skip,
// no cities or units) and nobody is there to click through dialogs. Events that
// would open a screen for the human and wait for input are dropped, so the AI
// game keeps running hands-off.
func (game *Game) skipInWatchMode(event GameEvent) bool {
    if !game.WatchMode {
        return false
    }

    switch event.(type) {
        // first contact with a wizard
        case *gamecore.GameEventDiplomacy: return true
        // research screens for the observer
        case *gamecore.GameEventLearnedSpell: return true
        case *gamecore.GameEventResearchSpell: return true
        // messages addressed to the human
        case *gamecore.GameEventNotice: return true
        case *gamecore.GameEventScroll: return true
        case *gamecore.GameEventNewBuilding: return true
        case *gamecore.GameEventHeroLevelUp: return true
        case *gamecore.GameEventVault: return true
        case *GameEventCityName: return true
        case *GameEventNewOutpost: return true
        case *gamecore.GameEventShowBanish: return true
        // offers to the human; AI players handle these without a screen
        case *gamecore.GameEventHireHero:
            return event.(*gamecore.GameEventHireHero).Player.IsHuman()
        case *gamecore.GameEventHireMercenaries:
            return event.(*gamecore.GameEventHireMercenaries).Player.IsHuman()
        case *gamecore.GameEventMerchant:
            return event.(*gamecore.GameEventMerchant).Player.IsHuman()
    }

    return false
}
