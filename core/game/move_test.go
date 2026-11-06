package game

import (
    "image"
    "testing"

    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/pathfinding"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// records what the model asked the frontend to do
type fakeMoveHandlers struct {
    steps int
    shown int
    nextUnit int
    battles int
    battleResult combat.CombatState
}

func (handlers *fakeMoveHandlers) ShowMovement(x int, y int, stack *playerlib.UnitStack, center bool) {
    handlers.shown += 1
}

func (handlers *fakeMoveHandlers) DoEncounter(player *playerlib.Player, stack *playerlib.UnitStack, encounter *maplib.ExtraEncounter, map_ *maplib.Map, x int, y int) combat.CombatState {
    return combat.CombatStateAttackerWin
}

func (handlers *fakeMoveHandlers) DoCombat(player *playerlib.Player, stack *playerlib.UnitStack, enemy *playerlib.Player, enemyStack *playerlib.UnitStack, zone combat.ZoneType) combat.CombatState {
    handlers.battles += 1
    return handlers.battleResult
}

func (handlers *fakeMoveHandlers) DefeatCity(player *playerlib.Player, stack *playerlib.UnitStack, enemy *playerlib.Player, city *citylib.City) (bool, int) {
    return false, 0
}

func (handlers *fakeMoveHandlers) DiscoverWizards() bool {
    return false
}

func (handlers *fakeMoveHandlers) ConfirmEncounter(encounter *maplib.ExtraEncounter) bool {
    return true
}

func (handlers *fakeMoveHandlers) SteppedStack() {
    handlers.steps += 1
}

func (handlers *fakeMoveHandlers) NextUnit(player *playerlib.Player) {
    handlers.nextUnit += 1
}

// a spearman (one move) with a three step path walks one step and keeps the rest of the path
func TestMoveSelectedStackHeadless(test *testing.T) {
    model, player, _ := makeBattleTestModel()

    unit := units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 1, 6, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
    player.AddUnit(unit)
    stack := player.FindStack(1, 6, data.PlaneArcanus)
    stack.CurrentPath = pathfinding.Path{image.Pt(2, 6), image.Pt(3, 6), image.Pt(4, 6)}
    player.SelectedStack = stack

    handlers := &fakeMoveHandlers{}
    model.MoveSelectedStack(handlers, player)

    if stack.X() != 2 || stack.Y() != 6 {
        test.Errorf("stack should be at 2,6, is at %v,%v", stack.X(), stack.Y())
    }
    if len(stack.CurrentPath) != 2 {
        test.Errorf("two steps of the path should be left, got %v", stack.CurrentPath)
    }
    if handlers.shown != 1 || handlers.steps != 1 {
        test.Errorf("one step should be shown, got shown=%v steps=%v", handlers.shown, handlers.steps)
    }
    if handlers.nextUnit != 1 {
        test.Errorf("next unit should be selected once the stack is out of moves, got %v", handlers.nextUnit)
    }
    if player.MovedStacksThisTurn != 1 {
        test.Errorf("moved stacks counter should be 1, got %v", player.MovedStacksThisTurn)
    }
}

// walking onto an enemy stack starts a battle and ends the move
func TestMoveSelectedStackAttacks(test *testing.T) {
    model, attacker, _ := makeBattleTestModel()
    stack := attacker.FindStack(3, 3, data.PlaneArcanus)
    stack.CurrentPath = pathfinding.Path{image.Pt(4, 3), image.Pt(5, 3)}
    attacker.SelectedStack = stack

    handlers := &fakeMoveHandlers{battleResult: combat.CombatStateAttackerFlee}
    model.MoveSelectedStack(handlers, attacker)

    if handlers.battles != 1 {
        test.Fatalf("one battle should be fought, got %v", handlers.battles)
    }
    if stack.X() != 3 || stack.Y() != 3 {
        test.Errorf("a fleeing attacker should step back to 3,3, is at %v,%v", stack.X(), stack.Y())
    }
    if len(stack.CurrentPath) != 0 {
        test.Errorf("the path should be cleared after a battle, got %v", stack.CurrentPath)
    }
    if !stack.GetRemainingMoves().IsZero() {
        test.Errorf("the stack should be out of moves after a battle")
    }
}
