package setup

import (
    "fmt"
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

const MaxPicks = 11

type WizardSlot struct {
    Name string
    // the block that the wizard's name is printed on in the ui
    Background int
    Base data.WizardBase
    // the portrait of the wizard shown when the user's cursor is on top of their name
    Portrait int
    Books []data.WizardBook
    ExtraRetort data.Retort
}

// some abilities can only be selected if other properties of the wizard are set
func SatisifiedDependencies(retort data.Retort, wizard *WizardCustom) bool {
    switch retort {
        case data.RetortAlchemy: return true
        case data.RetortWarlord: return true
        case data.RetortChanneler: return true
        case data.RetortArchmage:
            // need at least 4 books of some magic type
            for _, book := range wizard.Books {
                if book.Count >= 4 {
                    return true
                }
            }
            return false
        case data.RetortArtificer: return true
        case data.RetortConjurer: return true
        case data.RetortSageMaster:
            // need at least 2 books of different magic types
            count := 0
            for _, book := range wizard.Books {
                if book.Count > 0 {
                    count += 1
                }
            }
            return count >= 2
        case data.RetortMyrran: return true
        case data.RetortDivinePower: return wizard.MagicLevel(data.LifeMagic) >= 4
        case data.RetortFamous: return true
        case data.RetortRunemaster:
            // need at least 3 books of different magic types with 2 picks per type
            count := 0
            for _, book := range wizard.Books {
                if book.Count >= 2 {
                    count += 1
                }
            }
            return count >= 3
        case data.RetortCharismatic: return true
        case data.RetortChaosMastery: return wizard.MagicLevel(data.ChaosMagic) >= 4
        case data.RetortNatureMastery: return wizard.MagicLevel(data.NatureMagic) >= 4
        case data.RetortSorceryMastery: return wizard.MagicLevel(data.SorceryMagic) >= 4
        case data.RetortInfernalPower: return wizard.MagicLevel(data.DeathMagic) >= 4
        case data.RetortManaFocusing:
            // need at least 4 books of some magic type
            for _, book := range wizard.Books {
                if book.Count >= 4 {
                    return true
                }
            }
            return false
        case data.RetortNodeMastery:
            // one pick in chaos, nature, and sorcery
            return wizard.MagicLevel(data.ChaosMagic) >= 1 && wizard.MagicLevel(data.NatureMagic) >= 1 && wizard.MagicLevel(data.SorceryMagic) >= 1
        case data.RetortNone: return true
    }

    return true
}

func DefaultWizardSlots() []WizardSlot {
    return []WizardSlot{
        WizardSlot{
            Name: "Merlin",
            Background: 9,
            Portrait: 0,
            Base: data.WizardMerlin,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.LifeMagic, Count: 5},
                data.WizardBook{Magic: data.NatureMagic, Count: 5},
            },
            ExtraRetort: data.RetortSageMaster,
        },
        WizardSlot{
            Name: "Raven",
            Background: 10,
            Portrait: 1,
            Base: data.WizardRaven,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.SorceryMagic, Count: 6},
                data.WizardBook{Magic: data.NatureMagic, Count: 5},
            },
            ExtraRetort: data.RetortNone,
        },
        WizardSlot{
            Name: "Sharee",
            Background: 11,
            Portrait: 2,
            Base: data.WizardSharee,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.DeathMagic, Count: 5},
                data.WizardBook{Magic: data.ChaosMagic, Count: 5},
            },
            ExtraRetort: data.RetortConjurer,
        },
        WizardSlot{
            Name: "Lo Pan",
            Background: 12,
            Portrait: 3,
            Base: data.WizardLoPan,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.SorceryMagic, Count: 5},
                data.WizardBook{Magic: data.ChaosMagic, Count: 5},
            },
            ExtraRetort: data.RetortChanneler,
        },
        WizardSlot{
            Name: "Jafar",
            Background: 13,
            Portrait: 4,
            Base: data.WizardJafar,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.SorceryMagic, Count: 10},
            },
            ExtraRetort: data.RetortAlchemy,
        },
        WizardSlot{
            Name: "Oberic",
            Background: 14,
            Portrait: 5,
            Base: data.WizardOberic,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.NatureMagic, Count: 5},
                data.WizardBook{Magic: data.ChaosMagic, Count: 5},
            },
            ExtraRetort: data.RetortManaFocusing,
        },
        WizardSlot{
            Name: "Rjak",
            Background: 15,
            Portrait: 6,
            Base: data.WizardRjak,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.DeathMagic, Count: 9},
            },
            ExtraRetort: data.RetortInfernalPower,
        },
        WizardSlot{
            Name: "Sss'ra",
            Background: 16,
            Portrait: 7,
            Base: data.WizardSssra,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.LifeMagic, Count: 4},
                data.WizardBook{Magic: data.ChaosMagic, Count: 4},
            },
            ExtraRetort: data.RetortMyrran,
        },
        WizardSlot{
            Name: "Tauron",
            Background: 17,
            Portrait: 8,
            Base: data.WizardTauron,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.ChaosMagic, Count: 10},
            },
            ExtraRetort: data.RetortChaosMastery,
        },
        WizardSlot{
            Name: "Freya",
            Background: 18,
            Portrait: 9,
            Base: data.WizardFreya,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.NatureMagic, Count: 10},
            },
            ExtraRetort: data.RetortNatureMastery,
        },
        WizardSlot{
            Name: "Horus",
            Background: 19,
            Portrait: 10,
            Base: data.WizardHorus,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.LifeMagic, Count: 5},
                data.WizardBook{Magic: data.SorceryMagic, Count: 5},
            },
            ExtraRetort: data.RetortArchmage,
        },
        WizardSlot{
            Name: "Ariel",
            Background: 20,
            Portrait: 11,
            Base: data.WizardAriel,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.LifeMagic, Count: 10},
            },
            ExtraRetort: data.RetortCharismatic,
        },
        WizardSlot{
            Name: "Tlaloc",
            Background: 21,
            Portrait: 12,
            Base: data.WizardTlaloc,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.NatureMagic, Count: 4},
                data.WizardBook{Magic: data.DeathMagic, Count: 5},
            },
            ExtraRetort: data.RetortWarlord,
        },
        WizardSlot{
            Name: "Kali",
            Background: 22,
            Portrait: 13,
            Base: data.WizardKali,
            Books: []data.WizardBook{
                data.WizardBook{Magic: data.SorceryMagic, Count: 5},
                data.WizardBook{Magic: data.DeathMagic, Count: 5},
            },
            ExtraRetort: data.RetortArtificer,
        },
        WizardSlot{
            Name: "Custom",
            Background: 23,
            Books: nil,
        },
    }
}

