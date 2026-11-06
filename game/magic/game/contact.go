package game

import (
    "github.com/bfalese/civ-of-magic/lib/coroutine"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    gamecore "github.com/bfalese/civ-of-magic/core/game"
)

func (game *Game) playFirstMeeting(yield coroutine.YieldFunc, event *gamecore.GameEventDiplomacy) {
    // no one to greet in watch mode (see skipInWatchMode)
    if event == nil || game.WatchMode {
        return
    }

    if yield != nil {
        game.doDiplomacy(yield, event.Player, event.Enemy)
        return
    }

    if game.Events != nil {
        select {
            case game.Events <- event:
            default:
        }
    }
}

func (game *Game) meetWizards(yield coroutine.YieldFunc, a *playerlib.Player, b *playerlib.Player) {
    if game.Model == nil {
        return
    }
    game.playFirstMeeting(yield, game.Model.MakeWizardContact(a, b))
}

func (game *Game) discoverWizards(yield coroutine.YieldFunc) bool {
    if game.Model == nil {
        return false
    }

    meetings := game.Model.DiscoverVisibleWizards()
    for _, meeting := range meetings {
        game.playFirstMeeting(yield, meeting)
    }
    return len(meetings) > 0
}
