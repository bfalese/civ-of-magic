package game

import (
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// The Spell of Mastery wins the game. Every wizard hears when someone starts casting it (the
// original warns them all), and when the casting finishes the caster wins: the other wizards
// are banished from the world and the game is over. The Ebitengine game shows its own screens for
// both (cast.go, mastery); command frontends get GameEventSpellOfMastery.

// someone started casting the Spell of Mastery (Started), or finished it and won
type GameEventSpellOfMastery struct {
    Player *playerlib.Player
    Started bool
}

// the player starts casting the Spell of Mastery: tell everyone
func (model *GameModel) AnnounceMasteryStarted(player *playerlib.Player) {
    select {
        case model.Events <- &GameEventSpellOfMastery{Player: player, Started: true}:
        default:
    }
}

// the player's Spell of Mastery is cast: they win
func (model *GameModel) WinByMastery(player *playerlib.Player) {
    if model.MasteryWinner != nil {
        return
    }
    model.MasteryWinner = player
    select {
        case model.Events <- &GameEventSpellOfMastery{Player: player}:
        default:
    }
    model.RefreshUI()
}
