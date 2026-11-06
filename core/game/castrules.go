package game

import (
    "errors"
    "fmt"
    "math/rand/v2"

    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// The rules of overland spells that target a unit or a city: which targets are allowed and what
// the spell does. A frontend picks the target (and shows the animation); the model checks it and
// applies the effect. Spells are matched by their name in spelldat.lbx (including its typos, such
// as "Invisiblity").

// spells that put one enchantment on a friendly unit
var unitEnchantmentSpells = map[string]data.UnitEnchantment{
    "Bless": data.UnitEnchantmentBless,
    "Heroism": data.UnitEnchantmentHeroism,
    "Giant Strength": data.UnitEnchantmentGiantStrength,
    "Lionheart": data.UnitEnchantmentLionHeart,
    "Immolation": data.UnitEnchantmentImmolation,
    "Resist Elements": data.UnitEnchantmentResistElements,
    "Resist Magic": data.UnitEnchantmentResistMagic,
    "Elemental Armor": data.UnitEnchantmentElementalArmor,
    "Righteousness": data.UnitEnchantmentRighteousness,
    "Cloak of Fear": data.UnitEnchantmentCloakOfFear,
    "True Sight": data.UnitEnchantmentTrueSight,
    "Flight": data.UnitEnchantmentFlight,
    "Endurance": data.UnitEnchantmentEndurance,
    "Holy Armor": data.UnitEnchantmentHolyArmor,
    "Holy Weapon": data.UnitEnchantmentHolyWeapon,
    "Invulnerability": data.UnitEnchantmentInvulnerability,
    "Planar Travel": data.UnitEnchantmentPlanarTravel,
    "Iron Skin": data.UnitEnchantmentIronSkin,
    "Path Finding": data.UnitEnchantmentPathFinding,
    "Regeneration": data.UnitEnchantmentRegeneration,
    "Stone Skin": data.UnitEnchantmentStoneSkin,
    "Water Walking": data.UnitEnchantmentWaterWalking,
    "Guardian Wind": data.UnitEnchantmentGuardianWind,
    "Invisiblity": data.UnitEnchantmentInvisibility,
    "Magic Immunity": data.UnitEnchantmentMagicImmunity,
    "Spell Lock": data.UnitEnchantmentSpellLock,
    "Wind Walking": data.UnitEnchantmentWindWalking,
    "Eldritch Weapon": data.UnitEnchantmentEldritchWeapon,
    "Flame Blade": data.UnitEnchantmentFlameBlade,
    "Wraith Form": data.UnitEnchantmentWraithForm,
}

// spells that put an enchantment on a friendly city
var cityEnchantmentSpells = map[string]data.CityEnchantment{
    "Flying Fortress": data.CityEnchantmentFlyingFortress,
    "Earth Gate": data.CityEnchantmentEarthGate,
    "Astral Gate": data.CityEnchantmentAstralGate,
    "Heavenly Light": data.CityEnchantmentHeavenlyLight,
    "Cloud of Shadow": data.CityEnchantmentCloudOfShadow,
    "Prosperity": data.CityEnchantmentProsperity,
    "Inspirations": data.CityEnchantmentInspirations,
    "Gaia's Blessing": data.CityEnchantmentGaiasBlessing,
    "Dark Rituals": data.CityEnchantmentDarkRituals,
    "Stream of Life": data.CityEnchantmentStreamOfLife,
    "Altar of Battle": data.CityEnchantmentAltarOfBattle,
    "Wall of Fire": data.CityEnchantmentWallOfFire,
    "Wall of Darkness": data.CityEnchantmentWallOfDarkness,
    "Consecration": data.CityEnchantmentConsecration,
    "Nature's Eye": data.CityEnchantmentNaturesEye,
}

// curses on an enemy city; the city may dispel them
var cityCurseSpells = map[string]data.CityEnchantment{
    "Chaos Rift": data.CityEnchantmentChaosRift,
    "Cursed Lands": data.CityEnchantmentCursedLands,
    "Famine": data.CityEnchantmentFamine,
    "Pestilence": data.CityEnchantmentPestilence,
    "Evil Presence": data.CityEnchantmentEvilPresence,
}

// how a spell affects a unit picked by the caster
type UnitSpellRule struct {
    // the cast animation to show on the unit's tile
    AnimationIndex int
    // nil if the unit can be targeted, otherwise the reason to show the player
    Check func(unit units.StackUnit) error
    Apply func(unit units.StackUnit)
}

// the rule for a spell cast on one of the caster's units; false if the spell isn't such a spell
func (model *GameModel) UnitSpellRule(spell spellbook.Spell) (UnitSpellRule, bool) {
    addEnchantment := func(enchantment data.UnitEnchantment, extra func(unit units.StackUnit)) UnitSpellRule {
        return UnitSpellRule{
            AnimationIndex: enchantment.CastAnimationIndex(),
            Check: func(unit units.StackUnit) error {
                if unit.HasEnchantment(enchantment) {
                    return fmt.Errorf("That unit already has %v cast on it", spell.Name)
                }
                return nil
            },
            Apply: func(unit units.StackUnit) {
                unit.AddEnchantment(enchantment)
                if extra != nil {
                    extra(unit)
                }
            },
        }
    }

    if enchantment, ok := unitEnchantmentSpells[spell.Name]; ok {
        return addEnchantment(enchantment, nil), true
    }

    switch spell.Name {
        case "Black Channels":
            return addEnchantment(data.UnitEnchantmentBlackChannels, func(unit units.StackUnit) {
                unit.SetUndead()
            }), true

        case "Chaos Channels":
            choices := []data.UnitEnchantment{
                data.UnitEnchantmentChaosChannelsDemonSkin,
                data.UnitEnchantmentChaosChannelsDemonWings,
                data.UnitEnchantmentChaosChannelsFireBreath,
            }
            choice := choices[rand.N(len(choices))]
            return UnitSpellRule{
                AnimationIndex: choice.CastAnimationIndex(),
                Check: func(unit units.StackUnit) error {
                    if unit.GetRace() == data.RaceFantastic {
                        return errors.New("That unit cannot be targeted")
                    }
                    for _, enchantment := range choices {
                        if unit.HasEnchantment(enchantment) {
                            return errors.New("That unit cannot be targeted")
                        }
                    }
                    return nil
                },
                Apply: func(unit units.StackUnit) {
                    unit.AddEnchantment(choice)
                },
            }, true

        case "Lycanthropy":
            return UnitSpellRule{
                AnimationIndex: 4,
                Check: func(unit units.StackUnit) error {
                    if unit.GetRace() == data.RaceFantastic {
                        return errors.New("Summoned units may not have cast Lycanthropy on them")
                    }
                    if unit.GetRace() == data.RaceHero {
                        return errors.New("Heroes units may not have cast Lycanthropy on them")
                    }
                    if unit.IsUndead() || unit.HasEnchantment(data.UnitEnchantmentBlackChannels) || unit.HasEnchantment(data.UnitEnchantmentChaosChannelsDemonSkin) || unit.HasEnchantment(data.UnitEnchantmentChaosChannelsDemonWings) || unit.HasEnchantment(data.UnitEnchantmentChaosChannelsFireBreath) {
                        return errors.New("Chaos channeled and Undead units may not have cast Lycanthropy on them")
                    }
                    return nil
                },
                Apply: func(unit units.StackUnit) {
                    model.ApplyLycanthropy(unit)
                },
            }, true

        case "Word of Recall":
            return UnitSpellRule{
                AnimationIndex: 1,
                Check: func(unit units.StackUnit) error {
                    owner := model.GetUnitOwner(unit)
                    if owner == nil {
                        return errors.New("That unit cannot be targeted")
                    }
                    summonCity := owner.FindSummoningCity()
                    if summonCity == nil {
                        return errors.New("You have no summoning circle to recall the unit to")
                    }
                    if unit.GetX() == summonCity.X && unit.GetY() == summonCity.Y {
                        return errors.New("Your unit is already in this city")
                    }
                    if unit.GetPlane() != summonCity.Plane && model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
                        return errors.New("Your unit cannot planar travel to this location")
                    }
                    return nil
                },
                Apply: func(unit units.StackUnit) {
                    model.RelocateUnit(model.GetUnitOwner(unit), unit)
                },
            }, true
    }

    return UnitSpellRule{}, false
}

