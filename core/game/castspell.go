package game

import (
    "errors"
    "fmt"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"

    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// Casting a finished overland spell without a frontend: the target comes as data. Used by the
// headless game and by commands. The Ebitengine UI has its own flow (cast.go) for target picking
// and animations, and shares the rules (castrules.go and the model's Apply* functions).

var ErrSpellNeedsTarget = errors.New("the spell needs a target")

// the spell isn't castable without the Ebitengine UI (Spell of Mastery's victory screens, Create
// Artifact without an artifact designed beforehand)
var ErrSpellNotSupported = errors.New("the spell can't be cast without the UI yet")

type SpellTarget struct {
    // a unit, for spells on a single unit
    Unit units.StackUnit
    // a tile, for spells on a city, a stack or the map
    Location *Location
    // a wizard, for spells on a wizard (and the owner of the enchantment for Spell Binding,
    // Disjunction)
    Wizard *playerlib.Player
    // a choice by name: the ward for Spell Ward, the global enchantment for Spell Binding and
    // Disjunction, the hero for Resurrection
    Choice string
}

// what kind of target a spell needs
func (model *GameModel) SpellTargetType(player *playerlib.Player, spell spellbook.Spell) (LocationType, bool) {
    if _, ok := model.UnitSpellRule(spell); ok {
        return LocationTypeFriendlyUnit, true
    }
    if rule, ok := model.CitySpellRule(player, spell); ok {
        return rule.LocationType, true
    }
    switch spell.Name {
        case "Black Wind", "Stasis", "Ice Storm", "Fire Storm": return LocationTypeEnemyUnit, true
        case "Nature's Cures", "Plane Shift": return LocationTypeFriendlyUnit, true
        case "Spell of Return", "Spell Ward", "Wall of Stone", "Summoning Circle", "Move Fortress": return LocationTypeFriendlyCity, true
        case "Earthquake", "Call the Void": return LocationTypeEnemyCity, true
        case "Earth Lore", "Enchant Road": return LocationTypeAny, true
        case "Change Terrain": return LocationTypeChangeTerrain, true
        case "Transmute": return LocationTypeTransmute, true
        case "Raise Volcano": return LocationTypeRaiseVolcano, true
        case "Corruption": return LocationTypeLand, true
        case "Warp Node": return LocationTypeEnemyMeldedNode, true
        case "Disenchant Area", "Disenchant True": return LocationTypeDisenchant, true
        case "Floating Island": return LocationTypeEmptyWater, true
        case "Spell Blast", "Cruel Unminding", "Drain Power", "Subversion": return LocationTypeWizard, true
        case "Spell Binding", "Disjunction", "Disjunction True": return LocationTypeEnemyGlobalEnchantment, true
        case "Resurrection": return LocationTypeDeadHero, true
    }
    return LocationTypeAny, false
}

// cast a spell whose casting just finished. Returns nil when the spell took effect (or fizzled,
// which is announced as an event), ErrSpellNeedsTarget / an error describing a bad target, or
// ErrSpellNotSupported.
func (model *GameModel) CastSpell(player *playerlib.Player, spell spellbook.Spell, target SpellTarget) error {
    fizzled, _ := model.CheckInstantFizzle(player, spell)
    if fizzled {
        model.ShowFizzleSpell(spell, player)
        return nil
    }

    if spell.IsOfRealm(data.ChaosMagic) || spell.IsOfRealm(data.DeathMagic) {
        model.MaybeDoNaturesWrath(player)
    }

    if rule, ok := model.UnitSpellRule(spell); ok {
        if target.Unit == nil {
            return ErrSpellNeedsTarget
        }
        if player.FindStackByUnit(target.Unit) == nil {
            return errors.New("not one of your units")
        }
        if err := rule.Check(target.Unit); err != nil {
            return err
        }
        rule.Apply(target.Unit)
        model.RefreshUI()
        return nil
    }

    if rule, ok := model.CitySpellRule(player, spell); ok {
        if target.Location == nil {
            return ErrSpellNeedsTarget
        }
        city, owner := model.FindCity(target.Location.X, target.Location.Y, target.Location.Plane)
        if city == nil {
            return errors.New("no city there")
        }
        if rule.LocationType == LocationTypeFriendlyCity && owner != player {
            return errors.New("not one of your cities")
        }
        if rule.LocationType == LocationTypeEnemyCity {
            if owner == player {
                return errors.New("not an enemy city")
            }
            if !city.CanTarget(spell) {
                return fmt.Errorf("You cannot cast %v on this city", spell.Name)
            }
        }
        if city.HasEnchantment(rule.Enchantment) {
            return fmt.Errorf("This city already has a %v cast on it", spell.Name)
        }
        if rule.Before(city) {
            model.AddCityEnchantment(city, player, rule.Enchantment)
            rule.After(city)
            model.RefreshUI()
        }
        return nil
    }

    if spell.Section == spellbook.SectionSummoning {
        if unit := units.GetUnitByName(spell.Name); !unit.IsNone() {
            model.SummonUnit(player, unit)
            model.RefreshUI()
            return nil
        }
    }

    switch spell.Name {
        case "Summon Hero", "Summon Champion":
            hero := model.ChooseSummonHero(player, spell.Name == "Summon Champion")
            if hero != nil {
                model.offerHero(player, hero)
            }
            return nil
        case "Incarnation":
            for _, hero := range player.HeroPool {
                if hero.HeroType == herolib.HeroTorin && hero.Status != herolib.StatusEmployed {
                    model.offerHero(player, hero)
                }
            }
            return nil
        case "Death Wish":
            model.ApplyDeathWish(player, spell)
            return nil
        case "Great Unsummoning":
            model.ApplyGreatUnsummoning(spell)
            model.RefreshUI()
            return nil
    }

    if enchantment := data.GetEnchantmentByName(spell.Name); enchantment != data.EnchantmentNone {
        if !player.GlobalEnchantments.Contains(enchantment) {
            player.GlobalEnchantments.Insert(enchantment)
            model.ApplyGlobalEnchantment(enchantment, player)
            model.RefreshUI()
        }
        return nil
    }

    locationType, needsTarget := model.SpellTargetType(player, spell)
    if needsTarget {
        switch locationType {
            case LocationTypeWizard:
                return model.castOnWizard(player, spell, target.Wizard)
            case LocationTypeEnemyGlobalEnchantment:
                return model.castOnGlobalEnchantment(player, spell, target.Wizard, target.Choice)
            case LocationTypeDeadHero:
                return model.castResurrection(player, target.Choice)
        }

        if target.Location == nil {
            return ErrSpellNeedsTarget
        }
        return model.castOnTile(player, spell, *target.Location, target.Choice)
    }

    switch spell.Name {
        case "Create Artifact", "Enchant Item":
            if player.CreateArtifact == nil {
                return ErrSpellNotSupported
            }
            select {
                case model.Events <- &GameEventVault{CreatedArtifact: player.CreateArtifact, Player: player}:
                default:
            }
            player.CreateArtifact = nil
            return nil
    }

    return ErrSpellNotSupported
}

// a hero for hire at no cost (summoned heroes)
func (model *GameModel) offerHero(player *playerlib.Player, hero *herolib.Hero) {
    model.SendOffer(&GameEventHireHero{Hero: hero, Player: player, Cost: 0})
}

func (model *GameModel) castOnTile(player *playerlib.Player, spell spellbook.Spell, location Location, choice string) error {
    x, y, plane := location.X, location.Y, location.Plane
    city, cityOwner := model.FindCity(x, y, plane)

    // map tile checks (terrain types, explored, ...) as for the UI's target picking
    if locationType, _ := model.SpellTargetType(player, spell); locationType != LocationTypeFriendlyUnit && locationType != LocationTypeEnemyUnit && locationType != LocationTypeFriendlyCity && locationType != LocationTypeEnemyCity {
        tileX, ok, message := model.CheckSpellTarget(player, spell, locationType, model.ComputeCityStackInfo(), plane, x, y)
        if !ok {
            if message == "" {
                message = fmt.Sprintf("%v can't be cast there", spell.Name)
            }
            return errors.New(message)
        }
        x = tileX
    }

    ownCity := func() (*citylib.City, error) {
        own := player.FindCity(x, y, plane)
        if own == nil {
            return nil, errors.New("no city of yours there")
        }
        return own, nil
    }

    // the city at the tile may resist spells on the stack in it
    cityResists := func(dispel bool) bool {
        if city == nil {
            return false
        }
        if !city.CanTarget(spell) || (dispel && city.CheckDispel(spell)) {
            model.ShowFizzleSpell(spell, player)
            return true
        }
        return false
    }

    enemyStack := func() (*playerlib.UnitStack, *playerlib.Player, error) {
        stack, owner := model.FindStack(x, y, plane)
        if stack == nil || owner == player {
            return nil, nil, errors.New("no enemy stack there")
        }
        return stack, owner, nil
    }

    switch spell.Name {
        case "Black Wind":
            stack, owner, err := enemyStack()
            if err != nil {
                return err
            }
            if city != nil && !city.CanTarget(spell) {
                model.ShowFizzleSpell(spell, player)
                return nil
            }
            model.ApplyBlackWind(stack, owner)
        case "Stasis":
            stack, _, err := enemyStack()
            if err != nil {
                return err
            }
            if cityResists(true) {
                return nil
            }
            model.ApplyStasis(stack)
        case "Ice Storm", "Fire Storm":
            stack, owner, err := enemyStack()
            if err != nil {
                return err
            }
            if city != nil && city.CheckDispel(spell) {
                model.ShowFizzleSpell(spell, player)
                return nil
            }
            if spell.Name == "Ice Storm" {
                model.ApplyIceStorm(stack, owner)
            } else {
                model.ApplyFireStorm(stack, owner)
            }
        case "Nature's Cures":
            stack := player.FindStack(x, y, plane)
            if stack == nil {
                return errors.New("no stack of yours there")
            }
            stack.NaturalHeal(1)
        case "Plane Shift":
            stack := player.FindStack(x, y, plane)
            if stack == nil {
                return errors.New("no stack of yours there")
            }
            if err := model.PlaneShift(stack, player); err != nil {
                return err
            }
            model.AnnounceVisibleWizards()
        case "Spell of Return":
            ownCity := player.FindCity(x, y, plane)
            if ownCity == nil {
                return errors.New("no city of yours there")
            }
            model.ApplySpellOfReturn(player, ownCity)
        case "Earthquake":
            if city == nil || cityOwner == player {
                return errors.New("no enemy city there")
            }
            model.DoEarthquake(city, cityOwner)
        case "Call the Void":
            if city == nil || cityOwner == player {
                return errors.New("no enemy city there")
            }
            if city.CheckDispel(spell) {
                model.ShowFizzleSpell(spell, player)
                return nil
            }
            model.DoCallTheVoid(city, cityOwner)
        case "Spell Ward":
            target, err := ownCity()
            if err != nil {
                return err
            }
            for _, ward := range model.AvailableSpellWards(target).Values() {
                if ward.Name() == choice {
                    model.AddCityEnchantment(target, player, ward)
                    model.RefreshUI()
                    return nil
                }
            }
            return fmt.Errorf("%q is not a ward available for %v", choice, target.Name)
        case "Wall of Stone", "Summoning Circle", "Move Fortress":
            target, err := ownCity()
            if err != nil {
                return err
            }
            newBuilding, message := buildinglib.BuildingCityWalls, "This city already has a Wall of Stone"
            switch spell.Name {
                case "Summoning Circle": newBuilding, message = buildinglib.BuildingSummoningCircle, "Your summoning circle is already in this city"
                case "Move Fortress": newBuilding, message = buildinglib.BuildingFortress, "Your fortress is already in this city"
            }
            if target.Buildings.Contains(newBuilding) {
                return errors.New(message)
            }
            model.AddCityBuilding(target, newBuilding)
            switch spell.Name {
                case "Wall of Stone":
                    if target.ProducingBuilding == buildinglib.BuildingCityWalls {
                        target.ProducingBuilding = buildinglib.BuildingTradeGoods
                    }
                case "Summoning Circle": model.ApplySummoningCircleMoved(player, target)
                case "Move Fortress": model.ApplyFortressMoved(player, target)
            }
        case "Earth Lore":
            player.LiftFogSquare(x, y, 5, plane)
            model.AnnounceVisibleWizards()
        case "Enchant Road":
            model.ApplyEnchantRoad(plane, x, y)
        case "Change Terrain":
            model.ApplyChangeTerrain(plane, x, y)
        case "Transmute":
            model.ApplyTransmute(plane, x, y)
        case "Raise Volcano":
            if city != nil && city.CheckDispel(spell) {
                model.ShowFizzleSpell(spell, player)
                return nil
            }
            model.ApplyRaiseVolcanoTerrain(plane, x, y, x, y)
            model.ApplyRaiseVolcano(plane, x, y, player)
        case "Corruption":
            if city != nil && city.CheckDispel(spell) {
                model.ShowFizzleSpell(spell, player)
                return nil
            }
            model.ApplyCorruption(plane, x, y)
        case "Warp Node":
            model.ApplyWarpNode(plane, x, y, player)
        case "Disenchant Area", "Disenchant True":
            model.ApplyDisenchantArea(plane, player, spell, spell.Name == "Disenchant True", x, y)
        case "Floating Island":
            model.ApplyFloatingIsland(plane, player, x, y)
            model.AnnounceVisibleWizards()
    }

    model.RefreshUI()
    return nil
}

// the castSpell hook (see StartPlayerTurn) for frontends that use commands: a spell that needs a
// target is kept as pending and announced with GameEventChooseSpellTarget; others are cast now
func (model *GameModel) FinishCast(player *playerlib.Player, spell spellbook.Spell) {
    if locationType, ok := model.SpellTargetType(player, spell); ok {
        if model.PendingSpells == nil {
            model.PendingSpells = make(map[*playerlib.Player]spellbook.Spell)
        }
        model.PendingSpells[player] = spell
        select {
            case model.Events <- &GameEventChooseSpellTarget{Player: player, Spell: spell, LocationType: locationType}:
            default:
        }
        return
    }

    if err := model.CastSpell(player, spell, SpellTarget{}); err != nil {
        select {
            case model.Events <- &GameEventNotice{Message: fmt.Sprintf("%v: %v", spell.Name, err)}:
            default:
        }
    }
}

// spells on another wizard: Spell Blast, Cruel Unminding, Drain Power, Subversion
func (model *GameModel) castOnWizard(player *playerlib.Player, spell spellbook.Spell, wizard *playerlib.Player) error {
    if wizard == nil {
        return ErrSpellNeedsTarget
    }
    if wizard == player || wizard.IsNeutral() || wizard.Defeated || wizard.Banished || !player.IsAwareOf(wizard) {
        return fmt.Errorf("%v can't be targeted", wizard.Wizard.Name)
    }

    ok := true
    switch spell.Name {
        case "Spell Blast": ok = model.ApplySpellBlast(player, wizard)
        case "Cruel Unminding": _, ok = model.ApplyCruelUnminding(wizard)
        case "Drain Power": _, ok = model.ApplyDrainPower(wizard)
        case "Subversion": ok = model.ApplySubversion(player, wizard)
    }
    if !ok {
        return fmt.Errorf("%v has no effect on %v", spell.Name, wizard.Wizard.Name)
    }
    model.RefreshUI()
    return nil
}

// Spell Binding (steal) and Disjunction (remove) on another wizard's global enchantment.
// A failed dispel roll is not an error: the spell was cast, it just didn't work.
func (model *GameModel) castOnGlobalEnchantment(player *playerlib.Player, spell spellbook.Spell, owner *playerlib.Player, choice string) error {
    if owner == nil || owner == player {
        return ErrSpellNeedsTarget
    }

    enchantment := data.GetEnchantmentByName(choice)
    if enchantment == data.EnchantmentNone || !owner.GlobalEnchantments.Contains(enchantment) {
        return fmt.Errorf("%v has no %q", owner.Wizard.Name, choice)
    }
    targetSpell := model.AllSpells.FindByName(enchantment.String())

    if spell.Name == "Spell Binding" {
        if player.GlobalEnchantments.Contains(enchantment) {
            return fmt.Errorf("you already have %v", choice)
        }
        if model.TrySpellBinding(player, owner, enchantment, targetSpell, 20000) {
            model.ApplyGlobalEnchantment(enchantment, player)
        }
    } else {
        dispelStrength := spell.Cost(true)
        if spell.Name == "Disjunction True" {
            dispelStrength *= 3
        }
        if player.Wizard.RetortEnabled(data.RetortRunemaster) {
            dispelStrength *= 2
        }
        model.TryDisjunction(owner, enchantment, targetSpell, dispelStrength)
    }

    model.RefreshUI()
    return nil
}

func (model *GameModel) castResurrection(player *playerlib.Player, heroName string) error {
    if player.FreeHeroSlots() == 0 {
        return errors.New("No free hero slots to resurrect hero")
    }
    for _, hero := range player.GetDeadHeroes() {
        if hero.GetName() == heroName || hero.Name == heroName {
            model.ResurrectHero(player, hero)
            model.RefreshUI()
            return nil
        }
    }
    return fmt.Errorf("no dead hero named %q", heroName)
}
