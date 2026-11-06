package game

import (
    "image"
    "testing"

    "github.com/bfalese/civ-of-magic/core/ai"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/pathfinding"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// AI decisions are applied and the AI's stacks move without a frontend; the battle goes
// through the handlers
func TestApplyAiDecisionsAndMoveHeadless(test *testing.T) {
    model, attacker, _ := makeBattleTestModel()
    attacker.AIBehavior = ai.MakeEnemyAI()
    stack := attacker.FindStack(3, 3, data.PlaneArcanus)

    research := spellbook.Spell{Name: "Test Research", ResearchCost: 10}
    decisions := []playerlib.AIDecision{
        &playerlib.AIResearchSpellDecision{Spell: research},
        &playerlib.AIMoveStackDecision{Stack: stack, Path: pathfinding.Path{image.Pt(4, 3)}},
    }

    names := func(race data.Race) string {
        return "Test Town"
    }
    model.ApplyAiDecisions(attacker, decisions, names)

    if attacker.ResearchingSpell.Name != "Test Research" {
        test.Errorf("research decision should set the researched spell, got %v", attacker.ResearchingSpell.Name)
    }
    if len(stack.CurrentPath) != 1 {
        test.Fatalf("move decision should set the stack's path, got %v", stack.CurrentPath)
    }

    handlers := &fakeMoveHandlers{battleResult: combat.CombatStateAttackerFlee}
    model.MoveAiStacks(handlers, attacker)

    if handlers.battles != 1 {
        test.Errorf("moving onto the enemy stack should start one battle, got %v", handlers.battles)
    }
    if stack.X() != 3 || stack.Y() != 3 {
        test.Errorf("a fleeing attacker should step back to 3,3, is at %v,%v", stack.X(), stack.Y())
    }
}

// a full AI turn (decide, act, move) runs headless
func TestDoAiTurnHeadless(test *testing.T) {
    model, _, enemy := makeBattleTestModel()
    enemy.AIBehavior = ai.MakeEnemyAI()

    handlers := &fakeMoveHandlers{battleResult: combat.CombatStateAttackerWin}
    names := func(race data.Race) string {
        return "Test Town"
    }

    for range 3 {
        model.DoAiTurn(handlers, enemy, names)
    }

    if len(enemy.Stacks) != 1 {
        test.Errorf("the AI should still have its stack, has %v", len(enemy.Stacks))
    }
}