// how a spell affects a city picked by the caster
type CitySpellRule struct {
    // friendly or enemy city
    LocationType LocationType
    Enchantment data.CityEnchantment
    // false stops the cast after the target was accepted (e.g. the city dispelled a curse; the
    // fizzle is announced as an event)
    Before func(city *citylib.City) bool
    // after the enchantment was added
    After func(city *citylib.City)
}

// the rule for a spell that enchants or curses a city; false if the spell isn't such a spell
func (model *GameModel) CitySpellRule(player *playerlib.Player, spell spellbook.Spell) (CitySpellRule, bool) {
    if enchantment, ok := cityEnchantmentSpells[spell.Name]; ok {
        rule := CitySpellRule{
            LocationType: LocationTypeFriendlyCity,
            Enchantment: enchantment,
            Before: func(city *citylib.City) bool { return true },
            After: func(city *citylib.City) {},
        }

        switch spell.Name {
            case "Consecration":
                rule.After = func(city *citylib.City) {
                    model.ApplyConsecration(city)
                }
            case "Nature's Eye":
                rule.After = func(city *citylib.City) {
                    player.UpdateFogVisibility()
                    model.AnnounceVisibleWizards()
                }
        }

        return rule, true
    }

    if enchantment, ok := cityCurseSpells[spell.Name]; ok {
        return CitySpellRule{
            LocationType: LocationTypeEnemyCity,
            Enchantment: enchantment,
            Before: func(city *citylib.City) bool {
                if city.CheckDispel(spell) {
                    model.ShowFizzleSpell(spell, player)
                    return false
                }
                return true
            },
            After: func(city *citylib.City) {},
        }, true
    }

    return CitySpellRule{}, false
}
