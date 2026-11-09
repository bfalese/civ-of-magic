package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
)

// a human and two AI wizards who all know each other; the dice always roll 0 (every chance hits)
func makeDiplomacyModel() (*GameModel, *playerlib.Player, *playerlib.Player, *playerlib.Player) {
    names := make(map[herolib.HeroType]string)
    human := playerlib.MakePlayer(setup.WizardCustom{Name: "Merlin", Banner: data.BannerRed}, true, 8, 8, names, nil)
    jafar := playerlib.MakePlayer(setup.WizardCustom{Name: "Jafar", Banner: data.BannerGreen}, false, 8, 8, names, nil)
    oberic := playerlib.MakePlayer(setup.WizardCustom{Name: "Oberic", Banner: data.BannerBlue}, false, 8, 8, names, nil)
    players := []*playerlib.Player{human, jafar, oberic}
    for _, one := range players {
        for _, other := range players {
            if one != other {
                one.AwarePlayer(other)
            }
        }
    }
    model := &GameModel{Players: players, Events: make(chan GameEvent, 100), TurnNumber: 10}
    model.DiplomacyRoll = func(int) int { return 0 }
    return model, human, jafar, oberic
}

func relation(player *playerlib.Player, other *playerlib.Player) *playerlib.Relationship {
    relationship, _ := player.GetDiplomaticRelation(other)
    return relationship
}

// set how player feels about other, and how keen they are on treaties and peace
func feel(player *playerlib.Player, other *playerlib.Player, visible int, treaty int, peace int) {
    relationship := relation(player, other)
    relationship.VisibleRelation = visible
    relationship.StartingRelation = visible
    relationship.TreatyInterest = treaty
    relationship.PeaceInterest = peace
}

func takeEvent[T GameEvent](model *GameModel) (T, bool) {
    var zero T
    for len(model.Events) > 0 {
        if event, ok := (<-model.Events).(T); ok {
            return event, true
        }
    }
    return zero, false
}

func TestAiDeclaresWar(test *testing.T) {
    model, human, jafar, oberic := makeDiplomacyModel()
    feel(jafar, human, -80, 0, 0)
    feel(jafar, oberic, 0, 0, 0)

    model.DoAiDiplomacy(jafar)
    if relation(jafar, human).Treaty != data.TreatyWar || relation(human, jafar).Treaty != data.TreatyWar {
        test.Errorf("a hated wizard should get a declaration of war")
    }
    if event, ok := takeEvent[*GameEventWarDeclared](model); !ok || event.Player != human || event.Wizard != jafar || event.Attacked {
        test.Errorf("the human should hear of it: %+v", event)
    }
    if relation(jafar, oberic).Treaty != data.TreatyNone {
        test.Errorf("a neutral relation shouldn't lead to war")
    }

    // allies don't declare war, however they feel
    model, human, jafar, _ = makeDiplomacyModel()
    jafar.AllianceWithPlayer(human)
    feel(jafar, human, -90, 0, 0)
    model.DoAiDiplomacy(jafar)
    if relation(jafar, human).Treaty != data.TreatyAlliance {
        test.Errorf("an ally shouldn't declare war")
    }
}

func TestAiProposesToTheHuman(test *testing.T) {
    model, human, jafar, oberic := makeDiplomacyModel()
    feel(jafar, human, 30, 60, 0)
    feel(jafar, oberic, 0, 0, 0)

    model.DoAiDiplomacy(jafar)
    proposal, ok := model.PendingProposals[human]
    if !ok || proposal.Wizard != jafar || proposal.Treaty != "pact" {
        test.Fatalf("a friendly wizard should propose a pact: %+v", proposal)
    }
    if event, ok := takeEvent[*GameEventTreatyProposal](model); !ok || event.Treaty != "pact" {
        test.Errorf("the proposal should come as an event")
    }
    if relation(jafar, human).TreatyInterest != 0 {
        test.Errorf("proposing uses up the interest")
    }
    view := model.makeDiplomacyViews(human)
    if len(view) == 0 || view[0].Proposal != "pact" {
        test.Errorf("the diplomacy view should show the proposal: %+v", view)
    }

    context := &CommandContext{Player: human}
    model.CurrentPlayer = 0
    if err := model.Execute(context, &AnswerProposalCommand{Accept: true}); err != nil {
        test.Fatalf("accept: %v", err)
    }
    if relation(human, jafar).Treaty != data.TreatyPact || relation(jafar, human).Treaty != data.TreatyPact {
        test.Errorf("accepting should make the pact on both sides")
    }
    if !jafar.AtPeaceWith(human) {
        test.Errorf("a pact keeps the peace")
    }
    if err := model.Execute(context, &AnswerProposalCommand{Accept: true}); err == nil {
        test.Errorf("no proposal is open any more")
    }

    // a proposal not answered by the end of the human's next turn lapses
    feel(oberic, human, 30, 60, 0)
    model.DoAiDiplomacy(oberic)
    model.TurnNumber += 1
    model.expireProposals(human)
    if _, ok := model.PendingProposals[human]; !ok {
        test.Errorf("the proposal is still open during the human's next turn")
    }
    model.TurnNumber += 1
    model.expireProposals(human)
    if _, ok := model.PendingProposals[human]; ok {
        test.Errorf("the proposal should have lapsed")
    }
}

