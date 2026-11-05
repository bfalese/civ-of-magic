package game

import (
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// make two wizards mutually aware. returns a diplomacy event when the human
// is meeting the other wizard for the first time. raiders and defeated
// wizards are ignored.
func (model *GameModel) MakeWizardContact(a *playerlib.Player, b *playerlib.Player) *GameEventDiplomacy {
    if a == nil || b == nil || a == b {
        return nil
    }
    if a.IsNeutral() || b.IsNeutral() || a.Defeated || b.Defeated {
        return nil
    }

    knewA := a.IsAwareOf(b)
    knewB := b.IsAwareOf(a)
    if knewA && knewB {
        return nil
    }

    a.AwarePlayer(b)
    b.AwarePlayer(a)

    if a.IsHuman() && !knewA {
        return &GameEventDiplomacy{Player: a, Enemy: b}
    }
    if b.IsHuman() && !knewB {
        return &GameEventDiplomacy{Player: b, Enemy: a}
    }

    return nil
}

func (model *GameModel) DiscoverVisibleWizards() []*GameEventDiplomacy {
    var meetings []*GameEventDiplomacy

    if model == nil {
        return meetings
    }

    for _, observer := range model.Players {
        if observer.IsNeutral() || observer.Defeated {
            continue
        }

        for _, other := range model.Players {
            if other == observer || other.IsNeutral() || other.Defeated {
                continue
            }
            if observer.IsAwareOf(other) {
                continue
            }
            if observer.CanSeePlayer(other) {
                if event := model.MakeWizardContact(observer, other); event != nil {
                    meetings = append(meetings, event)
                }
            }
        }
    }

    return meetings
}