type WizardCustom struct {
    Name string
    // FIXME: remove portrait
    Portrait int
    Base data.WizardBase
    Retorts []data.Retort
    Books []data.WizardBook
    StartingSpells spellbook.Spells
    Race data.Race
    Banner data.BannerType
}

func (wizard *WizardCustom) MostBooks() data.MagicType {
    max := 0
    kind := data.LifeMagic

    for _, book := range wizard.Books {
        if book.Count > max {
            max = book.Count
            kind = book.Magic
        }
    }

    return kind
}

func (wizard *WizardCustom) TotalBooks() int {
    count := 0
    for _, book := range wizard.Books {
        count += book.Count
    }

    return count
}

func (wizard *WizardCustom) RetortEnabled(ability data.Retort) bool {
    for _, check := range wizard.Retorts {
        if check == ability {
            return true
        }
    }

    return false
}

func (wizard *WizardCustom) EnableRetort(retort data.Retort){
    for _, check := range wizard.Retorts {
        if check == retort {
            return
        }
    }

    wizard.Retorts = append(wizard.Retorts, retort)
}

func (wizard *WizardCustom) ToggleRetort(retort data.Retort, picksLeft int){
    var out []data.Retort

    found := false

    for _, check := range wizard.Retorts {
        if check == retort {
            found = true
        } else {
            out = append(out, check)
        }
    }

    if !found && retort.PickCost() <= picksLeft {
        out = append(out, retort)
    }

    wizard.Retorts = out
}

func (wizard *WizardCustom) AddMagicLevel(kind data.MagicType, count int){
    for i := range wizard.Books {
        if wizard.Books[i].Magic == kind {
            wizard.Books[i].Count += count
            return
        }
    }

    wizard.Books = append(wizard.Books, data.WizardBook{
        Magic: kind,
        Count: count,
    })
}

