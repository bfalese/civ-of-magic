package game

import (
    "io"
    "log"
    "os"
    "strings"
    "testing"

    "github.com/bfalese/civ-of-magic/core/gamedata"
)

// through the host: your Spell of Mastery wins the game, a rival's loses it
func TestHostSpellOfMastery(test *testing.T) {
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

    mastery := gameData.Spells.FindByName("Spell of Mastery")
    if !mastery.Valid() {
        test.Fatalf("no Spell of Mastery in the data")
    }

    // you, with mana and skill to cast it at once
    host, err := MakeGameHost(gameData, HostSettings{Opponents: 1})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    human := host.Human
    human.KnownSpells.AddSpell(mastery)
    human.Mana = 100000
    human.CastingSkillPower = 100000000
    // the spell costs SpellOfMasteryCost (60000 at the start): enough skill left this turn for it
    human.RemainingCastingSkill = 100000
    host.TakeEvents()
    if err := host.ExecuteJSON([]byte(`{"type": "cast-spell", "command": {"spell": "Spell of Mastery"}}`)); err != nil {
        test.Fatalf("cast: %v", err)
    }
    if host.GameOver() != "won" {
        test.Errorf("casting the Spell of Mastery should win, game over is %q", host.GameOver())
    }
    announced := false
    for _, event := range host.TakeEvents() {
        if strings.Contains(string(event), `"spell-of-mastery"`) {
            announced = true
        }
    }
    if !announced {
        test.Errorf("the win should come as a spell-of-mastery event")
    }
    if err := host.Execute(&EndTurnCommand{}); err == nil {
        test.Errorf("the game is over: no more turns")
    }

    // a rival about to finish it: the next end-turn loses the game
    host, err = MakeGameHost(gameData, HostSettings{Opponents: 1})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    rival := host.ai.WizardsLeft()[0]
    rival.KnownSpells.AddSpell(mastery)
    rival.CastingSpell = mastery
    rival.CastingSpellProgress = rival.ComputeEffectiveSpellCost(mastery, true) - 1
    rival.Mana = 10000
    // a new game's wizards get their casting skill at the end of the first turn
    rival.RemainingCastingSkill = 100
    host.TakeEvents()
    if err := host.Execute(&EndTurnCommand{}); err != nil {
        test.Fatalf("end turn: %v", err)
    }
    for host.WaitingFor() == WaitingBattle {
        host.Execute(&BattleAutoCommand{})
    }
    if host.GameOver() != "lost" || host.Model.MasteryWinner != rival {
        test.Errorf("the rival's Spell of Mastery should end the game: %q", host.GameOver())
    }
}
