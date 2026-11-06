package game

import (
    "math/rand/v2"
    "slices"

    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/combat"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/data"
)

// An overworld battle: the attacking stack moved onto the defending stack.
// Flow (the frontend drives it, since the battle screen and dialogs sit in between):
//   battle := model.MakeBattle(...)
//   run battle.Combat (combat screen, combat.Run, or combat.DoStrategicCombat)
//   outcome := model.ApplyBattleOutcome(battle, result, defeatCity)   // before the end screen
//   aftermath := model.ApplyBattleAftermath(battle, result, outcome)  // after the end screen
type Battle struct {
    Attacker *playerlib.Player
    AttackerStack *playerlib.UnitStack
    Defender *playerlib.Player
    DefenderStack *playerlib.UnitStack
    Zone combat.ZoneType
    Landscape combat.CombatLandscape
    // the armies are Combat.AttackingArmy and Combat.DefendingArmy
    Combat *combat.CombatModel
}

type BattleResult struct {
    State combat.CombatState
    DefeatedAttackers int
    DefeatedDefenders int
}

// what changed on the overworld, as shown on the combat end screen
type BattleOutcome struct {
    AttackerFame int
    DefenderFame int
    CityPopulationLoss int
    CityBuildingLoss []buildinglib.Building
}

type BattleAftermath struct {
    // number of units raised as undead by the winner
    UndeadUnits int
    // a hero of a human player died and its equipment went to the vault
    HumanHeroDied bool
}

func (result BattleResult) AttackerWon() bool {
    return result.State == combat.CombatStateAttackerWin || result.State == combat.CombatStateDefenderFlee
}

func (result BattleResult) DefenderWon() bool {
    return result.State == combat.CombatStateDefenderWin || result.State == combat.CombatStateAttackerFlee
}

// set up the combat model for a battle between two overworld stacks
func (model *GameModel) MakeBattle(allSpells spellbook.Spells, attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, defenderStack *playerlib.UnitStack, zone combat.ZoneType, events chan combat.CombatEvent) *Battle {
    landscape := model.GetCombatLandscape(defenderStack.X(), defenderStack.Y(), defenderStack.Plane())

    createArmy := func (player *playerlib.Player, stack *playerlib.UnitStack) *combat.Army {
        army := combat.Army{
            Player: player,
        }

        for _, unit := range stack.Units() {
            // flying units always get added to battle
            if !unit.IsFlying() {
                if landscape == combat.CombatLandscapeWater && unit.IsLandWalker() {
                    continue
                }

                // dont add sailing units to non-water combat
                if landscape != combat.CombatLandscapeWater && unit.IsSailing() {
                    continue
                }
            }

            army.AddUnit(unit)
        }

        return &army
    }

    attackingArmy := createArmy(attacker, attackerStack)
    defendingArmy := createArmy(defender, defenderStack)

    combatModel := combat.MakeCombatModel(allSpells, defendingArmy, attackingArmy, landscape, defenderStack.Plane(), zone, model.GetInfluenceMagic(attackerStack.X(), attackerStack.Y(), attackerStack.Plane()), attackerStack.X(), attackerStack.Y(), events, nil)

    if zone.City != nil && zone.City.HasEnchantment(data.CityEnchantmentHeavenlyLight) {
        combatModel.AddGlobalEnchantment(data.CombatEnchantmentTrueLight)
    }
    if zone.City != nil && zone.City.HasEnchantment(data.CityEnchantmentCloudOfShadow) {
        combatModel.AddGlobalEnchantment(data.CombatEnchantmentDarkness)
    } else {
        for _, enchantments := range model.GetAllGlobalEnchantments() {
            if enchantments.Contains(data.EnchantmentEternalNight) {
                combatModel.AddGlobalEnchantment(data.CombatEnchantmentDarkness)
                break
            }
        }
    }

    return &Battle{
        Attacker: attacker,
        AttackerStack: attackerStack,
        Defender: defender,
        DefenderStack: defenderStack,
        Zone: zone,
        Landscape: landscape,
        Combat: combatModel,
    }
}

