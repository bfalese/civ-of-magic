package game

import (
    "time"
    "log"

    "github.com/bfalese/civ-of-magic/core/ai"
    "github.com/bfalese/civ-of-magic/core/maplib"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/serialize"
    settingslib "github.com/bfalese/civ-of-magic/game/magic/settings"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/terrain"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    gamecore "github.com/bfalese/civ-of-magic/core/game"
)

const SerializeVersion = 1

type SerializedTargetCity struct {
    X int `json:"x"`
    Y int `json:"y"`
    Plane data.Plane `json:"plane"`
    Banner data.BannerType `json:"banner"`
}

// we only really have to keep track of the non-instant events
type SerializedRandomEvent struct {
    Type gamecore.RandomEventType `json:"type"`
    Year uint64 `json:"year"`

    TargetCity *SerializedTargetCity `json:"city,omitempty"`

    // events that target players are all instant, so this will almost always be nil
    TargetPlayer *data.BannerType `json:"player,omitempty"`
}

func serializeRandomEvents(events []*gamecore.RandomEvent) []SerializedRandomEvent {
    out := make([]SerializedRandomEvent, 0, len(events))
    for _, event := range events {
        serialized := SerializedRandomEvent{
            Type: event.Type,
            Year: event.BirthYear,
        }

        if event.TargetCity != nil {
            serialized.TargetCity = &SerializedTargetCity{
                X: event.TargetCity.X,
                Y: event.TargetCity.Y,
                Plane: event.TargetCity.Plane,
                Banner: event.TargetCity.ReignProvider.GetBanner(),
            }
        }

        if event.TargetPlayer != nil {
            banner := event.TargetPlayer.GetBanner()
            serialized.TargetPlayer = &banner
        }

        out = append(out, serialized)
    }

    return out
}

type SerializedGame struct {
    Metadata serialize.SaveMetadata `json:"metadata"`
    Arcanus maplib.SerializedMap `json:"arcanus"`
    Myrror maplib.SerializedMap `json:"myrror"`
    Plane data.Plane `json:"plane"`
    // ArtifactPool is the old remaining-name list. New saves also write
    // ArtifactCatalog + ArtifactAvailable; load prefers those when present.
    ArtifactPool []string `json:"artifact-pool,omitempty"`
    ArtifactCatalog []artifact.SerializedArtifact `json:"artifact-catalog,omitempty"`
    ArtifactAvailable []bool `json:"artifact-available,omitempty"`
    Settings setup.NewGameSettings `json:"settings"`
    // remake UI prefs (end-of-turn wait, keybindings, ...). Omitted in
    // saves written before this field existed; json "settings" is the
    // new-game difficulty/land/magic struct, not this.
    Preferences *settingslib.SerializedSettings `json:"preferences,omitempty"`
    CurrentPlayer int `json:"current-player"`
    Turn uint64 `json:"turn"`
    LastEventTurn uint64 `json:"last-event-turn"`
    Players []playerlib.SerializedPlayer `json:"players"`
    Events []SerializedRandomEvent `json:"events"`
}

func SerializeModel(model *gamecore.GameModel, saveName string, prefs *settingslib.Settings) SerializedGame {
    var players []playerlib.SerializedPlayer
    for _, player := range model.Players {
        players = append(players, playerlib.SerializePlayer(player))
    }

    return SerializedGame{
        Metadata: serialize.SaveMetadata{
            Version: SerializeVersion,
            Date: time.Now(),
            Name: saveName,
        },
        Arcanus: maplib.SerializeMap(model.ArcanusMap),
        Myrror: maplib.SerializeMap(model.MyrrorMap),
        ArtifactPool: model.ArtifactPool.AvailableNames(),
        ArtifactCatalog: model.ArtifactPool.Serialize(),
        ArtifactAvailable: model.ArtifactPool.AvailableMask(),
        Plane:  model.Plane,
        Settings: model.Settings,
        Preferences: settingslib.SerializeSettings(prefs),
        CurrentPlayer: model.CurrentPlayer,
        Turn: model.TurnNumber,
        LastEventTurn: model.LastEventTurn,
        Players: players,
        Events: serializeRandomEvents(model.RandomEvents),
    }
}

func reconstructArtifactPool(serializedGame *SerializedGame, vanilla *artifact.Catalog, allSpells spellbook.Spells) *artifact.Catalog {
    if len(serializedGame.ArtifactCatalog) > 0 {
        return artifact.ReconstructCatalog(serializedGame.ArtifactCatalog, serializedGame.ArtifactAvailable, allSpells)
    }

    return vanilla.WithAvailableNames(serializedGame.ArtifactPool)
}

