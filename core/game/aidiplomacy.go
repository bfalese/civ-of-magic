package game

import (
    "errors"
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// AI wizards start diplomacy on their turn (DoAiDiplomacy): with each wizard they know, from how
// they feel about them (the visible relation) and how keen they are on treaties or peace, they
// may declare war, propose peace, a wizard's pact or an alliance. Another AI answers at once
// (Player.WillAcceptTreaty); the human gets a treaty-proposal event and answers with
// answer-proposal before the end of their next turn. Treaties matter: the AI doesn't attack a
// wizard it keeps the peace with (Player.AtPeaceWith), and attacking such a wizard breaks the
// treaty (BreakTreatyOnAttack).
//
// Not in the original's full detail (which also weighs personality and objectives): these are
// thresholds and chances on the relation and the interests the core already tracks.

// a treaty an AI wizard proposed to the human, waiting for answer-proposal
type TreatyProposal struct {
    Wizard *playerlib.Player
    // "pact", "alliance" or "peace"
    Treaty string
    // the turn it was made; it lasts through the human's next turn
    Turn uint64
}

// an AI wizard proposes a treaty to the player (the human): answer with answer-proposal
type GameEventTreatyProposal struct {
    Player *playerlib.Player
    Wizard *playerlib.Player
    Treaty string
}

// a wizard declared war on the player, or broke a treaty by attacking them
type GameEventWarDeclared struct {
    Player *playerlib.Player
    Wizard *playerlib.Player
    // the treaty broke because the wizard attacked
    Attacked bool
}

// a roll in [0, n): rand.N, or the test's choice
func (model *GameModel) diplomacyRoll(n int) int {
    if model.DiplomacyRoll != nil {
        return model.DiplomacyRoll(n)
    }
    return rand.N(n)
}

// the chances, in percent per turn, that an AI wizard takes a step when the conditions hold
const (
    warChance = 15
    peaceChance = 25
    pactChance = 20
    allianceChance = 15
)

// the AI player's diplomacy for this turn, towards each wizard they know
func (model *GameModel) DoAiDiplomacy(player *playerlib.Player) {
    if player.IsHuman() || player.IsNeutral() || player.Defeated || player.Banished {
        return
    }
    for _, other := range model.Players {
        if other == player || other.IsNeutral() || other.Defeated || !player.IsAwareOf(other) {
            continue
        }
        relationship, _ := player.GetDiplomaticRelation(other)
        relation := relationship.VisibleRelation

        switch {
            case relationship.Treaty != data.TreatyWar && relationship.PeaceCounter == 0 && relationship.Treaty != data.TreatyAlliance &&
                 relation <= -40 && model.diplomacyRoll(100) < warChance:
                model.declareWar(player, other, false)

            case relationship.Treaty == data.TreatyWar && relation >= -30 && relationship.PeaceInterest >= 50 &&
                 model.diplomacyRoll(100) < peaceChance:
                model.proposeTreaty(player, other, "peace")

            case relationship.Treaty == data.TreatyNone && relationship.PeaceCounter == 0 && relation >= -10 && relationship.TreatyInterest >= 50 &&
                 model.diplomacyRoll(100) < pactChance:
                model.proposeTreaty(player, other, "pact")

            case relationship.Treaty == data.TreatyPact && relation >= 40 && relationship.TreatyInterest >= 70 &&
                 model.diplomacyRoll(100) < allianceChance:
                model.proposeTreaty(player, other, "alliance")
        }
    }
}

// the player offers other a treaty: an AI answers now, the human by command
func (model *GameModel) proposeTreaty(player *playerlib.Player, other *playerlib.Player, treaty string) {
    // proposing uses up the keenness; it comes back over the turns (Relationship.UpdateTurn)
    if relationship, ok := player.GetDiplomaticRelation(other); ok {
        if treaty == "peace" {
            relationship.PeaceInterest = 0
        } else {
            relationship.TreatyInterest = 0
        }
    }

    if other.IsHuman() {
        if model.PendingProposals == nil {
            model.PendingProposals = make(map[*playerlib.Player]*TreatyProposal)
        }
        model.PendingProposals[other] = &TreatyProposal{Wizard: player, Treaty: treaty, Turn: model.TurnNumber}
        select {
            case model.Events <- &GameEventTreatyProposal{Player: other, Wizard: player, Treaty: treaty}:
            default:
        }
        return
    }

    if other.WillAcceptTreaty(player, treaty) {
        player.ProposeTreaty(other, treaty)
    } else {
        // a refusal cools things a little
        player.AdjustDiplomaticRelation(other, -2)
    }
}

func (model *GameModel) declareWar(player *playerlib.Player, other *playerlib.Player, attacked bool) {
    player.DeclareWar(other)
    select {
        case model.Events <- &GameEventWarDeclared{Player: other, Wizard: player, Attacked: attacked}:
        default:
    }
}

// attacker starts a battle with defender: every battle sours the two wizards on each other a
// little; a pact, an alliance or peace between them ends in war, and the defender thinks much
// worse of the attacker
func (model *GameModel) BreakTreatyOnAttack(attacker *playerlib.Player, defender *playerlib.Player) {
    if attacker == nil || defender == nil || attacker == defender || attacker.IsNeutral() || defender.IsNeutral() {
        return
    }
    defender.AdjustDiplomaticRelation(attacker, -5)
    attacker.AdjustDiplomaticRelation(defender, -2)
    if !attacker.AtPeaceWith(defender) && !defender.AtPeaceWith(attacker) {
        return
    }
    defender.AdjustDiplomaticRelation(attacker, -35)
    if relationship, ok := defender.GetDiplomaticRelation(attacker); ok {
        relationship.VisibleRelation = max(-100, relationship.VisibleRelation - 40)
    }
    model.declareWar(attacker, defender, true)
}

// the human's answer to the open proposal: accept makes the treaty, decline cools the proposer
func (model *GameModel) AnswerProposal(player *playerlib.Player, accept bool) error {
    proposal, ok := model.PendingProposals[player]
    if !ok {
        return errors.New("no treaty proposal is open")
    }
    delete(model.PendingProposals, player)
    if !accept {
        proposal.Wizard.AdjustDiplomaticRelation(player, -5)
        return nil
    }
    if proposal.Treaty == "alliance" {
        if relationship, ok := player.GetDiplomaticRelation(proposal.Wizard); ok && relationship.Treaty == data.TreatyWar {
            return errors.New("you are at war with them")
        }
    }
    return proposal.Wizard.ProposeTreaty(player, proposal.Treaty)
}

// proposals the human didn't answer by the end of their turn lapse (called at the start of the
// human's turn: one made during the last round of AI turns is still open)
func (model *GameModel) expireProposals(player *playerlib.Player) {
    if proposal, ok := model.PendingProposals[player]; ok && proposal.Turn + 1 < model.TurnNumber {
        delete(model.PendingProposals, player)
    }
}