// the result of a battle fought by the combat model (combat screen or combat.Run)
func (battle *Battle) Result(state combat.CombatState) BattleResult {
    return BattleResult{
        State: state,
        DefeatedAttackers: battle.Combat.DefeatedAttackers,
        DefeatedDefenders: battle.Combat.DefeatedDefenders,
    }
}

// fight the battle without a screen: auto-resolve when strategic is true, otherwise
// run the combat model with both sides controlled by the AI
func (battle *Battle) Run(strategic bool) BattleResult {
    if strategic {
        state, defeatedAttackers, defeatedDefenders := combat.DoStrategicCombat(battle.Combat.AttackingArmy, battle.Combat.DefendingArmy)
        return BattleResult{State: state, DefeatedAttackers: defeatedAttackers, DefeatedDefenders: defeatedDefenders}
    }

    return battle.Result(combat.Run(battle.Combat))
}

// first part of applying a battle to the overworld: city capture, fame, units changing sides
// and collateral damage to the city. defeatCity is called when the attacker wins at a city;
// it decides whether to raze (the frontend asks a human) and returns (raze, plundered gold).
func (model *GameModel) ApplyBattleOutcome(battle *Battle, result BattleResult, defeatCity func(attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, city *citylib.City) (bool, int)) BattleOutcome {
    attacker := battle.Attacker
    attackerStack := battle.AttackerStack
    defender := battle.Defender
    defenderStack := battle.DefenderStack
    zone := battle.Zone
    attackingArmy := battle.Combat.AttackingArmy
    defendingArmy := battle.Combat.DefendingArmy

    // returns the fame that should be added to the winner and loser. the loser fame is negative
    distributeFame := func(winner *playerlib.Player, loser *playerlib.Player, loserStack *playerlib.UnitStack, defeatedUnits int) (int, int) {
        winnerFame := 0
        loserFame := 0

        if defeatedUnits >= 4 {
            winnerFame += 1
            loserFame -= 1
        }

        for _, unit := range loserStack.Units() {
            if unit.GetRawUnit().CastingCost >= 600 {
                winnerFame += 1
                loserFame -= 1
                break
            }
            if unit.IsHero() {
                hero := unit.(*herolib.Hero)
                loserFame -= (int(hero.GetExperienceLevel()) + 1) / 2
            }
        }

        return winnerFame, loserFame
    }

    // fame
    var attackerFame, defenderFame int

    if result.AttackerWon() {
        if zone.City != nil {
            razeCity, gold := defeatCity(attacker, attackerStack, defender, zone.City)
            // if razeCity is true then we pass in false to get the fame for capturing the city
            attackerFame += zone.City.FameForCaptureOrRaze(!razeCity)
            defenderFame += zone.City.FameForCaptureOrRaze(false)

            attacker.Gold += gold
            defender.Gold -= gold
        }

        winner, loser := distributeFame(attacker, defender, defenderStack, result.DefeatedDefenders)
        attackerFame += winner
        defenderFame += loser

        // some units might have been raised from the dead but been owned by the opposite side
        for _, unit := range attackingArmy.GetUnits() {
            if unit.Unit.GetHealth() > 0 {
                attackerStack.AddUnit(unit.Unit)
                // FIXME: its annoying to have to set all 3 of these fields. maybe consolidate them into just
                // SetOwner(player)
                unit.Unit.SetBanner(attacker.GetBanner())
                unit.Unit.SetGlobalEnchantmentProvider(attacker.MakeUnitEnchantmentProvider())
                unit.Unit.SetExperienceInfo(attacker.MakeExperienceInfo())
                defender.RemoveUnit(unit.Unit)
            }
        }

    } else if result.DefenderWon() {
        winner, loser := distributeFame(defender, attacker, attackerStack, result.DefeatedAttackers)
        defenderFame += winner
        attackerFame += loser

        for _, unit := range defendingArmy.GetUnits() {
            if unit.Unit.GetHealth() > 0 {
                defenderStack.AddUnit(unit.Unit)
                unit.Unit.SetBanner(defender.GetBanner())
                unit.Unit.SetGlobalEnchantmentProvider(defender.MakeUnitEnchantmentProvider())
                unit.Unit.SetExperienceInfo(defender.MakeExperienceInfo())
                attacker.RemoveUnit(unit.Unit)
            }
        }
    }

    attacker.Fame = max(0, attacker.Fame + attackerFame)
    defender.Fame = max(0, defender.Fame + defenderFame)

    cityPopulationLoss := 0
    var cityBuildingLoss []buildinglib.Building

    if zone.City != nil && result.State == combat.CombatStateAttackerWin {
        // maximum chance is 50%, minimum is 10%
        chance := min(50, 10 + battle.Combat.CollateralDamage * 2)
        for range zone.City.Citizens() - 1 {
            if rand.N(100) < chance {
                cityPopulationLoss += 1
            }
        }

        minBuildingChance := 10
        if attacker.GetBanner() == data.BannerBrown {
            minBuildingChance = 50
        }

        chance = min(75, minBuildingChance + battle.Combat.CollateralDamage)
        for _, building := range zone.City.Buildings.Values() {
            if building == buildinglib.BuildingFortress || building == buildinglib.BuildingSummoningCircle {
                continue
            }

            if rand.N(100) < chance {
                cityBuildingLoss = append(cityBuildingLoss, building)
            }
        }

        zone.City.Population -= cityPopulationLoss * 1000
        for _, building := range cityBuildingLoss {
            zone.City.Buildings.Remove(building)
        }
        zone.City.ResetCitizens()
    }

    return BattleOutcome{
        AttackerFame: attackerFame,
        DefenderFame: defenderFame,
        CityPopulationLoss: cityPopulationLoss,
        CityBuildingLoss: cityBuildingLoss,
    }
}

