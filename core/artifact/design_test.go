package artifact

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/set"
)

type testWizard struct {
    books int
}

func (wizard testWizard) RetortEnabled(retort data.Retort) bool { return false }
func (wizard testWizard) MagicLevel(magic data.MagicType) int { return wizard.books }

func makeTestPowerTable() (map[Power]int, map[Power]set.Set[ArtifactType]) {
    attack1 := Power{Type: PowerTypeAttack, Amount: 1, Name: "+1 Attack"}
    attack3 := Power{Type: PowerTypeAttack, Amount: 3, Name: "+3 Attack"}
    defense := Power{Type: PowerTypeDefense, Amount: 1, Name: "+1 Defense"}
    flaming := Power{Type: PowerTypeAbility1, Name: "Flaming", Ability: data.ItemAbilityFlaming, Magic: data.ChaosMagic, Amount: 2}
    costs := map[Power]int{attack1: 20, attack3: 300, defense: 20, flaming: 100}
    swords := *set.NewSet(ArtifactTypeSword)
    compatibilities := map[Power]set.Set[ArtifactType]{attack1: swords, attack3: swords, defense: *set.NewSet(ArtifactTypeSword, ArtifactTypeShield), flaming: swords}
    return costs, compatibilities
}

func TestBuildDesign(test *testing.T) {
    costs, compatibilities := makeTestPowerTable()
    wizard := testWizard{books: 3}
    spells := spellbook.SpellsFromArray([]spellbook.Spell{{Name: "Fire Bolt", CastCost: 5}})

    item, err := BuildDesign(Design{Type: "Sword", Powers: []string{"+1 Attack", "+1 Defense", "Flaming"}}, false, wizard, spells, costs, compatibilities)
    if err != nil {
        test.Fatalf("design: %v", err)
    }
    if item.Type != ArtifactTypeSword || len(item.Powers) != 3 || item.Cost != CalculateCost(item, costs) || item.Name == "" {
        test.Errorf("item: %+v", item)
    }

    bad := []Design{
        {Type: "Hammer", Powers: nil},
        {Type: "Sword", Powers: []string{"+1 Attack", "+3 Attack"}},
        {Type: "Shield", Powers: []string{"+1 Attack"}},
        {Type: "Sword", Powers: []string{"Laser Eyes"}},
        {Type: "Sword", Image: 50},
        {Type: "Sword", Spell: "Fire Bolt", Charges: 2},
        {Type: "Wand", Spell: "Fire Bolt", Charges: 9},
    }
    for _, design := range bad {
        if _, err := BuildDesign(design, false, wizard, spells, costs, compatibilities); err == nil {
            test.Errorf("design %+v should be refused", design)
        }
    }

    if _, err := BuildDesign(Design{Type: "Sword", Powers: []string{"+3 Attack"}}, true, wizard, spells, costs, compatibilities); err == nil {
        test.Errorf("enchant item can't use a 300 cost power")
    }
    if _, err := BuildDesign(Design{Type: "Sword", Powers: []string{"Flaming"}}, false, testWizard{books: 1}, spells, costs, compatibilities); err == nil {
        test.Errorf("flaming needs two chaos books")
    }

    wand, err := BuildDesign(Design{Type: "Wand", Spell: "Fire Bolt", Charges: 2}, false, wizard, spells, costs, compatibilities)
    if err != nil || !wand.HasSpellCharges() {
        test.Errorf("a wand with fire bolt charges: %v", err)
    }
}
