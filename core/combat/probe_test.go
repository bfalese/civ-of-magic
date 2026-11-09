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
    for i, match := range cases {
        caseEnd := len(body) - switchStart
        if i + 1 < len(cases) {
            caseEnd = cases[i + 1][0]
        }
        caseBody := body[switchStart + match[1]:switchStart + caseEnd]
        if strings.Contains(caseBody, "DoTargetUnitSpell") || strings.Contains(caseBody, "DoTargetTileSpell") || strings.Contains(caseBody, "DoSummoningSpell") {
            for _, name := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(body[switchStart + match[2]:switchStart + match[3]], -1) {
                targeted[name[1]] = true
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

    // bowmen: Warp Wood has a target now
    model, army = makeProbeBattle(test, []units.Unit{units.HighMenBowmen})
    if hasTarget, _ := model.SpellHasTarget(nil, army, nil, spell("Warp Wood", data.NatureMagic)); !hasTarget {
        test.Errorf("Warp Wood should target bowmen")
    }
    if hasTarget, _ := model.SpellHasTarget(nil, army, nil, spell("Dispel Evil", data.LifeMagic)); hasTarget {
        test.Errorf("Dispel Evil shouldn't target bowmen")
    }
}
