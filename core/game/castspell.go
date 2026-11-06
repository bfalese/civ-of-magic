package game

import (
    "errors"
    "fmt"

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

// the spell isn't castable without the Ebitengine UI yet (its choices are still made in screens:
// Spell Ward, Subversion, Spell Binding, Disjunction, Resurrection, Create Artifact, map spells...)
var ErrSpellNotSupported = errors.New("the spell can't be cast without the UI yet")

type SpellTarget struct {
    // a unit, for spells on a single unit
    Unit units.StackUnit
    // a tile, for spells on a city or a stack
    Location *Location
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
        case "Spell of Return": return LocationTypeFriendlyCity, true
        case "Earthquake": return LocationTypeEnemyCity, true
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

    // spells on a stack or a city at a tile
    switch spell.Name {
        case "Black Wind", "Stasis", "Ice Storm", "Fire Storm", "Nature's Cures", "Plane Shift", "Spell of Return", "Earthquake":
            if target.Location == nil {
                return ErrSpellNeedsTarget
            }
            return model.castOnTile(player, spell, *target.Location)
    }

    return ErrSpellNotSupported
}

// a hero for hire at no cost (summoned heroes)
func (model *GameModel) offerHero(player *playerlib.Player, hero *herolib.Hero) {
    select {
        case model.Events <- &GameEventHireHero{Hero: hero, Player: player, Cost: 0}:
        default:
    }
}

func (model *GameModel) castOnTile(player *playerlib.Player, spell spellbook.Spell, location Location) error {
    x, y, plane := location.X, location.Y, location.Plane
    city, cityOwner := model.FindCity(x, y, plane)

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
    }

    model.RefreshUI()
    return nil
}