// second part of applying a battle to the overworld: recalled units go home, dead units are
// removed (heroes' equipment goes to the vault), undead and zombies join the winner, and the
// winner's units gain experience. Vault events are sent on model.Events.
func (model *GameModel) ApplyBattleAftermath(battle *Battle, result BattleResult) BattleAftermath {
    attacker := battle.Attacker
    attackerStack := battle.AttackerStack
    defender := battle.Defender
    defenderStack := battle.DefenderStack
    landscape := battle.Landscape
    combatModel := battle.Combat
    attackingArmy := combatModel.AttackingArmy
    defendingArmy := combatModel.DefendingArmy

    var recalledAttackers []units.StackUnit
    var recalledDefenders []units.StackUnit

    // FIXME: resolve the attacker/defender stack at the end of combat?
    for _, unit := range combatModel.AttackingArmy.RecalledUnits {
        recalledAttackers = append(recalledAttackers, unit.Unit.(units.StackUnit))
    }
    for _, unit := range combatModel.DefendingArmy.RecalledUnits {
        recalledDefenders = append(recalledDefenders, unit.Unit.(units.StackUnit))
    }

    // Redistribute equipment of died heros
    showHeroNotice := false

    // recall units
    relocateUnits := func(player *playerlib.Player, units []units.StackUnit) {
        for _, unit := range units {
            model.RelocateUnit(player, unit)
        }
    }

    relocateUnits(attacker, recalledAttackers)
    relocateUnits(defender, recalledDefenders)

    // remove dead units
    killUnits := func (player *playerlib.Player, stack *playerlib.UnitStack, landscape combat.CombatLandscape){
        // first remove sailing units
        for _, unit := range stack.Units() {
            if unit.IsSailing() && unit.GetHealth() <= 0 {
                player.LoseUnit(unit)
            }
        }

        transport := stack.HasSailingUnits(false)

        for _, unit := range stack.Units() {
            dead := unit.GetHealth() <= 0

            // if combat was on water and there are no sailing ships left then all units should die
            // FIXME: handle the case that there were originally two ships and one died, thus not being able to transport some units
            if landscape == combat.CombatLandscapeWater && unit.IsLandWalker() && !transport {
                dead = true
            }

            if dead {
                player.RemoveUnit(unit)

                if unit.IsHero() {
                    hero := unit.(*herolib.Hero)
                    vaultEvents := model.DistributeEquipment(player, hero)
                    if player.IsHuman() {
                        showHeroNotice = showHeroNotice || len(vaultEvents) > 0
                    }

                    for _, event := range vaultEvents {
                        select {
                            case model.Events <- event:
                            default:
                        }
                    }

                }
            }
        }
    }

    killUnits(attacker, attackerStack, landscape)
    killUnits(defender, defenderStack, landscape)

    var undeadUnits int

    switch result.State {
        case combat.CombatStateAttackerWin, combat.CombatStateDefenderFlee:
            undeadUnits = len(combatModel.UndeadUnits)

            for _, unit := range combatModel.UndeadUnits {
                defender.RemoveUnit(unit.Unit)
                if len(attackerStack.Units()) < data.MaxUnitsInStack {
                    attacker.AddUnit(attacker.UpdateUnit(unit.Unit))
                }
            }

            if attacker.HasEnchantment(data.EnchantmentZombieMastery) {
                for _, unit := range append(slices.Clone(defendingArmy.KilledUnits), attackingArmy.KilledUnits...) {
                    if unit.GetRace() != data.RaceFantastic && unit.GetRace() != data.RaceHero && len(attackerStack.Units()) < data.MaxUnitsInStack {
                        attacker.AddUnit(units.MakeOverworldUnitFromUnit(units.Zombie, attackerStack.X(), attackerStack.Y(), attackerStack.Plane(), attacker.GetBanner(), attacker.MakeExperienceInfo(), attacker.MakeUnitEnchantmentProvider()))
                    }
                }
            }

        case combat.CombatStateDefenderWin, combat.CombatStateAttackerFlee:
            undeadUnits = len(combatModel.UndeadUnits)

            for _, unit := range combatModel.UndeadUnits {
                attacker.RemoveUnit(unit.Unit)
                if len(defenderStack.Units()) < data.MaxUnitsInStack {
                    defender.AddUnit(defender.UpdateUnit(unit.Unit))
                }
            }

            if defender.HasEnchantment(data.EnchantmentZombieMastery) {
                for _, unit := range append(slices.Clone(defendingArmy.KilledUnits), attackingArmy.KilledUnits...) {
                    if unit.GetRace() != data.RaceFantastic && unit.GetRace() != data.RaceHero && len(defenderStack.Units()) < data.MaxUnitsInStack {
                        defender.AddUnit(units.MakeOverworldUnitFromUnit(units.Zombie, defenderStack.X(), defenderStack.Y(), defenderStack.Plane(), defender.GetBanner(), defender.MakeExperienceInfo(), defender.MakeUnitEnchantmentProvider()))
                    }
                }
            }
    }

    // experience
    if result.AttackerWon() {
        for _, unit := range attackerStack.Units() {
            if unit.GetRace() != data.RaceFantastic {
                model.AddExperience(attacker, unit, result.DefeatedDefenders * 2)
            }
        }
    } else if result.DefenderWon() {
        for _, unit := range defenderStack.Units() {
            if unit.GetRace() != data.RaceFantastic {
                model.AddExperience(defender, unit, result.DefeatedAttackers * 2)
            }
        }
    }

    return BattleAftermath{
        UndeadUnits: undeadUnits,
        HumanHeroDied: showHeroNotice,
    }
}

// the attacker won a battle at a city: capture it or raze it, and handle the defender
// losing the game (no cities left) or being banished (the city had their fortress).
// returns the gold plundered from the defender.
func (model *GameModel) DefeatCity(attacker *playerlib.Player, defender *playerlib.Player, city *citylib.City, raze bool) int {
    gold := defender.ComputePlunderedGold(city)

    containedFortress := city.Buildings.Contains(buildinglib.BuildingFortress)

    if raze {
        defender.RemoveCity(city)
    } else {
        ChangeCityOwner(city, defender, attacker, ChangeCityRemoveOwnerEnchantments)
    }

    attacker.DidConquerCity(city, raze)
    defender.DidLoseCity(city)

    // player is defeated if they have no cities left
    defeated := len(defender.Cities) == 0

    if defeated {
        defender.Defeated = true
        attacker.DidDefeat(defender)
    }

    if containedFortress {
        defender.Banished = true

        attacker.DidBanish(defender)

        if attacker.IsHuman() || defender.IsHuman() {
            model.Events <- &GameEventShowBanish{Attacker: attacker, Defender: defender}
        }

        if !defeated {
            defender.CastSpellOfReturn()
        }
    }

    return gold
}