func reconstructRandomEvents(serializedEvents []SerializedRandomEvent, model *gamecore.GameModel) []*gamecore.RandomEvent {
    var out []*gamecore.RandomEvent

    findTargetCity := func(serializedCity *SerializedTargetCity) *citylib.City {
        city, _ := model.FindCity(serializedCity.X, serializedCity.Y, serializedCity.Plane)
        if city == nil {
            log.Printf("Warning: could not find target city at (%d, %d) on %v during event deserialization", serializedCity.X, serializedCity.Y, serializedCity.Plane)
        }
        return city
    }

    // implement later if needed
    /*
    findTargetPlayer := func(banner *data.BannerType) *playerlib.Player {
        return nil
    }
    */

    for _, event := range serializedEvents {
        switch event.Type {
            case gamecore.RandomEventBadMoon: out = append(out, gamecore.MakeBadMoonEvent(event.Year))
            case gamecore.RandomEventConjunctionChaos: out = append(out, gamecore.MakeConjunctionChaosEvent(event.Year))
            case gamecore.RandomEventConjunctionNature: out = append(out, gamecore.MakeConjunctionNatureEvent(event.Year))
            case gamecore.RandomEventConjunctionSorcery: out = append(out, gamecore.MakeConjunctionSorceryEvent(event.Year))
            case gamecore.RandomEventPopulationBoom: out = append(out, gamecore.MakePopulationBoomEvent(event.Year, findTargetCity(event.TargetCity)))
            case gamecore.RandomEventPlague: out = append(out, gamecore.MakePlagueEvent(event.Year, findTargetCity(event.TargetCity)))
            case gamecore.RandomEventDisjunction: out = append(out, gamecore.MakeDisjunctionEvent(event.Year))
            case gamecore.RandomEventGoodMoon: out = append(out, gamecore.MakeGoodMoonEvent(event.Year))
            case gamecore.RandomEventManaShort: out = append(out, gamecore.MakeManaShortEvent(event.Year))

            case gamecore.RandomEventDepletion, gamecore.RandomEventDiplomaticMarriage, gamecore.RandomEventRebellion,
                gamecore.RandomEventDonation, gamecore.RandomEventGift, gamecore.RandomEventGreatMeteor, gamecore.RandomEventPiracy,
                gamecore.RandomEventEarthquake, gamecore.RandomEventNewMinerals:
                log.Printf("Warning: unhandled event %d during deserialization", event.Type)
        }
    }

    return out
}

func MakeModelFromSerialized(
    serializedGame *SerializedGame, events chan GameEvent,
    heroNames map[int]map[herolib.HeroType]string,
    allSpells spellbook.Spells,
    artifactPool *artifact.Catalog,
    buildingInfo buildinglib.BuildingInfos,
    terrainData *terrain.TerrainData,
) *gamecore.GameModel {
    model := &gamecore.GameModel{
        Plane: serializedGame.Plane,
        ArtifactPool: reconstructArtifactPool(serializedGame, artifactPool, allSpells),
        Settings: serializedGame.Settings,
        HeroNames: heroNames,
        AllSpells: allSpells,
        CurrentPlayer: serializedGame.CurrentPlayer,
        TurnNumber: serializedGame.Turn,
        Events: events,
        BuildingInfo: buildingInfo,
        LastEventTurn: serializedGame.LastEventTurn,
    }

    var players []*playerlib.Player

    var cityInitializers []func(*maplib.Map, *maplib.Map)
    var playerInitializers []func([]*playerlib.Player)

    for _, serializedPlayer := range serializedGame.Players {
        player, initializeCities, initializePlayer := playerlib.ReconstructPlayer(&serializedPlayer, model, allSpells, buildingInfo, model)
        cityInitializers = append(cityInitializers, initializeCities)
        playerInitializers = append(playerInitializers, initializePlayer)

        if !player.Human {
            if player.GetBanner() == data.BannerBrown {
                player.AIBehavior = ai.MakeRaiderAI()
            } else {
                // use the same active wizard AI as a freshly-started game
                // (model.go AddPlayer) so loading a save doesn't silently drop
                // back to the legacy EnemyAI
                player.AIBehavior = ai.MakeEnemy2AI()
            }
            player.StrategicCombat = true
        }

        players = append(players, player)
    }

    for _, initializer := range playerInitializers {
        initializer(players)
    }

    model.Players = players

    wizards := make([]maplib.Wizard, 0, len(serializedGame.Players))
    for _, player := range model.Players {
        wizards = append(wizards, player)
    }

    model.ArcanusMap = maplib.ReconstructMap(serializedGame.Arcanus, terrainData, model, wizards)
    model.MyrrorMap = maplib.ReconstructMap(serializedGame.Myrror, terrainData, model, wizards)

    for _, initializer := range cityInitializers {
        initializer(model.ArcanusMap, model.MyrrorMap)
    }

    model.RandomEvents = reconstructRandomEvents(serializedGame.Events, model)

    return model
}
