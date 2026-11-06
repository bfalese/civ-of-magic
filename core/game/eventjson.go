package game

import (
    "encoding/json"

    "github.com/bfalese/civ-of-magic/core/artifact"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// JSON form of the events the model emits, for frontends that don't share memory with the model
// (the wasm bridge, a network client). The counterpart of commands (command.go).
//
// An event is written as {"type": "new-building", "event": {...}}. Players are referred to by
// banner (unique per player), cities and units by location, as in commands. Only data is sent:
// the frontend decides how to show it.

// a player, by banner
type PlayerRef struct {
    Banner string `json:"banner"`
    Wizard string `json:"wizard"`
    Human bool `json:"human,omitempty"`
}

type CityRef struct {
    Name string `json:"name"`
    Location Location `json:"location"`
    Race string `json:"race"`
}

type UnitRef struct {
    Name string `json:"name"`
    Location Location `json:"location"`
}

type HeroRef struct {
    Name string `json:"name"`
    Title string `json:"title"`
    Level int `json:"level"`
    Location Location `json:"location"`
}

type TreasureItemJSON struct {
    // gold, mana, item, prisoner, spell, spellbook, retort
    Kind string `json:"kind"`
    // the item as text, e.g. "100 gold coins"
    Text string `json:"text"`
    Amount int `json:"amount,omitempty"`
    Spell string `json:"spell,omitempty"`
    Magic string `json:"magic,omitempty"`
    Retort string `json:"retort,omitempty"`
    Artifact *artifact.SerializedArtifact `json:"artifact,omitempty"`
    Hero *HeroRef `json:"hero,omitempty"`
}

type eventEnvelope struct {
    Type string `json:"type"`
    Event any `json:"event"`
}

func MakePlayerRef(player *playerlib.Player) *PlayerRef {
    if player == nil {
        return nil
    }
    return &PlayerRef{
        Banner: player.GetBanner().String(),
        Wizard: player.Wizard.Name,
        Human: player.IsHuman(),
    }
}

func MakeCityRef(city *citylib.City) *CityRef {
    if city == nil {
        return nil
    }
    return &CityRef{
        Name: city.Name,
        Location: Location{Plane: city.Plane, X: city.X, Y: city.Y},
        Race: city.Race.String(),
    }
}

func MakeUnitRef(unit units.StackUnit) UnitRef {
    return UnitRef{
        Name: unit.GetName(),
        Location: Location{Plane: unit.GetPlane(), X: unit.GetX(), Y: unit.GetY()},
    }
}

func MakeHeroRef(hero *herolib.Hero) *HeroRef {
    if hero == nil {
        return nil
    }
    return &HeroRef{
        Name: hero.GetName(),
        Title: hero.GetTitle(),
        Level: int(hero.GetHeroExperienceLevel()),
        Location: Location{Plane: hero.GetPlane(), X: hero.GetX(), Y: hero.GetY()},
    }
}

func serializeArtifactRef(item *artifact.Artifact) *artifact.SerializedArtifact {
    if item == nil {
        return nil
    }
    serialized := artifact.SerializeArtifact(item)
    return &serialized
}

func makeTreasureItemJSON(item TreasureItem) TreasureItemJSON {
    out := TreasureItemJSON{Text: item.String()}
    switch item.(type) {
        case *TreasureGold:
            out.Kind = "gold"
            out.Amount = item.(*TreasureGold).Amount
        case *TreasureMana:
            out.Kind = "mana"
            out.Amount = item.(*TreasureMana).Amount
        case *TreasureMagicalItem:
            out.Kind = "item"
            out.Artifact = serializeArtifactRef(item.(*TreasureMagicalItem).Artifact)
        case *TreasurePrisonerHero:
            out.Kind = "prisoner"
            out.Hero = MakeHeroRef(item.(*TreasurePrisonerHero).Hero)
        case *TreasureSpell:
            out.Kind = "spell"
            out.Spell = item.(*TreasureSpell).Spell.Name
        case *TreasureSpellbook:
            spellbook := item.(*TreasureSpellbook)
            out.Kind = "spellbook"
            out.Magic = spellbook.Magic.String()
            out.Amount = spellbook.Count
        case *TreasureRetort:
            out.Kind = "retort"
            out.Retort = item.(*TreasureRetort).Retort.String()
    }
    return out
}

// the JSON type name and body of an event; ok is false for events that aren't part of the model's
// output (the Ebitengine UI puts its own events on the same channel)
func (model *GameModel) eventJSON(event GameEvent) (string, any, bool) {
    switch event.(type) {
        case *GameEventNotice:
            notice := event.(*GameEventNotice)
            return "notice", struct {
                Message string `json:"message"`
            }{notice.Message}, true

        case *GameEventRefreshUI:
            return "refresh", struct{}{}, true

        case *GameEventScroll:
            scroll := event.(*GameEventScroll)
            return "scroll", struct {
                Title string `json:"title"`
                Text string `json:"text"`
                Old bool `json:"old,omitempty"`
            }{scroll.Title, scroll.Text, scroll.Old}, true

        case *GameEventShowRandomEvent:
            show := event.(*GameEventShowRandomEvent)
            randomEvent := show.Event
            message := randomEvent.Message
            if !show.Starting {
                message = randomEvent.MessageStop
            }
            return "random-event", struct {
                Event string `json:"event"`
                Starting bool `json:"starting"`
                Message string `json:"message"`
                Good bool `json:"good"`
                City *CityRef `json:"city,omitempty"`
                Player *PlayerRef `json:"player,omitempty"`
            }{randomEvent.Type.Name(), show.Starting, message, randomEvent.Type.IsGood(), MakeCityRef(randomEvent.TargetCity), MakePlayerRef(randomEvent.TargetPlayer)}, true

        case *GameEventVault:
            vault := event.(*GameEventVault)
            return "vault", struct {
                Player *PlayerRef `json:"player"`
                Artifact *artifact.SerializedArtifact `json:"artifact"`
            }{MakePlayerRef(vault.Player), serializeArtifactRef(vault.CreatedArtifact)}, true

        case *GameEventDiplomacy:
            diplomacy := event.(*GameEventDiplomacy)
            return "diplomacy", struct {
                Player *PlayerRef `json:"player"`
                Enemy *PlayerRef `json:"enemy"`
            }{MakePlayerRef(diplomacy.Player), MakePlayerRef(diplomacy.Enemy)}, true

        case *GameEventHireMercenaries:
            hire := event.(*GameEventHireMercenaries)
            mercenaries := []UnitRef{}
            for _, unit := range hire.Units {
                mercenaries = append(mercenaries, MakeUnitRef(unit))
            }
            return "hire-mercenaries", struct {
                Player *PlayerRef `json:"player"`
                Units []UnitRef `json:"units"`
                Cost int `json:"cost"`
            }{MakePlayerRef(hire.Player), mercenaries, hire.Cost}, true

        case *GameEventMerchant:
            merchant := event.(*GameEventMerchant)
            return "merchant", struct {
                Player *PlayerRef `json:"player"`
                Artifact *artifact.SerializedArtifact `json:"artifact"`
                Cost int `json:"cost"`
            }{MakePlayerRef(merchant.Player), serializeArtifactRef(merchant.Artifact), merchant.Cost}, true

        case *GameEventHireHero:
            hire := event.(*GameEventHireHero)
            return "hire-hero", struct {
                Player *PlayerRef `json:"player"`
                Hero *HeroRef `json:"hero"`
                Cost int `json:"cost"`
            }{MakePlayerRef(hire.Player), MakeHeroRef(hire.Hero), hire.Cost}, true

        case *GameEventHeroLevelUp:
            levelUp := event.(*GameEventHeroLevelUp)
            return "hero-level-up", struct {
                Hero *HeroRef `json:"hero"`
            }{MakeHeroRef(levelUp.Hero)}, true

        case *GameEventLearnedSpell:
            learned := event.(*GameEventLearnedSpell)
            return "learned-spell", struct {
                Player *PlayerRef `json:"player"`
                Spell string `json:"spell"`
                Magic string `json:"magic"`
            }{MakePlayerRef(learned.Player), learned.Spell.Name, learned.Spell.Magic.String()}, true

        case *GameEventResearchSpell:
            research := event.(*GameEventResearchSpell)
            candidates := []string{}
            for _, spell := range research.Player.ResearchCandidateSpells.Spells {
                candidates = append(candidates, spell.Name)
            }
            return "choose-research", struct {
                Player *PlayerRef `json:"player"`
                Candidates []string `json:"candidates"`
            }{MakePlayerRef(research.Player), candidates}, true

        case *GameEventNewBuilding:
            newBuilding := event.(*GameEventNewBuilding)
            return "new-building", struct {
                Player *PlayerRef `json:"player"`
                City *CityRef `json:"city"`
                Building string `json:"building"`
            }{MakePlayerRef(newBuilding.Player), MakeCityRef(newBuilding.City), model.BuildingInfo.Name(newBuilding.Building)}, true

        case *GameEventShowBanish:
            banish := event.(*GameEventShowBanish)
            return "banish", struct {
                Attacker *PlayerRef `json:"attacker"`
                Defender *PlayerRef `json:"defender"`
            }{MakePlayerRef(banish.Attacker), MakePlayerRef(banish.Defender)}, true

        case *GameEventNewOutpost:
            outpost := event.(*GameEventNewOutpost)
            return "new-outpost", struct {
                Player *PlayerRef `json:"player"`
                City *CityRef `json:"city"`
            }{MakePlayerRef(outpost.Player), MakeCityRef(outpost.City)}, true

        case *GameEventTreasure:
            treasure := event.(*GameEventTreasure)
            items := []TreasureItemJSON{}
            for _, item := range treasure.Treasure.Treasures {
                items = append(items, makeTreasureItemJSON(item))
            }
            return "treasure", struct {
                Player *PlayerRef `json:"player"`
                Location Location `json:"location"`
                Items []TreasureItemJSON `json:"items"`
            }{MakePlayerRef(treasure.Player), Location{Plane: treasure.Treasure.Point.Plane, X: treasure.Treasure.Point.X, Y: treasure.Treasure.Point.Y}, items}, true

        case *GameEventBattle:
            battle := event.(*GameEventBattle)
            eventType := "battle-ended"
            if battle.Started {
                eventType = "battle-started"
            }
            return eventType, struct {
                Attacker *PlayerRef `json:"attacker"`
                Defender *PlayerRef `json:"defender"`
                Location Location `json:"location"`
                Result string `json:"result,omitempty"`
            }{MakePlayerRef(battle.Attacker), MakePlayerRef(battle.Defender), battle.Location, battle.State}, true

        case *GameEventChooseEncounter:
            choose := event.(*GameEventChooseEncounter)
            return "choose-encounter", struct {
                Player *PlayerRef `json:"player"`
                Encounter string `json:"encounter"`
                Guards int `json:"guards"`
            }{MakePlayerRef(choose.Player), choose.Encounter, choose.Guards}, true

        case *GameEventChooseRaze:
            choose := event.(*GameEventChooseRaze)
            return "choose-raze", struct {
                Player *PlayerRef `json:"player"`
                City *CityRef `json:"city"`
            }{MakePlayerRef(choose.Player), MakeCityRef(choose.City)}, true

        case *GameEventChooseSpellTarget:
            choose := event.(*GameEventChooseSpellTarget)
            return "choose-spell-target", struct {
                Player *PlayerRef `json:"player"`
                Spell string `json:"spell"`
                Target string `json:"target"`
            }{MakePlayerRef(choose.Player), choose.Spell.Name, choose.LocationType.Name()}, true
    }

    return "", nil, false
}

// the event as JSON ({"type": ..., "event": {...}}); ok is false if the event isn't one of the
// model's (e.g. a UI-only event on the shared channel)
func (model *GameModel) EncodeEvent(event GameEvent) ([]byte, bool, error) {
    eventType, body, ok := model.eventJSON(event)
    if !ok {
        return nil, false, nil
    }

    encoded, err := json.Marshal(eventEnvelope{Type: eventType, Event: body})
    return encoded, true, err
}

