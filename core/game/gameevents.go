package game

import (
    "github.com/bfalese/civ-of-magic/core/spellbook"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    "github.com/bfalese/civ-of-magic/core/artifact"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
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

type GameEventHireMercenaries struct {
    Units []*units.OverworldUnit
    Player *playerlib.Player
    Cost int
}

type GameEventMerchant struct {
    Artifact *artifact.Artifact
    Player *playerlib.Player
    Cost int
}

type GameEventHireHero struct {
    Hero *herolib.Hero
    Player *playerlib.Player
    Cost int
}

type GameEventHeroLevelUp struct {
    Hero *herolib.Hero
}

type GameEventLearnedSpell struct {
    Player *playerlib.Player
    Spell spellbook.Spell
}

type GameEventResearchSpell struct {
    Player *playerlib.Player
}

type GameEventNewBuilding struct {
    City *citylib.City
    Building buildinglib.Building
    Player *playerlib.Player
}

// the attacker captured or razed the city holding the defender's fortress
type GameEventShowBanish struct {
    Attacker *playerlib.Player
    Defender *playerlib.Player
}

type GameEventNewOutpost struct {
    City *citylib.City
    Stack *playerlib.UnitStack
    Player *playerlib.Player
}
