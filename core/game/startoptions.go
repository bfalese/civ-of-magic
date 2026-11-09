package game

import (
    "fmt"
    "math/rand/v2"
    "slices"

    "github.com/bfalese/civ-of-magic/core/assets"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// The choices for a new game (a frontend's start screen), and turning the chosen premade wizard,
// race and banner into the human wizard (HostSettings Wizard / Race / Banner).

type StartOptions struct {
    Wizards []StartWizard `json:"wizards"`
    // Arcanus races for most wizards; a wizard with the Myrran retort may also pick a Myrror race
    Races []StartRace `json:"races"`
    Banners []string `json:"banners"`
    Difficulties []string `json:"difficulties"`
    Magic []string `json:"magic"`
    LandSizes []string `json:"land-sizes"`
    MinOpponents int `json:"min-opponents"`
    MaxOpponents int `json:"max-opponents"`
    // for the custom wizard builder (HostSettings Custom)
    Custom CustomWizardOptions `json:"custom"`
}

type CustomWizardOptions struct {
    // books and retorts together use exactly this many picks
    Picks int `json:"picks"`
    // the realms of spellbooks, in the game's order; Life and Death don't mix
    Realms []string `json:"realms"`
    Retorts []StartRetort `json:"retorts"`
    // the portraits to choose from: {portrait (the number custom takes), asset}
    Portraits []StartPortrait `json:"portraits"`
}

type StartRetort struct {
    Name string `json:"name"`
    Picks int `json:"picks"`
    // what it needs in books, as the game explains it ("" when nothing)
    Requires string `json:"requires,omitempty"`
}

type StartPortrait struct {
    Portrait int `json:"portrait"`
    Asset string `json:"asset"`
}

type StartWizard struct {
    Name string `json:"name"`
    // spellbooks by realm, e.g. {"Life": 5, "Nature": 5}
    Books map[string]int `json:"books"`
    Retort string `json:"retort,omitempty"`
    // can start on Myrror with a Myrran race
    Myrran bool `json:"myrran,omitempty"`
    // the portrait's asset id
    Portrait string `json:"portrait"`
}

type StartRace struct {
    Name string `json:"name"`
    Myrran bool `json:"myrran,omitempty"`
}

var startBanners = []data.BannerType{data.BannerGreen, data.BannerBlue, data.BannerRed, data.BannerPurple, data.BannerYellow}

func MakeStartOptions() StartOptions {
    options := StartOptions{
        Difficulties: []string{"intro", "easy", "average", "hard", "extreme", "impossible"},
        Magic: []string{"weak", "normal", "powerful"},
        LandSizes: []string{"small", "medium", "large"},
        MinOpponents: 1,
        MaxOpponents: 4,
    }
    for _, slot := range setup.DefaultWizardSlots() {
        if slot.Name == "Custom" {
            continue
        }
        wizard := StartWizard{Name: slot.Name, Books: map[string]int{}, Myrran: slot.ExtraRetort == data.RetortMyrran, Portrait: assets.WizardPortrait(slot.Base)}
        for _, book := range slot.Books {
            wizard.Books[book.Magic.String()] += book.Count
        }
        if slot.ExtraRetort != data.RetortNone {
            wizard.Retort = slot.ExtraRetort.String()
        }
        options.Wizards = append(options.Wizards, wizard)
    }
    for _, race := range data.ArcanianRaces() {
        options.Races = append(options.Races, StartRace{Name: race.String()})
    }
    for _, race := range data.MyrranRaces() {
        options.Races = append(options.Races, StartRace{Name: race.String(), Myrran: true})
    }
    for _, banner := range startBanners {
        options.Banners = append(options.Banners, banner.String())
    }

    options.Custom = CustomWizardOptions{Picks: setup.MaxPicks, Realms: []string{}, Retorts: []StartRetort{}, Portraits: []StartPortrait{}}
    for _, realm := range startRealms {
        options.Custom.Realms = append(options.Custom.Realms, realm.String())
    }
    for _, retort := range startRetorts() {
        options.Custom.Retorts = append(options.Custom.Retorts, StartRetort{Name: retort.String(), Picks: retort.PickCost(), Requires: retort.DependencyExplanation()})
    }
    for base := data.WizardMerlin; base <= data.WizardKali; base++ {
        options.Custom.Portraits = append(options.Custom.Portraits, StartPortrait{Portrait: int(base), Asset: assets.WizardPortrait(base)})
    }
    return options
}

// the race of the first city: one by name, or random among those the wizard may play (Myrror
// races only with the Myrran retort)
func chooseStartRace(raceName string, myrran bool) (data.Race, error) {
    races := slices.Clone(data.ArcanianRaces())
    if myrran {
        races = append(races, data.MyrranRaces()...)
    }
    if raceName == "" {
        return races[rand.N(len(races))], nil
    }
    found := slices.IndexFunc(races, func(candidate data.Race) bool { return candidate.String() == raceName })
    if found < 0 {
        if slices.ContainsFunc(data.MyrranRaces(), func(candidate data.Race) bool { return candidate.String() == raceName }) {
            return 0, fmt.Errorf("only a wizard with the Myrran retort can play the %v", raceName)
        }
        return 0, fmt.Errorf("unknown race %q", raceName)
    }
    return races[found], nil
}

// a banner by name, or a random one
func chooseStartBanner(bannerName string) (data.BannerType, error) {
    if bannerName == "" {
        return startBanners[rand.N(len(startBanners))], nil
    }
    found := slices.IndexFunc(startBanners, func(candidate data.BannerType) bool { return candidate.String() == bannerName })
    if found < 0 {
        return 0, fmt.Errorf("unknown banner %q", bannerName)
    }
    return startBanners[found], nil
}

// the human wizard from a premade wizard's name, a race and a banner (empty race or banner:
// random among the allowed ones)
func makeStartWizard(name string, raceName string, bannerName string, allSpells spellbook.Spells) (*setup.WizardCustom, error) {
    index := slices.IndexFunc(setup.DefaultWizardSlots(), func(slot setup.WizardSlot) bool {
        return slot.Name == name && slot.Name != "Custom"
    })
    if index < 0 {
        return nil, fmt.Errorf("no wizard named %q", name)
    }
    slot := setup.DefaultWizardSlots()[index]

    race, err := chooseStartRace(raceName, slot.ExtraRetort == data.RetortMyrran)
    if err != nil {
        return nil, err
    }
    banner, err := chooseStartBanner(bannerName)
    if err != nil {
        return nil, err
    }

    wizard := &setup.WizardCustom{
        Name: slot.Name,
        Base: slot.Base,
        Race: race,
        Books: slices.Clone(slot.Books),
        Banner: banner,
    }
    if slot.ExtraRetort != data.RetortNone {
        wizard.Retorts = []data.Retort{slot.ExtraRetort}
    }
    wizard.StartingSpells.AddAllSpells(setup.GetStartingSpells(wizard, allSpells))
    return wizard, nil
}

// a wizard made on the start screen (HostSettings Custom): a name, a portrait, spellbooks and
// retorts that use all setup.MaxPicks picks
type CustomWizardSettings struct {
    Name string `json:"name"`
    // the portrait: a premade wizard's (data.WizardBase, 0-13)
    Portrait int `json:"portrait"`
    // spellbooks by realm, e.g. {"Life": 6, "Nature": 3}
    Books map[string]int `json:"books"`
    Retorts []string `json:"retorts"`
}

var startRealms = []data.MagicType{data.LifeMagic, data.DeathMagic, data.ChaosMagic, data.NatureMagic, data.SorceryMagic}

func startRetorts() []data.Retort {
    var out []data.Retort
    for retort := data.RetortAlchemy; retort < data.RetortNone; retort++ {
        out = append(out, retort)
    }
    return out
}

// the wizard the builder's choices make, and the picks they use; an error for a choice that can't
// be (an unknown realm or retort, too many books, Life with Death, a retort chosen twice). Retort
// requirements and the pick count are checked by the callers.
func parseCustomWizard(custom CustomWizardSettings) (*setup.WizardCustom, int, error) {
    if custom.Portrait < int(data.WizardMerlin) || custom.Portrait > int(data.WizardKali) {
        return nil, 0, fmt.Errorf("no portrait %v", custom.Portrait)
    }
    wizard := &setup.WizardCustom{Name: custom.Name, Base: data.WizardBase(custom.Portrait), Portrait: custom.Portrait}

    picks := 0
    for _, realm := range startRealms {
        count, ok := custom.Books[realm.String()]
        if !ok {
            continue
        }
        if count < 0 || count > 11 {
            return nil, 0, fmt.Errorf("%v: 0 to 11 books, not %v", realm, count)
        }
        if count > 0 {
            wizard.SetMagicLevel(realm, count)
            picks += count
        }
    }
    for realmName := range custom.Books {
        if !slices.ContainsFunc(startRealms, func(realm data.MagicType) bool { return realm.String() == realmName }) {
            return nil, 0, fmt.Errorf("unknown realm %q", realmName)
        }
    }
    if wizard.MagicLevel(data.LifeMagic) > 0 && wizard.MagicLevel(data.DeathMagic) > 0 {
        return nil, 0, fmt.Errorf("a wizard can't have both Life and Death books")
    }

    for _, retortName := range custom.Retorts {
        index := slices.IndexFunc(startRetorts(), func(retort data.Retort) bool { return retort.String() == retortName })
        if index < 0 {
            return nil, 0, fmt.Errorf("unknown retort %q", retortName)
        }
        retort := startRetorts()[index]
        if wizard.RetortEnabled(retort) {
            return nil, 0, fmt.Errorf("%v chosen twice", retortName)
        }
        wizard.Retorts = append(wizard.Retorts, retort)
        picks += retort.PickCost()
    }
    return wizard, picks, nil
}

// what the builder shows while the player chooses (civ.checkWizard): the picks left, the first
// problem with the choices so far, and for each retort why it can't be taken now ("" when it can;
// for a retort already taken, why it can't stay: its books are missing)
type CustomWizardCheck struct {
    PicksLeft int `json:"picks-left"`
    Error string `json:"error,omitempty"`
    Retorts map[string]string `json:"retorts"`
    // the choices make a wizard a game can start with
    Ready bool `json:"ready"`
}

func CheckCustomWizard(custom CustomWizardSettings) CustomWizardCheck {
    check := CustomWizardCheck{Retorts: map[string]string{}}
    wizard, picks, err := parseCustomWizard(custom)
    if err != nil {
        check.Error = err.Error()
        return check
    }
    check.PicksLeft = setup.MaxPicks - picks
    for _, retort := range wizard.Retorts {
        if !setup.SatisifiedDependencies(retort, wizard) {
            check.Error = retort.DependencyExplanation()
            break
        }
    }
    for _, retort := range startRetorts() {
        why := ""
        if !setup.SatisifiedDependencies(retort, wizard) {
            why = retort.DependencyExplanation()
        } else if !wizard.RetortEnabled(retort) {
            if retort.PickCost() > check.PicksLeft {
                why = fmt.Sprintf("needs %v pick(s)", retort.PickCost())
            }
        }
        check.Retorts[retort.String()] = why
    }
    if check.Error == "" && check.PicksLeft < 0 {
        check.Error = fmt.Sprintf("%v picks too many", -check.PicksLeft)
    }
    check.Ready = check.Error == "" && check.PicksLeft == 0 && custom.Name != ""
    return check
}

// the human wizard from the start screen's builder, checked as the desktop's new wizard screen
// does: every pick used, books 0-11 per realm, not both Life and Death, retorts whose book
// requirements are met. The starting spells are picked at random for the books, as for premade
// wizards.
func makeCustomWizard(custom CustomWizardSettings, raceName string, bannerName string, allSpells spellbook.Spells) (*setup.WizardCustom, error) {
    if custom.Name == "" {
        return nil, fmt.Errorf("the wizard needs a name")
    }
    wizard, picks, err := parseCustomWizard(custom)
    if err != nil {
        return nil, err
    }
    for _, retort := range wizard.Retorts {
        if !setup.SatisifiedDependencies(retort, wizard) {
            return nil, fmt.Errorf("%v", retort.DependencyExplanation())
        }
    }
    if picks != setup.MaxPicks {
        return nil, fmt.Errorf("use all %v picks (%v used)", setup.MaxPicks, picks)
    }

    race, err := chooseStartRace(raceName, wizard.RetortEnabled(data.RetortMyrran))
    if err != nil {
        return nil, err
    }
    banner, err := chooseStartBanner(bannerName)
    if err != nil {
        return nil, err
    }
    wizard.Race = race
    wizard.Banner = banner
    wizard.StartingSpells.AddAllSpells(setup.GetStartingSpells(wizard, allSpells))
    return wizard, nil
}
