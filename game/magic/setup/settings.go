package setup

import (
    "github.com/bfalese/civ-of-magic/core/data"
)

const OpponentsMax = 4
const LandSizeMax = 2

type NewGameSettings struct {
    Difficulty data.DifficultySetting
    Opponents int
    LandSize int
    Magic data.MagicSetting
    // set to true to disable raiders from appearing in the game
    DisableRaiders bool
}

func (settings *NewGameSettings) DifficultyNext() {
    difficulties := []data.DifficultySetting{
        data.DifficultyIntro,
        data.DifficultyEasy,
        data.DifficultyAverage,
        data.DifficultyHard,
        data.DifficultyExtreme,
        data.DifficultyImpossible,
    }

    for i, diff := range difficulties {
        if diff == settings.Difficulty {
            settings.Difficulty = difficulties[(i + 1) % len(difficulties)]
            return
        }
    }
}

func (settings *NewGameSettings) OpponentsNext() {
    settings.Opponents += 1
    if settings.Opponents > OpponentsMax {
        settings.Opponents = 1
    }
}

func (settings *NewGameSettings) LandSizeNext() {
    settings.LandSize += 1
    if settings.LandSize > LandSizeMax {
        settings.LandSize = 0
    }
}

func (settings *NewGameSettings) MagicNext() {
    switch settings.Magic {
        case data.MagicSettingWeak: settings.Magic = data.MagicSettingNormal
        case data.MagicSettingNormal: settings.Magic = data.MagicSettingPowerful
        case data.MagicSettingPowerful: settings.Magic = data.MagicSettingWeak
    }
}

func (settings *NewGameSettings) DifficultyString() string {
    names := map[data.DifficultySetting]string{
        data.DifficultyIntro: "Intro",
        data.DifficultyEasy: "Easy",
        data.DifficultyAverage: "Average",
        data.DifficultyHard: "Hard",
        data.DifficultyExtreme: "Extreme",
        data.DifficultyImpossible: "Impossible",
    }
    return names[settings.Difficulty]
}

func (settings *NewGameSettings) OpponentsString() string {
    kinds := []string{"One", "Two", "Three", "Four"}
    return kinds[settings.Opponents - 1]
}

func (settings *NewGameSettings) LandSizeString() string {
    kinds := []string{"Small", "Medium", "Large"}
    return kinds[settings.LandSize]
}

func (settings *NewGameSettings) MagicString() string {
    kinds := map[data.MagicSetting]string{
        data.MagicSettingWeak: "Weak",
        data.MagicSettingNormal: "Normal",
        data.MagicSettingPowerful: "Powerful",
    }
    return kinds[settings.Magic]
}