func (wizard *WizardCustom) SetMagicLevel(kind data.MagicType, count int){
    var out []data.WizardBook

    found := false

    for _, book := range wizard.Books {
        if book.Magic == kind {
            found = true
            if count != 0 {
                book.Count = count
                out = append(out, book)
            }
        } else {
            out = append(out, book)
        }
    }

    if !found {
        out = append(out, data.WizardBook{
            Magic: kind,
            Count: count,
        })
    }

    // fmt.Printf("Books: %+v\n", out)

    wizard.Books = out
}

// number of books for the given magic type
func (wizard *WizardCustom) MagicLevel(kind data.MagicType) int {
    for _, book := range wizard.Books {
        if book.Magic == kind {
            return book.Count
        }
    }

    return 0
}

func JoinAbilities(abilities []data.Retort) string {
    // this could be simplified by iterating backwards through the array and
    // preprending 'and' or ', ' for each element, depending on its index

    if len(abilities) == 0 {
        return ""
    }

    // x
    if len(abilities) == 1 {
        return abilities[0].String()
    }

    // x and y
    if len(abilities) == 2 {
        return fmt.Sprintf("%v and %v", abilities[0], abilities[1])
    }

    // x, y, and z
    out := ""
    for i := 0; i < len(abilities) - 2; i++ {
        out += fmt.Sprintf("%v, ", abilities[i])
    }
    out += fmt.Sprintf("%v and %v", abilities[len(abilities) - 2], abilities[len(abilities) - 1])
    return out
}

// a struct that stores how many spells the user can choose from each rarity
type ChooseSpellInfo struct {
    CommonMax int
    UncommonMax int
    RareMax int

    CommonSpells spellbook.Spells
    UncommonSpells spellbook.Spells
    RareSpells spellbook.Spells
}

func MakeChooseSpellInfo(allSpells spellbook.Spells, magic data.MagicType, books int) ChooseSpellInfo {
    computeCommon := func(books int) int {
        if books == 0 || books == 11 {
            return 0
        }

        if books < 11 {
            return books - 1
        }

        return 0
    }

    computeUncommon := func(books int) int {
        if books == 11 {
            return 2
        }

        return 0
    }

    computeRare := func(books int) int {
        if books == 11 {
            return 1
        }

        return 0
    }

    chooseSpells := func(rarity spellbook.SpellRarity) spellbook.Spells {
        return allSpells.GetSpellsByMagic(magic).GetSpellsByRarity(rarity)
    }

    commonMax := computeCommon(books)
    uncommonMax := computeUncommon(books)
    rareMax := computeRare(books)

    commonSpells := chooseSpells(spellbook.SpellRarityCommon)
    uncommonSpells := chooseSpells(spellbook.SpellRarityUncommon)
    rareSpells := chooseSpells(spellbook.SpellRarityRare)

    return ChooseSpellInfo{
        CommonMax: commonMax,
        UncommonMax: uncommonMax,
        RareMax: rareMax,

        CommonSpells: commonSpells,
        UncommonSpells: uncommonSpells,
        RareSpells: rareSpells,
    }
}

func GetStartingSpells(wizard *WizardCustom, allSpells spellbook.Spells) spellbook.Spells {
    var spellsOut spellbook.Spells
    magicOrder := []data.MagicType{data.LifeMagic, data.DeathMagic, data.ChaosMagic, data.NatureMagic, data.SorceryMagic}

    for _, magic := range magicOrder {
        spellInfo := MakeChooseSpellInfo(allSpells, magic, wizard.MagicLevel(magic))

        if wizard.MagicLevel(magic) == 11 {
            spellsOut.AddAllSpells(spellInfo.CommonSpells)
        }

        // assign common spells
        for _, index := range rand.Perm(len(spellInfo.CommonSpells.Spells))[0:spellInfo.CommonMax] {
            spellsOut.AddSpell(spellInfo.CommonSpells.Spells[index])
        }

        // assign uncommon spells
        for _, index := range rand.Perm(len(spellInfo.UncommonSpells.Spells))[0:spellInfo.UncommonMax] {
            spellsOut.AddSpell(spellInfo.UncommonSpells.Spells[index])
        }

        // assign rare spells
        for _, index := range rand.Perm(len(spellInfo.RareSpells.Spells))[0:spellInfo.RareMax] {
            spellsOut.AddSpell(spellInfo.RareSpells.Spells[index])
        }
    }

    return spellsOut
}
