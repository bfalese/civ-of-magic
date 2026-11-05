package game

import (
    "github.com/bfalese/civ-of-magic/core/artifact"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

type GameEvent interface {
}

type GameEventNotice struct {
    Message string
}

type GameEventRefreshUI struct {
}

type GameEventScroll struct {
    Title string
    Text string
    Old bool // a replayed event, don't add it again
}

// https://masterofmagic.fandom.com/wiki/Event
type GameEventShowRandomEvent struct {
    Event *RandomEvent
    // true if the event is just starting, or false if it is ending
    Starting bool
}

type GameEventVault struct {
    CreatedArtifact *artifact.Artifact
    Player *playerlib.Player
}

type GameEventDiplomacy struct {
    Player *playerlib.Player
    Enemy *playerlib.Player
}
