package combat

import (
    "os"
    "regexp"
    "strings"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// the spells InvokeSpell asks a target for (by its source) are the probed ones plus Raise Dead
// (which picks among dead units first and has its own check): a new targeted spell, or a case
// that starts doing more than asking, must update probedSpells
func TestProbedSpellsList(test *testing.T) {
    source, err := os.ReadFile("model.go")
    if err != nil {
        test.Fatalf("read model.go: %v", err)
    }
    text := string(source)
    start := strings.Index(text, "func (model *CombatModel) InvokeSpell(")
    end := strings.Index(text[start:], "\n}\n")
    body := text[start:start + end]
    switchStart := strings.Index(body, "    switch spell.Name {")
    cases := regexp.MustCompile(`(?m)^        case ("[^:]+"):\n`).FindAllStringSubmatchIndex(body[switchStart:], -1)

    targeted := map[string]bool{}
    area := map[string]bool{}
    enchantments := map[string]string{}
    for i, match := range cases {
        caseEnd := len(body) - switchStart
        if i + 1 < len(cases) {
            caseEnd = cases[i + 1][0]
        }
        caseBody := body[switchStart + match[1]:switchStart + caseEnd]
        names := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(body[switchStart + match[2]:switchStart + match[3]], -1)
        if strings.Contains(caseBody, "DoTargetUnitSpell") || strings.Contains(caseBody, "DoTargetTileSpell") || strings.Contains(caseBody, "DoSummoningSpell") {
            for _, name := range names {
                targeted[name[1]] = true
            }
        }
        if strings.Contains(caseBody, "DoAllUnitsSpell") {
            for _, name := range names {
                area[name[1]] = true
            }
        }
        if constant := regexp.MustCompile(`CastEnchantment\(army, data\.(CombatEnchantment\w+)`).FindStringSubmatch(caseBody); constant != nil {
            for _, name := range names {
                enchantments[name[1]] = constant[1]
            }
        }
    }
    if len(targeted) < 60 {
        test.Fatalf("found only %v targeted spells in InvokeSpell: the scan is broken", len(targeted))
    }
    for name := range targeted {
        if !probedSpells[name] && name != "Raise Dead" {
            test.Errorf("%v asks for a target in InvokeSpell but isn't in probedSpells", name)
        }
    }
    for name := range probedSpells {
        if !targeted[name] {
            test.Errorf("%v is in probedSpells but InvokeSpell doesn't ask a target for it", name)
        }
    }

    // area spells (Mass Healing has its own rule), and combat enchantments by their constant
    for name := range area {
        if !areaSpells[name] && name != "Mass Healing" {
            test.Errorf("%v applies DoAllUnitsSpell but isn't in areaSpells", name)
        }
    }
    for name := range areaSpells {
        if !area[name] {
            test.Errorf("%v is in areaSpells but InvokeSpell doesn't apply DoAllUnitsSpell for it", name)
        }
    }
    if len(enchantments) != len(enchantmentSpells) {
        test.Errorf("InvokeSpell casts %v combat enchantments, enchantmentSpells has %v", len(enchantments), len(enchantmentSpells))
    }
    for name, constant := range enchantments {
        enchantment, ok := enchantmentSpells[name]
        if !ok {
            test.Errorf("%v casts %v but isn't in enchantmentSpells", name, constant)
            continue
        }
        if constant != "CombatEnchantment" + strings.ReplaceAll(enchantment.Name(), " ", "") {
            test.Errorf("enchantmentSpells[%v] is %v, InvokeSpell casts %v", name, enchantment.Name(), constant)
        }
    }
}

func makeProbeBattle(test *testing.T, defenders []units.Unit) (*CombatModel, *Army) {
    human := playerlib.MakePlayer(setup.WizardCustom{Name: "Human", Banner: data.BannerBlue}, true, 0, 0, nil, &noGlobalEnchantments{})
    enemy := playerlib.MakePlayer(setup.WizardCustom{Name: "Enemy", Banner: data.BannerRed}, false, 0, 0, nil, &noGlobalEnchantments{})
    attacking := &Army{Player: human}
    attacking.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSwordsmen, 1, 1, data.PlaneArcanus, human.Wizard.Banner, human.MakeExperienceInfo(), human.MakeUnitEnchantmentProvider()))
    defending := &Army{Player: enemy}
    for _, unit := range defenders {
        defending.AddUnit(units.MakeOverworldUnitFromUnit(unit, 1, 1, data.PlaneArcanus, enemy.Wizard.Banner, enemy.MakeExperienceInfo(), enemy.MakeUnitEnchantmentProvider()))
    }
    var allSpells spellbook.Spells
    model := MakeCombatModel(allSpells, defending, attacking, CombatLandscapeGrass, data.PlaneArcanus, ZoneType{}, data.MagicNone, 0, 0, make(chan CombatEvent, 10), nil)
    return model, attacking
}

