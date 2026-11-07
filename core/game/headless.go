package game

import (
    "fmt"

    "github.com/bfalese/civ-of-magic/core/combat"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
    "github.com/bfalese/civ-of-magic/core/maplib"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// A game played by AI wizards only, without any frontend: no Ebitengine, no LBX files (the data
// comes from core/gamedata). Like watch mode, player 0 is a passive "human" observer with no
// cities or units, because parts of the model expect a human player to exist.
//
// Uses: a regression run after refactors, AI experiments, balance statistics.
//
// Spells go through GameModel.CastSpell; the ones it can't cast without the UI yet (and spells
// with no valid target) are counted in Stats.SkippedSpells.
type HeadlessGame struct {
    Model *GameModel
    Data *gamedata.GameData
    Stats HeadlessStats
    // log a summary line per wizard every year
    LogSummary bool
    // called with every model event as JSON (see EncodeEvent), if set
    OnEventJSON func(encoded []byte)
    // events that failed to encode, or model events without a JSON form
    EventErrors []error
}

type HeadlessStats struct {
    Battles int
    AttackerWins int
    Encounters int
    CitiesCaptured int
    CitiesRazed int
    SpellsCast int
    SpellsSkipped int
    // names of the skipped spells, with counts
    SkippedSpells map[string]int
    TreasuresFound int
}

type HeadlessSettings struct {
    Opponents int
    LandSize int
    Raiders bool
    Difficulty data.DifficultySetting
    Magic data.MagicSetting
}

// set up a new game with settings.Opponents AI wizards
func MakeHeadlessGame(gameData *gamedata.GameData, settings HeadlessSettings) (*HeadlessGame, error) {
    headless := makeHeadlessModel(gameData, settings)
    model := headless.Model

    observer, err := headless.addPlayers(nil, settings)
    if err != nil {
        return nil, err
    }

    // the observer takes no part in the game (same as watch mode)
    observer.Admin = true
    observer.Banished = true
    for _, city := range observer.GetCities() {
        observer.RemoveCity(city)
    }
    observer.Stacks = nil
    observer.SelectedStack = nil
    observer.Skip = true

    model.CurrentPlayer = 0
    headless.startPlayerTurn(observer)
    headless.drainEvents()

    return headless, nil
}

// the model and its data, no players yet
func makeHeadlessModel(gameData *gamedata.GameData, settings HeadlessSettings) *HeadlessGame {
    newGameSettings := setup.NewGameSettings{
        Opponents: settings.Opponents,
        Difficulty: settings.Difficulty,
        Magic: settings.Magic,
        LandSize: settings.LandSize,
        DisableRaiders: !settings.Raiders,
    }

    events := make(chan GameEvent, 1000)
    model := MakeGameModel(gamedata.TerrainData(), newGameSettings, data.PlaneArcanus, events, gameData.HeroNames, gameData.Spells, gameData.ArtifactCatalog(), gameData.Buildings)
    model.IsAggressiveAI = func() bool {
        return false
    }
    model.ArtifactPowers = gameData.ArtifactPowers

    return &HeadlessGame{
        Model: model,
        Data: gameData,
    }
}

// the human player (player 0; the given wizard, or a random one), settings.Opponents AI wizards and
// the raiders. Returns the human player.
func (headless *HeadlessGame) addPlayers(humanWizard *setup.WizardCustom, settings HeadlessSettings) (*playerlib.Player, error) {
    model := headless.Model
    gameData := headless.Data

    arcanusCityArea := model.MakeCityValidArea(data.PlaneArcanus)
    myrrorCityArea := model.MakeCityValidArea(data.PlaneMyrror)

    var wizard setup.WizardCustom
    if humanWizard != nil {
        wizard = *humanWizard
    } else {
        chosen, ok := ChooseUniqueWizard(nil, gameData.Spells)
        if !ok {
            return nil, fmt.Errorf("could not choose a wizard")
        }
        wizard = chosen
    }
    human, _ := model.InitializePlayer(wizard, true, arcanusCityArea, myrrorCityArea, headless.suggestCityName)

    for range settings.Opponents {
        wizard, ok := ChooseUniqueWizard(model.Players, gameData.Spells)
        if !ok {
            return nil, fmt.Errorf("could not choose a wizard for every opponent")
        }
        model.InitializePlayer(wizard, false, arcanusCityArea, myrrorCityArea, headless.suggestCityName)
    }

    if settings.Raiders {
        model.InitializeNeutralPlayer(arcanusCityArea, myrrorCityArea, headless.suggestCityName)
    }

    return human, nil
}

func (headless *HeadlessGame) suggestCityName(race data.Race) string {
    return headless.Model.SuggestCityName(headless.Data.CityNames, race)
}

// the AI wizards still in the game (not the observer, not the raiders)
func (headless *HeadlessGame) WizardsLeft() []*playerlib.Player {
    var out []*playerlib.Player
    for _, player := range headless.Model.Players {
        if player.IsHuman() || player.IsNeutral() || player.Defeated {
            continue
        }
        out = append(out, player)
    }
    return out
}

// one player's turn, then hand over to the next player (the year advances after the last one)
func (headless *HeadlessGame) Step() {
    model := headless.Model
    player := model.Players[model.CurrentPlayer]

    if !player.IsHuman() {
        model.DoAiTurn(headless, player, headless.suggestCityName)
        headless.drainEvents()
        headless.DiscoverWizards()
    }

    model.DoNextTurn(false, headless.LogSummary, headless.startPlayerTurn)
    headless.drainEvents()
}

// play until the given number of years passed or at most one wizard is left.
// returns the number of years played.
func (headless *HeadlessGame) RunYears(years int) int {
    start := headless.Model.TurnNumber
    for headless.Model.TurnNumber < start + uint64(years) && len(headless.WizardsLeft()) > 1 {
        headless.Step()
    }
    return int(headless.Model.TurnNumber - start)
}

func (headless *HeadlessGame) startPlayerTurn(player *playerlib.Player) {
    if !headless.Model.StartPlayerTurn(player, headless.castSpell) {
        return
    }

    if player.GlobalEnchantments.Contains(data.EnchantmentAwareness) {
        headless.Model.ExploreFogForAwareness(player)
    }

    headless.DiscoverWizards()
}

// a spell finished casting: cast it through the model with a target chosen here (the AI picks
// targets only for unit spells)
func (headless *HeadlessGame) castSpell(player *playerlib.Player, spell spellbook.Spell) {
    model := headless.Model
    var target SpellTarget

    if rule, ok := model.UnitSpellRule(spell); ok {
        target.Unit = player.CastingSpellTarget
        player.CastingSpellTarget = nil
        if target.Unit == nil || player.FindStackByUnit(target.Unit) == nil {
            target.Unit = nil
            for unit := range player.Units() {
                if rule.Check(unit) == nil {
                    target.Unit = unit
                    break
                }
            }
        }
    } else if rule, ok := model.CitySpellRule(player, spell); ok && rule.LocationType == LocationTypeFriendlyCity {
        for _, city := range player.Cities {
            if !city.HasEnchantment(rule.Enchantment) {
                target.Location = &Location{Plane: city.Plane, X: city.X, Y: city.Y}
                break
            }
        }
    } else if locationType, ok := model.SpellTargetType(player, spell); ok {
        switch locationType {
            case LocationTypeWizard:
                for _, other := range model.Players {
                    if other != player && !other.IsHuman() && !other.IsNeutral() && !other.Defeated && !other.Banished && player.IsAwareOf(other) {
                        target.Wizard = other
                        break
                    }
                }
            case LocationTypeEnemyCity:
                for _, other := range model.GetEnemies(player) {
                    for _, city := range other.Cities {
                        if player.IsExplored(city.X, city.Y, city.Plane) {
                            target.Location = &Location{Plane: city.Plane, X: city.X, Y: city.Y}
                            break
                        }
                    }
                    if target.Location != nil {
                        break
                    }
                }
        }
    }

    if err := model.CastSpell(player, spell, target); err != nil {
        headless.Stats.SpellsSkipped += 1
        if headless.Stats.SkippedSpells == nil {
            headless.Stats.SkippedSpells = make(map[string]int)
        }
        headless.Stats.SkippedSpells[spell.Name] += 1
        return
    }

    headless.Stats.SpellsCast += 1
}

// handle what the model sent: AI players answer offers and take treasure; screens are skipped
func (headless *HeadlessGame) drainEvents() {
    for {
        select {
            case event := <-headless.Model.Events:
                headless.encodeEvent(event)
                headless.handleAiEvent(event)
            default:
                return
        }
    }
}

// the AI players' part of an event: they answer offers and take treasure
func (headless *HeadlessGame) handleAiEvent(event GameEvent) {
    model := headless.Model
    switch event.(type) {
        case *GameEventHireHero:
            hire := event.(*GameEventHireHero)
            if !hire.Player.IsHuman() && hire.Player.AIBehavior != nil {
                hire.Player.AIBehavior.HandleHireHero(hire.Player, hire.Hero, 0, true, data.PlanePoint{})
            }
        case *GameEventHireMercenaries:
            hire := event.(*GameEventHireMercenaries)
            if !hire.Player.IsHuman() && hire.Player.AIBehavior != nil {
                hire.Player.AIBehavior.HandleHireMercenaries(hire.Player, hire.Units, hire.Cost)
            }
        case *GameEventMerchant:
            merchant := event.(*GameEventMerchant)
            if !merchant.Player.IsHuman() && merchant.Player.AIBehavior != nil {
                merchant.Player.AIBehavior.HandleMerchantItem(merchant.Player, merchant.Artifact, merchant.Cost)
            }
        case *GameEventTreasure:
            treasure := event.(*GameEventTreasure)
            if !treasure.Player.IsHuman() {
                headless.Stats.TreasuresFound += 1
                model.ApplyTreasure(nil, treasure.Player, treasure.Treasure, model.AllSpells)
            }
    }
}

func (headless *HeadlessGame) encodeEvent(event GameEvent) {
    encoded, ok, err := headless.Model.EncodeEvent(event)
    if err != nil {
        headless.EventErrors = append(headless.EventErrors, fmt.Errorf("%T: %v", event, err))
        return
    }
    if !ok {
        headless.EventErrors = append(headless.EventErrors, fmt.Errorf("%T has no JSON form", event))
        return
    }
    if headless.OnEventJSON != nil {
        headless.OnEventJSON(encoded)
    }
}

// MovementHandler: battles, encounters and city capture without screens

func (headless *HeadlessGame) ShowMovement(x int, y int, stack *playerlib.UnitStack, center bool) {
}

func (headless *HeadlessGame) DiscoverWizards() bool {
    return headless.Model.AnnounceVisibleWizards()
}

func (headless *HeadlessGame) DefeatCity(attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, city *citylib.City) (bool, int) {
    raze := false
    if attacker.AIBehavior != nil {
        raze = attacker.AIBehavior.ConfirmRazeTown(city)
    }

    if raze {
        headless.Stats.CitiesRazed += 1
    } else {
        headless.Stats.CitiesCaptured += 1
    }

    return raze, headless.Model.DefeatCity(attacker, defender, city, raze)
}

func (headless *HeadlessGame) DoCombat(attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, defenderStack *playerlib.UnitStack, zone combat.ZoneType) combat.CombatState {
    model := headless.Model
    events := make(chan combat.CombatEvent, 1000)

    battle := model.MakeBattle(model.AllSpells, attacker, attackerStack, defender, defenderStack, zone, events)
    result := battle.Run(false)

    headless.Stats.Battles += 1
    if result.AttackerWon() {
        headless.Stats.AttackerWins += 1
    }

    model.ApplyBattleOutcome(battle, result, headless.DefeatCity)
    model.ApplyBattleAftermath(battle, result)

    return result.State
}

func (headless *HeadlessGame) DoEncounter(player *playerlib.Player, stack *playerlib.UnitStack, encounter *maplib.ExtraEncounter, mapUse *maplib.Map, x int, y int) combat.CombatState {
    model := headless.Model
    point := data.PlanePoint{X: x, Y: y, Plane: mapUse.Plane}
    headless.Stats.Encounters += 1

    if len(encounter.Units) == 0 {
        mapUse.RemoveEncounter(x, y)
        model.CreateTreasure(model.AllSpells.Copy(), headless.Data.ArtifactPowers, encounter.Type, encounter.Budget, point, player)
        return combat.CombatStateNoCombat
    }

    defender, enemies, zone := model.EncounterDefenders(encounter, mapUse.Plane, x, y)
    result := headless.DoCombat(player, stack, defender, playerlib.MakeUnitStackFromUnits(enemies), zone)
    if model.ResolveEncounter(encounter, mapUse, x, y, enemies, result) {
        model.CreateTreasure(model.AllSpells.Copy(), headless.Data.ArtifactPowers, encounter.Type, encounter.Budget, point, player)
    }

    return result
}

// one line per wizard: name, cities, units, gold, mana, fame, defeated
func (headless *HeadlessGame) Summary() string {
    out := fmt.Sprintf("Year %v (turn %v)\n", headless.Model.TurnDate(), headless.Model.TurnNumber)
    for _, player := range headless.Model.Players {
        if player.IsHuman() {
            continue
        }
        status := ""
        if player.Defeated {
            status = " DEFEATED"
        } else if player.Banished {
            status = " banished"
        }
        out += fmt.Sprintf("  %-12v cities=%-3v units=%-3v gold=%-5v mana=%-5v fame=%-3v spells=%v%v\n",
            player.Wizard.Name, len(player.Cities), player.UnitCount(), player.Gold, player.Mana, player.Fame, len(player.KnownSpells.Spells), status)
    }
    stats := headless.Stats
    out += fmt.Sprintf("  battles=%v (attacker won %v) encounters=%v treasures=%v cities captured=%v razed=%v spells cast=%v skipped=%v %v",
        stats.Battles, stats.AttackerWins, stats.Encounters, stats.TreasuresFound, stats.CitiesCaptured, stats.CitiesRazed, stats.SpellsCast, stats.SpellsSkipped, stats.SkippedSpells)
    return out
}

