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

// a wizard's answer to a spell trade: they'd give Receive for Give (answer with answer-trade)
type GameEventTradeOffer struct {
    Player *playerlib.Player
    Wizard *playerlib.Player
    Give spellbook.Spell
    Receive spellbook.Spell
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

// treasure found in a cleared encounter (see CreateTreasure)
type GameEventTreasure struct {
    Treasure Treasure
    Player *playerlib.Player
}

// a finished spell waits for its target (frontends using commands answer with cast-spell-target)
type GameEventChooseSpellTarget struct {
    Player *playerlib.Player
    Spell spellbook.Spell
    LocationType LocationType
}

// a battle involving the session's player started (Started) or ended (State)
type GameEventBattle struct {
    Attacker *playerlib.Player
    Defender *playerlib.Player
    Location Location
    Started bool
    State string
}

// a stack of the session's player reached a lair / node / tower: enter? (confirm-encounter)
type GameEventChooseEncounter struct {
    Player *playerlib.Player
    Encounter string
    Guards int
}

// the session's player won at a city: raze it or keep it? (raze-city)
type GameEventChooseRaze struct {
    Player *playerlib.Player
    City *citylib.City
}