// AI wizards answer each other: a pact between friends, a refusal from a wizard who doesn't care
func TestAiProposesToAi(test *testing.T) {
    model, human, jafar, oberic := makeDiplomacyModel()
    feel(jafar, human, 0, 0, 0)
    feel(jafar, oberic, 30, 60, 0)
    feel(oberic, jafar, 10, 30, 0)
    model.DoAiDiplomacy(jafar)
    if relation(oberic, jafar).Treaty != data.TreatyPact {
        test.Errorf("Oberic should accept a pact from a wizard he likes")
    }

    model, human, jafar, oberic = makeDiplomacyModel()
    feel(jafar, human, 0, 0, 0)
    feel(jafar, oberic, 30, 60, 0)
    feel(oberic, jafar, -20, 30, 0)
    model.DoAiDiplomacy(jafar)
    if relation(oberic, jafar).Treaty != data.TreatyNone || relation(jafar, oberic).StartingRelation != 28 {
        test.Errorf("a refused pact: treaty %v, relation %v", relation(oberic, jafar).Treaty, relation(jafar, oberic).StartingRelation)
    }

    // war, then peace once tempers cool
    model, human, jafar, oberic = makeDiplomacyModel()
    jafar.DeclareWar(oberic)
    feel(jafar, human, 0, 0, 0)
    feel(jafar, oberic, 0, 0, 60)
    feel(oberic, jafar, -10, 0, 40)
    model.DoAiDiplomacy(jafar)
    if relation(oberic, jafar).Treaty == data.TreatyWar || relation(oberic, jafar).PeaceCounter != 50 || !jafar.AtPeaceWith(oberic) {
        test.Errorf("peace should end the war: %+v", relation(oberic, jafar))
    }
}

// attacking a wizard you have a treaty with breaks it
func TestBreakTreatyOnAttack(test *testing.T) {
    model, human, jafar, _ := makeDiplomacyModel()
    human.ProposeTreaty(jafar, "pact")
    feel(jafar, human, 20, 0, 0)

    model.BreakTreatyOnAttack(human, jafar)
    if relation(human, jafar).Treaty != data.TreatyWar || relation(jafar, human).Treaty != data.TreatyWar {
        test.Errorf("attacking a pact partner should mean war")
    }
    if relation(jafar, human).VisibleRelation != -20 {
        test.Errorf("the victim should think much worse of the attacker: %v", relation(jafar, human).VisibleRelation)
    }
    if event, ok := takeEvent[*GameEventWarDeclared](model); !ok || !event.Attacked || event.Wizard != human {
        test.Errorf("the broken treaty should be announced: %+v", event)
    }

    // no treaty: no war, but the defender thinks a little worse of the attacker
    model, human, jafar, _ = makeDiplomacyModel()
    feel(jafar, human, 0, 0, 0)
    model.BreakTreatyOnAttack(human, jafar)
    if relation(jafar, human).Treaty != data.TreatyNone || len(model.Events) != 0 || relation(jafar, human).StartingRelation != -5 {
        test.Errorf("attacking without a treaty: treaty %v, relation %v", relation(jafar, human).Treaty, relation(jafar, human).StartingRelation)
    }
}

func TestPeaceRunsOut(test *testing.T) {
    _, human, jafar, _ := makeDiplomacyModel()
    human.ProposeTreaty(jafar, "peace")
    relationship := relation(jafar, human)
    for range 50 {
        relationship.UpdateTurn()
    }
    if relationship.PeaceCounter != 0 || jafar.AtPeaceWith(human) {
        test.Errorf("peace should last 50 turns: %v left", relationship.PeaceCounter)
    }
}
