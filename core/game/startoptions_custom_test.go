package game

import (
    "io"
    "log"
    "os"
    "strings"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

func TestMakeCustomWizard(test *testing.T) {
    var spells spellbook.Spells
    custom := func(books map[string]int, retorts ...string) CustomWizardSettings {
        return CustomWizardSettings{Name: "Tester", Portrait: 3, Books: books, Retorts: retorts}
    }

    wizard, err := makeCustomWizard(custom(map[string]int{"Life": 6, "Nature": 3}, "Divine Power"), "", "red", spells)
    if err != nil {
        test.Fatalf("a valid wizard: %v", err)
    }
    if wizard.Name != "Tester" || wizard.Base != data.WizardLoPan || wizard.MagicLevel(data.LifeMagic) != 6 || !wizard.RetortEnabled(data.RetortDivinePower) || wizard.Banner != data.BannerRed {
        test.Errorf("the wizard as chosen: %+v", wizard)
    }

    for _, bad := range []struct {
        settings CustomWizardSettings
        why string
    }{
        {custom(map[string]int{"Life": 5}), "use all 11 picks"},
        {custom(map[string]int{"Life": 6, "Death": 5}), "both Life and Death"},
        {custom(map[string]int{"Chaos": 10}, "Divine Power"), "Life Magic"},
        {custom(map[string]int{"Chaos": 9}, "Alchemy", "Alchemy"), "twice"},
        {custom(map[string]int{"Fire": 11}), "unknown realm"},
        {custom(map[string]int{"Chaos": 10}, "Luck"), "unknown retort"},
        {custom(map[string]int{"Chaos": 12}), "0 to 11"},
        {CustomWizardSettings{Portrait: 0, Books: map[string]int{"Chaos": 11}}, "name"},
        {CustomWizardSettings{Name: "X", Portrait: 20, Books: map[string]int{"Chaos": 11}}, "portrait"},
    } {
        if _, err := makeCustomWizard(bad.settings, "", "", spells); err == nil || !strings.Contains(err.Error(), bad.why) {
            test.Errorf("%+v: want an error about %q, got %v", bad.settings, bad.why, err)
        }
    }

    // a Myrror race needs the Myrran retort
    if _, err := makeCustomWizard(custom(map[string]int{"Sorcery": 10}, "Alchemy"), "Dark Elf", "", spells); err == nil {
        test.Errorf("a Myrror race without the Myrran retort should be refused")
    }
    if wizard, err := makeCustomWizard(custom(map[string]int{"Sorcery": 8}, "Myrran"), "Dark Elf", "", spells); err != nil || wizard.Race != data.RaceDarkElf {
        test.Errorf("Myrran retort with a Myrror race: %v", err)
    }

    options := MakeStartOptions()
    if options.Custom.Picks != 11 || len(options.Custom.Realms) != 5 || len(options.Custom.Retorts) != 18 || len(options.Custom.Portraits) != 14 {
        test.Errorf("builder options: %+v", options.Custom)
    }
}

// a game with a custom wizard from JSON settings: its books, retorts and starting spells
func TestHostCustomWizard(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    settings, err := DecodeHostSettings([]byte(`{"opponents": 1, "custom": {"name": "Zed", "portrait": 7, "books": {"Chaos": 6, "Sorcery": 3}, "retorts": ["Warlord"]}, "race": "High Elf", "banner": "yellow"}`))
    if err != nil {
        test.Fatalf("settings: %v", err)
    }
    host, err := MakeGameHost(gameData, settings)
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    wizard := host.Human.Wizard
    if wizard.Name != "Zed" || wizard.MagicLevel(data.ChaosMagic) != 6 || !wizard.RetortEnabled(data.RetortWarlord) || host.Human.GetBanner() != data.BannerYellow {
        test.Errorf("the custom wizard: %+v", wizard)
    }
    chaos := 0
    for _, spell := range host.Human.KnownSpells.Spells {
        if spell.Magic == data.ChaosMagic {
            chaos += 1
        }
    }
    if chaos < 5 {
        test.Errorf("6 Chaos books should give 5 common Chaos spells, got %v", chaos)
    }
    if host.Model.MakePlayerView(host.Human).You.Mirror.Portrait != "wizard-portrait/7" {
        test.Errorf("the portrait should be the chosen one")
    }

    settings.Custom.Books = map[string]int{"Chaos": 2}
    if _, err := MakeGameHost(gameData, settings); err == nil {
        test.Errorf("unused picks should be refused")
    }
}

func TestCheckCustomWizard(test *testing.T) {
    check := CheckCustomWizard(CustomWizardSettings{Name: "X", Portrait: 0, Books: map[string]int{"Life": 4}})
    if check.PicksLeft != 7 || check.Ready || check.Error != "" {
        test.Errorf("4 Life books: %+v", check)
    }
    if check.Retorts["Divine Power"] != "" || check.Retorts["Infernal Power"] == "" || check.Retorts["Runemaster"] == "" {
        test.Errorf("retorts with 4 Life books: %+v", check.Retorts)
    }
    check = CheckCustomWizard(CustomWizardSettings{Name: "X", Portrait: 0, Books: map[string]int{"Life": 4}, Retorts: []string{"Myrran", "Famous", "Warlord"}})
    if check.PicksLeft != 0 || !check.Ready || check.Retorts["Alchemy"] != "needs 1 pick(s)" {
        test.Errorf("all picks used: %+v", check)
    }
    check = CheckCustomWizard(CustomWizardSettings{Name: "X", Portrait: 0, Books: map[string]int{"Chaos": 11}, Retorts: []string{"Divine Power"}})
    if check.Ready || !strings.Contains(check.Error, "Life Magic") || check.Retorts["Divine Power"] == "" {
        test.Errorf("a retort without its books: %+v", check)
    }
}