func TestSpellHasTarget(test *testing.T) {
    // no city: no walls to disrupt; skeletons are fantastic death units
    model, army := makeProbeBattle(test, []units.Unit{units.Skeleton})
    spell := func(name string, magic data.MagicType) spellbook.Spell {
        return spellbook.Spell{Name: name, Magic: magic}
    }
    check := func(name string, magic data.MagicType, wantTarget bool, wantNeeds bool) {
        hasTarget, needsTarget := model.SpellHasTarget(nil, army, nil, spell(name, magic))
        if hasTarget != wantTarget || needsTarget != wantNeeds {
            test.Errorf("%v: has target %v needs %v, want %v %v", name, hasTarget, needsTarget, wantTarget, wantNeeds)
        }
    }
    events := len(model.Events)

    check("Disrupt", data.SorceryMagic, false, true)
    check("Fire Bolt", data.ChaosMagic, true, true)
    check("Dispel Evil", data.LifeMagic, true, true)
    check("Healing", data.LifeMagic, true, true)
    check("Recall Hero", data.ArcaneMagic, false, true)
    check("Warp Wood", data.NatureMagic, false, true)
    check("Raise Dead", data.LifeMagic, false, true)
    check("Animate Dead", data.DeathMagic, false, true)
    check("Phantom Warriors", data.SorceryMagic, true, true)
    // spells without a target: always worth trying
    check("Flame Strike", data.ChaosMagic, false, false)
    check("Wall of Fire", data.ChaosMagic, false, false)

    if len(model.Events) != events {
        test.Errorf("a probe sent %v events", len(model.Events) - events)
    }
    if army.Casted || model.probe != nil {
        test.Errorf("a probe left traces: casted %v, probe %v", army.Casted, model.probe)
    }

    // why spells wouldn't work
    reason := func(name string, magic data.MagicType) string {
        _, why := model.SpellCanWork(nil, army, nil, spell(name, magic))
        return why
    }
    for _, expect := range []struct{ name string; magic data.MagicType; why string }{
        {"Disrupt", data.SorceryMagic, "nothing to cast it on here"},
        {"Raise Dead", data.LifeMagic, "no dead to raise"},
        {"Holy Word", data.LifeMagic, ""},
        {"Mass Healing", data.LifeMagic, "nobody is wounded"},
        {"Prayer", data.LifeMagic, ""},
        {"Fire Bolt", data.ChaosMagic, ""},
        {"Flame Strike", data.ChaosMagic, ""},
    } {
        if why := reason(expect.name, expect.magic); why != expect.why {
            test.Errorf("%v: %q, want %q", expect.name, why, expect.why)
        }
    }
    army.AddEnchantment(data.CombatEnchantmentPrayer)
    if why := reason("Prayer", data.LifeMagic); why != "already in effect" {
        test.Errorf("Prayer in effect: %q", why)
    }
    for _, unit := range army.units {
        unit.TakeDamage(1, DamageNormal)
    }
    if why := reason("Mass Healing", data.LifeMagic); why != "" {
        test.Errorf("Mass Healing with a wounded unit: %q", why)
    }

    // bowmen: Warp Wood has a target now; Holy Word doesn't (no fantastic units)
    model, army = makeProbeBattle(test, []units.Unit{units.HighMenBowmen})
    if hasTarget, _ := model.SpellHasTarget(nil, army, nil, spell("Warp Wood", data.NatureMagic)); !hasTarget {
        test.Errorf("Warp Wood should target bowmen")
    }
    if hasTarget, _ := model.SpellHasTarget(nil, army, nil, spell("Dispel Evil", data.LifeMagic)); hasTarget {
        test.Errorf("Dispel Evil shouldn't target bowmen")
    }
    if _, why := model.SpellCanWork(nil, army, nil, spell("Holy Word", data.LifeMagic)); why != "nothing it would affect here" {
        test.Errorf("Holy Word on bowmen: %q", why)
    }
}

// Disenchant Area needs something to dispel, Call Chaos a living enemy, Summon Demon a free tile
func TestSpellCanWorkDisenchantChaosDemon(test *testing.T) {
    model, army := makeProbeBattle(test, []units.Unit{units.HighMenSpearmen})
    reason := func(name string) string {
        _, why := model.SpellCanWork(nil, army, nil, spellbook.Spell{Name: name})
        return why
    }
    enemy := model.GetOppositeArmyForPlayer(army.Player)

    for _, name := range []string{"Disenchant Area", "Disenchant True"} {
        if why := reason(name); why != "nothing to dispel here" {
            test.Errorf("%v with no enchantments anywhere: %q", name, why)
        }
    }
    enemy.units[0].AddEnchantment(data.UnitEnchantmentGiantStrength)
    if why := reason("Disenchant Area"); why != "" {
        test.Errorf("an enemy unit's enchantment can be dispelled: %q", why)
    }
    enemy.units[0].Enchantments = nil
    army.units[0].AddCurse(data.UnitCurseWeakness)
    if why := reason("Disenchant True"); why != "" {
        test.Errorf("a curse on your unit can be dispelled: %q", why)
    }
    army.units[0].Curses = nil
    enemy.AddEnchantment(data.CombatEnchantmentPrayer)
    if why := reason("Disenchant Area"); why != "" {
        test.Errorf("an enemy combat enchantment can be dispelled: %q", why)
    }

    if why := reason("Call Chaos"); why != "" {
        test.Errorf("Call Chaos with a living enemy: %q", why)
    }
    enemy.units[0].TakeDamage(enemy.units[0].GetHealth(), DamageNormal)
    if why := reason("Call Chaos"); why != "nothing it would affect here" {
        test.Errorf("Call Chaos with no living enemy: %q", why)
    }

    if why := reason("Summon Demon"); why != "" {
        test.Errorf("Summon Demon on an open battlefield: %q", why)
    }
}
