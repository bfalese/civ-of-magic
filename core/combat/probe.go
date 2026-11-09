package combat

import (
    "slices"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// Whether a combat spell has anything to be cast on, without casting it: InvokeSpell runs in
// probe mode, where DoTargetUnitSpell and DoTargetTileSpell only check the targets their filters
// accept and return. Only the spells in probedSpells are probed: their InvokeSpell case does
// nothing but ask for a target (TestProbedSpellsList checks the list against InvokeSpell), so a
// probe has no effect on the battle.

// the spells whose only step is picking a unit or a tile (summons pick a tile on your side)
var probedSpells = map[string]bool{
    "Fireball": true, "Ice Bolt": true, "Star Fires": true, "Psionic Blast": true, "Doom Bolt": true,
    "Fire Bolt": true, "Lightning Bolt": true, "Warp Lightning": true, "Life Drain": true,
    "Dispel Evil": true, "Healing": true, "Recall Hero": true, "Cracks Call": true,
    "Earth to Mud": true, "Web": true, "Banish": true, "Dispel Magic True": true, "Dispel Magic": true,
    "Word of Recall": true, "Disintegrate": true, "Disrupt": true, "Magic Vortex": true,
    "Warp Wood": true, "Word of Death": true, "Phantom Warriors": true, "Phantom Beast": true,
    "Summon Demons": true, "Earth Elemental": true, "Air Elemental": true, "Fire Elemental": true,
    "Creature Binding": true, "Mind Storm": true, "Bless": true, "Weakness": true,
    "Black Sleep": true, "Vertigo": true, "Shatter": true, "Warp Creature": true, "Confusion": true,
    "Possession": true, "Petrify": true, "Heroism": true, "Holy Armor": true, "Holy Weapon": true,
    "Invulnerability": true, "Lionheart": true, "Righteousness": true, "True Sight": true,
    "Elemental Armor": true, "Giant Strength": true, "Iron Skin": true, "Regeneration": true,
    "Resist Elements": true, "Stone Skin": true, "Flight": true, "Guardian Wind": true, "Haste": true,
    "Invisiblity": true, "Magic Immunity": true, "Resist Magic": true, "Spell Lock": true,
    "Eldritch Weapon": true, "Flame Blade": true, "Immolation": true, "Berserk": true,
    "Cloak of Fear": true, "Wraith Form": true,
}

// the area spells: their case only applies DoAllUnitsSpell, probed the same way (anyone it would
// touch?)
var areaSpells = map[string]bool{"Flame Strike": true, "Holy Word": true, "Death Spell": true}

// the combat enchantments (their case only calls CastEnchantment): pointless once in effect
var enchantmentSpells = map[string]data.CombatEnchantment{
    "High Prayer": data.CombatEnchantmentHighPrayer, "Prayer": data.CombatEnchantmentPrayer,
    "True Light": data.CombatEnchantmentTrueLight, "Call Lightning": data.CombatEnchantmentCallLightning,
    "Entangle": data.CombatEnchantmentEntangle, "Blur": data.CombatEnchantmentBlur,
    "Counter Magic": data.CombatEnchantmentCounterMagic, "Mass Invisibility": data.CombatEnchantmentMassInvisibility,
    "Metal Fires": data.CombatEnchantmentMetalFires, "Warp Reality": data.CombatEnchantmentWarpReality,
    "Black Prayer": data.CombatEnchantmentBlackPrayer, "Darkness": data.CombatEnchantmentDarkness,
    "Mana Leak": data.CombatEnchantmentManaLeak, "Terror": data.CombatEnchantmentTerror,
    "Wrack": data.CombatEnchantmentWrack,
}

// why casting the spell now would do nothing, or "" when it may work: no target (Disrupt
// without walls), nobody an area spell would touch, nobody wounded for Mass Healing, a combat
// enchantment or wall already in effect, no dead to raise, nothing Disenchant Area could dispel,
// no enemy for Call Chaos, no free tile for Summon Demon
func (model *CombatModel) SpellCanWork(spellSystem SpellSystem, army *Army, unitCaster *ArmyUnit, spell spellbook.Spell) (bool, string) {
    if enchantment, ok := enchantmentSpells[spell.Name]; ok {
        if army.HasEnchantment(enchantment) || slices.Contains(model.GlobalEnchantments, enchantment) {
            return false, "already in effect"
        }
        return true, ""
    }
    switch spell.Name {
        case "Mass Healing":
            for _, unit := range army.units {
                if unit.GetHealth() > 0 && unit.GetHealth() < unit.GetMaxHealth() && unit.GetRealm() != data.DeathMagic {
                    return true, ""
                }
            }
            return false, "nobody is wounded"
        case "Wall of Fire":
            if model.AnyTile(func(tile *Tile) bool { return tile.Fire != nil }) {
                return false, "the wall of fire is already up"
            }
            return true, ""
        case "Disenchant Area", "Disenchant True":
            if !model.hasSomethingToDisenchant(army) {
                return false, "nothing to dispel here"
            }
            return true, ""
        case "Call Chaos":
            for _, unit := range model.GetOppositeArmyForPlayer(army.Player).units {
                if unit.GetHealth() > 0 {
                    return true, ""
                }
            }
            return false, "nothing it would affect here"
        case "Summon Demon":
            // the demon appears on any free tile near the middle (FindEmptyTile)
            if !model.AnyTileAt(func(x int, y int) bool { return model.GetUnit(x, y) == nil }) {
                return false, "no room to summon"
            }
            return true, ""
        case "Wall of Darkness":
            if model.AnyTile(func(tile *Tile) bool { return tile.Darkness != nil }) {
                return false, "the wall of darkness is already up"
            }
            return true, ""
    }
    if areaSpells[spell.Name] {
        if hasTarget, needsTarget := model.probeSpell(spellSystem, army, unitCaster, spell); needsTarget && !hasTarget {
            return false, "nothing it would affect here"
        }
        return true, ""
    }
    if hasTarget, needsTarget := model.SpellHasTarget(spellSystem, army, unitCaster, spell); needsTarget && !hasTarget {
        if spell.Name == "Raise Dead" || spell.Name == "Animate Dead" {
            return false, "no dead to raise"
        }
        return false, "nothing to cast it on here"
    }
    return true, ""
}

// what a probe found
type spellProbe struct {
    asked bool
    found bool
}

// whether the spell has a target now, for army (and unitCaster when a unit casts it). needsTarget
// is false for spells that don't pick a target (or that this check doesn't know): those are
// always fine to try.
func (model *CombatModel) SpellHasTarget(spellSystem SpellSystem, army *Army, unitCaster *ArmyUnit, spell spellbook.Spell) (hasTarget bool, needsTarget bool) {
    switch spell.Name {
        case "Raise Dead":
            // a dead unit of the army's that isn't fantastic
            for _, killed := range army.KilledUnits {
                if killed.GetRace() != data.RaceFantastic {
                    return model.hasEmptyTileOnSide(army), true
                }
            }
            return false, true
        case "Animate Dead":
            for _, killed := range append(append([]*ArmyUnit{}, model.AttackingArmy.KilledUnits...), model.DefendingArmy.KilledUnits...) {
                if !killed.Unit.IsHero() && killed.GetRealm() != data.DeathMagic && !killed.Summoned {
                    return true, true
                }
            }
            return false, true
    }
    if !probedSpells[spell.Name] {
        return false, false
    }
    return model.probeSpell(spellSystem, army, unitCaster, spell)
}

// run InvokeSpell in probe mode (only for spells whose case does nothing but pick targets)
func (model *CombatModel) probeSpell(spellSystem SpellSystem, army *Army, unitCaster *ArmyUnit, spell spellbook.Spell) (hasTarget bool, needsTarget bool) {
    probe := &spellProbe{}
    model.probe = probe
    defer func() { model.probe = nil }()
    model.InvokeSpell(spellSystem, army, unitCaster, spell, func(bool) {})
    if !probe.asked {
        // the case changed and no longer asks for a target: let the player try
        return false, false
    }
    return probe.found, true
}

// Disenchant Area / True: an enchantment on the other army or its living units, or a curse on one
// of the army's own living units (DoDisenchantArea's targets)
func (model *CombatModel) hasSomethingToDisenchant(army *Army) bool {
    enemy := model.GetOppositeArmyForPlayer(army.Player)
    if len(enemy.Enchantments) > 0 {
        return true
    }
    for _, unit := range enemy.units {
        if unit.GetHealth() > 0 && (len(unit.Enchantments) > 0 || len(unit.Unit.GetEnchantments()) > 0) {
            return true
        }
    }
    for _, unit := range army.units {
        if unit.GetHealth() > 0 && len(unit.GetCurses()) > 0 {
            return true
        }
    }
    return false
}

// whether any battlefield tile passes the check
func (model *CombatModel) AnyTileAt(check func(x int, y int) bool) bool {
    for y, row := range model.Tiles {
        for x := range row {
            if check(x, y) {
                return true
            }
        }
    }
    return false
}

func (model *CombatModel) hasEmptyTileOnSide(army *Army) bool {
    side := model.GetSideForPlayer(army.Player)
    for y, row := range model.Tiles {
        for x := range row {
            if model.GetUnit(x, y) == nil && model.IsOnSide(x, y, side) {
                return true
            }
        }
    }
    return false
}

// in probe mode: whether any unit of the team passes the filter
func (model *CombatModel) probeUnits(team Team, canTarget func(*ArmyUnit) bool) {
    model.probe.asked = true
    for _, army := range []*Army{model.AttackingArmy, model.DefendingArmy} {
        for _, unit := range army.units {
            if unit.GetHealth() <= 0 {
                continue
            }
            if team != TeamEither && unit.Team != team {
                continue
            }
            if canTarget == nil || canTarget(unit) {
                model.probe.found = true
                return
            }
        }
    }
}

// in probe mode: whether any tile passes the filter
func (model *CombatModel) probeTiles(canTarget func(int, int) bool) {
    model.probe.asked = true
    for y, row := range model.Tiles {
        for x := range row {
            if canTarget == nil || canTarget(x, y) {
                model.probe.found = true
                return
            }
        }
    }
}

// A spell the AI cast (a wizard's, or a unit's from its charges), for frontends that replay the
// battle: the actions passed to Update may implement SpellCastObserver to hear about it.
type SpellCast struct {
    Army *Army
    // the unit that cast it; nil for the wizard
    Caster *ArmyUnit
    Spell spellbook.Spell
    // what the AI picked: a unit, or a tile (HasTile)
    Target *ArmyUnit
    TileX, TileY int
    HasTile bool
}

type SpellCastObserver interface {
    // an AI may cast now: what changed before belongs to earlier steps
    BeforeSpellCast()
    // the AI cast this spell and its projectiles have landed
    SpellCast(cast SpellCast)
}

// note the AI's target choice for the cast in progress
func (model *CombatModel) noteAITarget(unit *ArmyUnit, x int, y int, hasTile bool) {
    if model.aiCast != nil {
        model.aiCast.Target = unit
        model.aiCast.TileX, model.aiCast.TileY, model.aiCast.HasTile = x, y, hasTile
    }
}
