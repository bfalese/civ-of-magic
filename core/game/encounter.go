package game

import (
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/setup"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/combat"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/lib/set"
)

// returns the artifact power table (costs, compatibilities); the frontend loads it
type ArtifactPowersFunc func() (map[artifact.Power]int, map[artifact.Power]set.Set[artifact.ArtifactType], error)

// An encounter (lair, node, tower, ...) entered by a stack. Flow:
//   empty encounter: ClearEncounter, then CreateTreasure
//   guarded: defender, enemies, zone := EncounterDefenders(...); fight the battle;
//            if ResolveEncounter(...) returns true the encounter was cleared: CreateTreasure

// the monsters guarding an encounter, as a neutral defending player with a stack
func (model *GameModel) EncounterDefenders(encounter *maplib.ExtraEncounter, plane data.Plane, x int, y int) (*playerlib.Player, []units.StackUnit, combat.ZoneType) {
    wizard := setup.WizardCustom{
        Name: encounter.Type.ShortName(),
    }

    defender := playerlib.MakePlayer(wizard, false, 1, 1, make(map[herolib.HeroType]string), model)
    defender.StrategicCombat = true

    var enemies []units.StackUnit

    for _, unit := range encounter.Units {
        enemies = append(enemies, units.MakeOverworldUnit(unit, x, y, plane))
    }

    zone := combat.ZoneType{
    }

    switch encounter.Type {
        case maplib.EncounterTypeLair, maplib.EncounterTypeCave: zone.Encounter = combat.ZoneLair
        case maplib.EncounterTypePlaneTower: zone.Encounter = combat.ZoneTower
        case maplib.EncounterTypeAncientTemple: zone.Encounter = combat.ZoneAncientTemple
        case maplib.EncounterTypeFallenTemple: zone.Encounter = combat.ZoneFallenTemple
        case maplib.EncounterTypeRuins: zone.Encounter = combat.ZoneRuins
        case maplib.EncounterTypeAbandonedKeep: zone.Encounter = combat.ZoneAbandonedKeep
        case maplib.EncounterTypeDungeon: zone.Encounter = combat.ZoneDungeon
        case maplib.EncounterTypeNatureNode: zone.Encounter = combat.ZoneNatureNode
        case maplib.EncounterTypeSorceryNode: zone.Encounter = combat.ZoneSorceryNode
        case maplib.EncounterTypeChaosNode: zone.Encounter = combat.ZoneChaosNode
    }

    return defender, enemies, zone
}

// apply the battle at an encounter. returns true if the attacker cleared it (the encounter is
// removed; a plane tower is cleared on both planes), otherwise the dead guards are removed.
// enemies are the units returned by EncounterDefenders.
func (model *GameModel) ResolveEncounter(encounter *maplib.ExtraEncounter, mapUse *maplib.Map, x int, y int, enemies []units.StackUnit, result combat.CombatState) bool {
    if result == combat.CombatStateAttackerWin {
        mapUse.RemoveEncounter(x, y)

        // defeating a plane tower also removes the tower from the other plane
        if encounter.Type == maplib.EncounterTypePlaneTower {
            mapUse.SetPlaneTower(x, y)
            otherMap := model.GetMap(mapUse.Plane.Opposite())
            otherMap.RemoveEncounter(x, y)
            otherMap.SetPlaneTower(x, y)
        }

        return true
    } else {
        var remaining []units.Unit
        for index := range enemies {
            if enemies[index].GetHealth() > 0 {
                remaining = append(remaining, encounter.Units[index])
            }
        }
        encounter.Units = remaining
    }

    return false
}

// roll the treasure for a cleared encounter and send it as GameEventTreasure
func (model *GameModel) CreateTreasure(allSpells spellbook.Spells, getPowers ArtifactPowersFunc, encounterType maplib.EncounterType, budget int, point data.PlanePoint, player *playerlib.Player) {
    var heroes []*herolib.Hero
    for _, hero := range player.HeroPool {
        // only include available heroes that are not champions
        if hero.Status == herolib.StatusAvailable && !hero.IsChampion() {
            heroes = append(heroes, hero)
        }
    }

    makeArtifacts := func () []*artifact.Artifact {
        return model.ArtifactPool.AvailableArtifacts()
    }

    // cannot find the last spell in treasure
    allSpells.RemoveSpell(allSpells.FindByName("Spell of Mastery"))

    treasure := MakeTreasure(getPowers, encounterType, budget, point, player.Wizard, player.KnownSpells, allSpells, heroes, makeArtifacts)
    // FIXME: show treasure ui for human, otherwise just apply treasure for AI
    select {
        case model.Events <- &GameEventTreasure{Treasure: treasure, Player: player}:
        default:
    }
}

// screens for a human player receiving treasure; AI players take the treasure through their AIBehavior
type TreasureHandler interface {
    // the vault screen
    GiveArtifact(player *playerlib.Player, artifact *artifact.Artifact)
    // offer to hire a freed prisoner hero
    OfferPrisonerHero(player *playerlib.Player, hero *herolib.Hero, point data.PlanePoint)
    // the learn spell animation (the spell is learned by the model afterwards)
    ShowLearnedSpell(player *playerlib.Player, spell spellbook.Spell)
}

// give the treasure to the player. allSpells is used to add researchable spells after spellbooks
func (model *GameModel) ApplyTreasure(handlers TreasureHandler, player *playerlib.Player, treasure Treasure, allSpells spellbook.Spells) {
    // apply gold/mana first (#758)
    for _, item := range treasure.Treasures {
        switch item.(type) {
            case *TreasureGold:
                gold := item.(*TreasureGold)
                player.Gold += gold.Amount
            case *TreasureMana:
                mana := item.(*TreasureMana)
                player.Mana += mana.Amount
        }
    }

    // then items/spells
    for _, item := range treasure.Treasures {
        switch item.(type) {
            case *TreasureMagicalItem:
                magicalItem := item.(*TreasureMagicalItem)
                if player.IsHuman() {
                    handlers.GiveArtifact(player, magicalItem.Artifact)
                } else {
                    if player.AIBehavior != nil {
                        // there is a light impedance mismatch between giving a treasure item and invoking the merchant item function
                        // but its ok for now
                        player.AIBehavior.HandleMerchantItem(player, magicalItem.Artifact, 0)
                    }
                }
                // if the treasure was one of the premade artifacts, then remove it from the pool
                model.ArtifactPool.Award(magicalItem.Artifact)
            case *TreasurePrisonerHero:
                hero := item.(*TreasurePrisonerHero)
                if player.IsHuman() {
                    handlers.OfferPrisonerHero(player, hero.Hero, treasure.Point)
                } else {
                    if player.AIBehavior != nil {
                        player.AIBehavior.HandleHireHero(player, hero.Hero, 0, false, treasure.Point)
                    }
                }
            case *TreasureSpell:
                spell := item.(*TreasureSpell)
                // add the spell to the research pool in case so it shows up in the spell book
                player.ResearchPoolSpells.AddSpell(spell.Spell)
                if player.IsHuman() {
                    handlers.ShowLearnedSpell(player, spell.Spell)
                }
                player.LearnSpell(spell.Spell)
            case *TreasureSpellbook:
                spellbook := item.(*TreasureSpellbook)
                player.Wizard.AddMagicLevel(spellbook.Magic, spellbook.Count)
                researchable := allSpells.Copy()
                // add more researchable spells based on the new magic level
                player.InitializeResearchableSpells(&researchable)
            case *TreasureRetort:
                retort := item.(*TreasureRetort)
                player.Wizard.EnableRetort(retort.Retort)
        }
    }
}
