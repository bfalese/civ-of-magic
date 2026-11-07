package game

import (
    "fmt"
    "log"
    "strings"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// the start of a player's turn: upkeep, income, spell casting and research progress, city growth
// and production, unit healing/experience, random offers. Returns false if the player skips the
// turn. castSpell is called (by the frontend) when the spell being cast completes; it runs at the same
// point in the turn as before, so later steps see its effects.
func (model *GameModel) StartPlayerTurn(player *playerlib.Player, castSpell func(*playerlib.Player, spellbook.Spell)) bool {
    if player.IsHuman() {
        model.ScrollEvents = nil
    }
    // a spell trade offered last turn isn't on the table any more
    delete(model.PendingTrades, player)

    if player.Skip {
        return false
    }

    player.MovedStacksThisTurn = 0
    player.SuppressAutoNextTurn = false

    disbandedMessages := model.DisbandUnits(player)

    if player.IsHuman() && len(disbandedMessages) > 0 {
        select {
            case model.Events<- &GameEventScroll{Title: "", Text: strings.Join(disbandedMessages, "\n")}:
            default:
        }
    }

    power := model.ComputePower(player)

    model.DissipateEnchantments(player, power)

    // timestop may have dissipated by now
    timeStop := player.HasEnchantment(data.EnchantmentTimeStop)

    player.Gold += player.GoldPerTurn()
    if player.Gold < 0 {
        player.Gold = 0
    }

    player.Mana += player.ManaPerTurn(power, model)

    if timeStop {
        player.Mana -= data.EnchantmentTimeStop.UpkeepMana()
    }

    player.Mana = max(0, player.Mana)

    if !player.CastingSpell.Invalid() {
        // mana spent on the skill is the minimum of {player's mana, casting skill, remaining cost for spell}
        manaSpent := player.Mana
        if manaSpent > player.RemainingCastingSkill {
            manaSpent = player.RemainingCastingSkill
        }

        spellCost := player.ComputeEffectiveSpellCost(player.CastingSpell, true)

        remainingMana := spellCost - player.CastingSpellProgress
        if remainingMana < manaSpent {
            manaSpent = remainingMana
        }

        player.IncreaseCastingSkillProgress(manaSpent)

        if spellCost <= player.CastingSpellProgress {
            castSpell(player, player.CastingSpell)
            player.CastingSpell = spellbook.Spell{}
            player.CastingSpellProgress = 0
        }
    }

    if player.ResearchingSpell.Valid() {
        if !timeStop {
            // log.Printf("wizard %v power=%v researching=%v progress=%v/%v perturn=%v", player.Wizard.Name, power, player.ResearchingSpell.Name, player.ResearchProgress, player.ResearchingSpell.ResearchCost, player.SpellResearchPerTurn(power))
            player.ResearchProgress += player.ComputeEffectiveResearchPerTurn(player.SpellResearchPerTurn(power), player.ResearchingSpell)
            if player.ResearchProgress >= player.ResearchingSpell.ResearchCost {

                if player.IsHuman() {
                    select {
                        case model.Events<- &GameEventLearnedSpell{Player: player, Spell: player.ResearchingSpell}:
                        default:
                    }
                }

                log.Printf("wizard %v learned %v", player.Wizard.Name, player.ResearchingSpell.Name)

                player.LearnSpell(player.ResearchingSpell)

                if player.IsHuman() {
                    select {
                        case model.Events<- &GameEventResearchSpell{Player: player}:
                        default:
                    }
                }
            }
        }
    } else if model.TurnNumber > 0 {

        if player.IsHuman() {
            select {
                case model.Events<- &GameEventResearchSpell{Player: player}:
                default:
            }
        }
    }

    player.CastingSkillPower += player.CastingSkillPerTurn(power)

    // reset casting skill for this turn
    player.RemainingCastingSkill = player.ComputeOverworldCastingSkill()

    var removeCities []*citylib.City

    if !timeStop {
        for _, city := range player.Cities {
            cityEvents := city.DoNextTurn(model.GetMap(city.Plane))
            for _, event := range cityEvents {
                switch event.(type) {
                case *citylib.CityEventPopulationGrowth:
                    if player.IsHuman() {
                        growthEvent := event.(*citylib.CityEventPopulationGrowth)

                        verb := "grown"
                        if !growthEvent.Grow {
                            verb = "shrunk"
                        }

                        scrollEvent := GameEventScroll{
                            Title: "CITY GROWTH",
                            Text: fmt.Sprintf("%v has %v to a population of %v.", city.Name, verb, city.Citizens()),
                        }

                        select {
                            case model.Events<- &scrollEvent:
                            default:
                        }
                    }
                case *citylib.CityEventCityAbandoned:
                    removeCities = append(removeCities, city)
                    if player.IsHuman() {
                        select {
                            case model.Events<- &GameEventNotice{Message: fmt.Sprintf("The city of %v has been abandoned.", city.Name)}:
                            default:
                        }
                    }
                case *citylib.CityEventNewBuilding:
                    newBuilding := event.(*citylib.CityEventNewBuilding)

                    if player.IsHuman() {
                        select {
                            case model.Events<- &GameEventNewBuilding{City: city, Building: newBuilding.Building, Player: player}:
                            default:
                        }
                    } else {
                        log.Printf("Year=%v AI %v(%v) city %v created %v", model.TurnNumber, player.Wizard.Name, player.GetBanner(), city.Name, model.BuildingInfo.Name(newBuilding.Building))
                    }
                case *citylib.CityEventOutpostDestroyed:
                    removeCities = append(removeCities, city)
                    if player.IsHuman() {
                        select {
                            case model.Events<- &GameEventNotice{Message: fmt.Sprintf("The outpost of %v has been deserted.", city.Name)}:
                            default:
                        }
                    }
                case *citylib.CityEventOutpostHamlet:
                    if player.IsHuman() {
                        select {
                            case model.Events<- &GameEventNotice{Message: fmt.Sprintf("The outpost of %v has grown into a hamlet.", city.Name)}:
                            default:
                        }
                    }
                case *citylib.CityEventNewUnit:
                    newUnit := event.(*citylib.CityEventNewUnit)
                    overworldUnit := units.MakeOverworldUnitFromUnit(newUnit.Unit, city.X, city.Y, city.Plane, city.GetBanner(), player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
                    // only normal units get weapon bonuses
                    if overworldUnit.GetRace() != data.RaceFantastic {
                        overworldUnit.SetWeaponBonus(newUnit.WeaponBonus)
                    }

                    // automatically apply chaos channels to new normal units
                    // checking the race is probably redundant because a new unit built by the city will never be a hero nor fantastic
                    if overworldUnit.GetRace() != data.RaceHero && overworldUnit.GetRace() != data.RaceFantastic && player.HasEnchantment(data.EnchantmentDoomMastery) {
                        model.ApplyChaosChannels(overworldUnit)
                    }

                    overworldUnit.AddExperience(newUnit.Experience)
                    player.CreateUnit(overworldUnit)
                    model.ResolveStackAt(city.X, city.Y, city.Plane)

                    if player.AIBehavior != nil {
                        player.AIBehavior.ProducedUnit(city, player)
                        log.Printf("Year=%v AI %v(%v) city %v created unit %v", model.TurnNumber, player.Wizard.Name, player.GetBanner(), city.Name, overworldUnit.GetName())
                    }
                }
            }
        }
    }

    model.DoBuildRoads(player)
    model.DoPurify(player)

    for _, stack := range player.Stacks {

        // every unit gains 1 experience at each turn
        unitExperience := model.GetExperienceBonus(stack)
        for _, unit := range stack.Units() {
            switch unit.GetRace() {
                case data.RaceHero: model.AddExperience(player, unit, 1)
                case data.RaceFantastic: // nothing
                default:
                    model.AddExperience(player, unit, unitExperience)
            }
        }

        // base healing rate is 5%. in a town is 10%, with animists guild is 16.67%
        rate := 0.05

        city := player.FindCity(stack.X(), stack.Y(), stack.Plane())

        if city != nil {
            rate = 0.1
            if city.Buildings.Contains(buildinglib.BuildingAnimistsGuild) {
                rate = 0.1667
            }

            if city.HasEnchantment(data.CityEnchantmentStreamOfLife) {
                rate = 1
            }
        }

        // any healer in the same stack provides an additional 20% healing rate
        for _, unit := range stack.Units() {
            if unit.HasAbility(data.AbilityHealer) {
                rate += 0.2
                break
            }
        }

        // heal all the way up
        if player.HasEnchantment(data.EnchantmentHerbMastery) {
            rate = 1
        }

        HandleStasis(stack)

        stack.NaturalHeal(rate)
        stack.ResetMoves()
        stack.EnableMovers()
    }

    for _, city := range removeCities {
        player.RemoveCity(city)
    }

    model.MaybeHireHero(player)
    model.MaybeHireMercenaries(player)
    model.MaybeBuyFromMerchant(player)

    player.UpdateFogVisibility()

    return true
}

// what happens once all players have taken their turn. randomEvents: whether new random events may
// start (a player preference); logSummary: log a per-wizard summary (watch mode).
func (model *GameModel) EndOfTurn(randomEvents bool, logSummary bool) {
    // put stuff here that should happen when all players have taken their turn

    model.pruneUnitIds()

    for _, player := range model.Players {
        if !player.Defeated {
            player.AddPowerHistory(model.ComputeWizardPower(player))
        }
    }

    model.RevertVolcanos()

    // FIXME: the wiki says armageddon will not do anything while time stop is in effect.
    // figure out what other global spells don't have any effect (great wasting, chaos rift, meteor storm)
    if !model.HasEnchantment(data.EnchantmentTimeStop) {
        model.DoArmageddon()
    }

    model.DoGreatWasting()

    model.DoChaosRift()

    model.DoMeteorStorm()

    model.TurnNumber += 1

     // gate random-event rolls behind the user toggle; in-flight events continue
     // their normal decay path inside DoRandomEvents and this just skips new rolls.
    if randomEvents {
        model.DoRandomEvents()
    }

    for _, player := range model.Players {
        if player.Defeated || player.Banished {
            continue
        }

        player.UpdateDiplomaticRelations()
    }

    if logSummary {
        log.Printf("---- End of Year %v Summary ----", model.TurnNumber - 1)
        for _, player := range model.Players {
            if !player.IsHuman() {
                power := model.ComputePower(player)
                log.Printf("Wizard %v (%v) - Gold: %v (%v), Food: %v, Mana: %v (%v), Cities: %v, Units: %v, Tax: %v, Research: %v (%v)",
                    player.Wizard.Name, player.GetBanner(), player.Gold, player.GoldPerTurn(), player.FoodPerTurn(), player.Mana, player.ManaPerTurn(power, model),
                    len(player.Cities), player.UnitCount(), player.TaxRate,
                    player.ResearchingSpell.Name, player.ResearchProgress)
            }
        }
    }

}

// advances to the next player (ending the global turn when everyone has played) and starts their
// turn. startPlayerTurn is the frontend's turn start (it calls StartPlayerTurn and does its own
// follow-up); see EndOfTurn for the flags.
func (model *GameModel) DoNextTurn(randomEvents bool, logSummary bool, startPlayerTurn func(*playerlib.Player)) {
    // if time stop is enabled then don't move to the other players, just keep doing the current player
    if model.CurrentPlayer >= 0 && model.Players[model.CurrentPlayer].HasEnchantment(data.EnchantmentTimeStop) {
        model.EndOfTurn(randomEvents, logSummary)
    } else {
        model.CurrentPlayer += 1
        if model.CurrentPlayer >= len(model.Players) {
            // all players did their turn, so the next global turn starts
            model.EndOfTurn(randomEvents, logSummary)
            model.CurrentPlayer = 0
        }
    }

    if len(model.Players) > 0 {
        player := model.Players[model.CurrentPlayer]

        aiPlayer := model.Players[model.CurrentPlayer]

        if aiPlayer.AIBehavior != nil {
            aiPlayer.AIBehavior.PreTurn(aiPlayer)
        }

        if player.Wizard.Banner != data.BannerBrown {
            startPlayerTurn(player)
        } else {
            // neutral enemies should reset their moves each turn
            for _, stack := range player.Stacks {
                HandleStasis(stack)
                stack.ResetMoves()
                stack.EnableMovers()
            }
        }

        if aiPlayer.AIBehavior != nil {
            aiPlayer.AIBehavior.NewTurn(aiPlayer)
        }
    }
}
