package game

import (
    "fmt"
    "math/rand/v2"
    "slices"

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
}

type StartWizard struct {
    Name string `json:"name"`
    // spellbooks by realm, e.g. {"Life": 5, "Nature": 5}
    Books map[string]int `json:"books"`
    Retort string `json:"retort,omitempty"`
    // can start on Myrror with a Myrran race
    Myrran bool `json:"myrran,omitempty"`
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
        wizard := StartWizard{Name: slot.Name, Books: map[string]int{}, Myrran: slot.ExtraRetort == data.RetortMyrran}
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
    return options
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
    myrran := slot.ExtraRetort == data.RetortMyrran

    races := slices.Clone(data.ArcanianRaces())
    if myrran {
        races = append(races, data.MyrranRaces()...)
    }
    race := races[rand.N(len(races))]
    if raceName != "" {
        found := slices.IndexFunc(races, func(candidate data.Race) bool { return candidate.String() == raceName })
        if found < 0 {
            if slices.ContainsFunc(data.MyrranRaces(), func(candidate data.Race) bool { return candidate.String() == raceName }) {
                return nil, fmt.Errorf("only a wizard with the Myrran retort can play the %v", raceName)
            }
            return nil, fmt.Errorf("unknown race %q", raceName)
        }
        race = races[found]
    }

    banner := startBanners[rand.N(len(startBanners))]
    if bannerName != "" {
        found := slices.IndexFunc(startBanners, func(candidate data.BannerType) bool { return candidate.String() == bannerName })
        if found < 0 {
            return nil, fmt.Errorf("unknown banner %q", bannerName)
        }
        banner = startBanners[found]
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
