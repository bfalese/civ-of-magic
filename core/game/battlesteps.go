package game

import (
    "slices"

    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/pathfinding"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// What happened in a battle, step by step, so a frontend can replay it: the core resolves moves
// and attacks at once, and the view after a command only shows the result.
type BattleStepView struct {
    // "move", "teleport", "melee", "ranged", "spell" (cast by your wizard or unit), or "effect"
    // (health changed otherwise: the AI's spells, a wall of fire, a magic vortex, ...)
    Kind string `json:"kind"`
    Unit *uint64 `json:"unit,omitempty"`
    // move: the tiles walked, in order; teleport: the destination
    Path [][2]int `json:"path,omitempty"`
    // melee / ranged: the unit attacked
    Target *uint64 `json:"target,omitempty"`
    Spell string `json:"spell,omitempty"`
    // the health this step changed, counterattacks included
    Changes []BattleHealthChange `json:"changes,omitempty"`
}

type BattleHealthChange struct {
    Unit uint64 `json:"unit"`
    // health lost; negative when healed
    Damage int `json:"damage"`
    Killed bool `json:"killed,omitempty"`
}

// the battle's actions (combat.ProxyActions) recording each move and attack, with the health it
// changed for every unit
type battleRecorder struct {
    *combat.ProxyActions
    steps []BattleStepView
    health map[*combat.ArmyUnit]int
}

func makeBattleRecorder(model *combat.CombatModel) *battleRecorder {
    recorder := &battleRecorder{ProxyActions: &combat.ProxyActions{Model: model}}
    recorder.snapshot()
    return recorder
}

func (recorder *battleRecorder) snapshot() {
    recorder.health = make(map[*combat.ArmyUnit]int)
    for _, army := range []*combat.Army{recorder.Model.AttackingArmy, recorder.Model.DefendingArmy} {
        for _, unit := range army.GetUnits() {
            recorder.health[unit] = max(0, unit.GetHealth())
        }
    }
}

// the health changes since the last snapshot; takes a new one
func (recorder *battleRecorder) changes() []BattleHealthChange {
    var changes []BattleHealthChange
    for unit, before := range recorder.health {
        now := max(0, unit.GetHealth())
        if now != before {
            changes = append(changes, BattleHealthChange{Unit: unit.Id, Damage: before - now, Killed: now == 0})
        }
    }
    slices.SortFunc(changes, func(a BattleHealthChange, b BattleHealthChange) int { return int(a.Unit) - int(b.Unit) })
    recorder.snapshot()
    return changes
}

func (recorder *battleRecorder) add(step BattleStepView) {
    step.Changes = recorder.changes()
    recorder.steps = append(recorder.steps, step)
}

// health that changed outside the recorded actions becomes an "effect" step
func (recorder *battleRecorder) flush() {
    changes := recorder.changes()
    if len(changes) > 0 {
        recorder.steps = append(recorder.steps, BattleStepView{Kind: "effect", Changes: changes})
    }
}

// the steps since the last reset
func (recorder *battleRecorder) current() []BattleStepView {
    recorder.flush()
    if recorder.steps == nil {
        return []BattleStepView{}
    }
    return recorder.steps
}

func (recorder *battleRecorder) reset() {
    recorder.flush()
    recorder.steps = nil
}

func unitIdRef(unit *combat.ArmyUnit) *uint64 {
    if unit == nil {
        return nil
    }
    id := unit.Id
    return &id
}

// as ProxyActions.MoveUnit (without the network synchronization), noting each tile; the unit
// turns to each tile it walks to (as the Ebitengine battle screen does)
func (recorder *battleRecorder) MoveUnit(mover *combat.ArmyUnit, path pathfinding.Path) {
    recorder.flush()
    walked := [][2]int{}
    for len(path) > 0 && mover.MovesLeft.GreaterThan(fraction.FromInt(0)) {
        mover.Facing = combat.FaceTowards(mover.X, mover.Y, path[0].X, path[0].Y)
        died := recorder.Model.MoveUnit(mover, path[0].X, path[0].Y)
        walked = append(walked, [2]int{mover.X, mover.Y})
        if died {
            break
        }
        path = path[1:]
    }
    mover.CurrentPath = nil
    recorder.add(BattleStepView{Kind: "move", Unit: unitIdRef(mover), Path: walked})
}

func (recorder *battleRecorder) Teleport(unit *combat.ArmyUnit, x int, y int, merge bool) {
    recorder.flush()
    recorder.ProxyActions.Teleport(unit, x, y, merge)
    recorder.add(BattleStepView{Kind: "teleport", Unit: unitIdRef(unit), Path: [][2]int{{unit.X, unit.Y}}})
}

func (recorder *battleRecorder) MeleeAttack(attacker *combat.ArmyUnit, defender *combat.ArmyUnit) {
    recorder.flush()
    attacker.Facing = combat.FaceTowards(attacker.X, attacker.Y, defender.X, defender.Y)
    defender.Facing = combat.FaceTowards(defender.X, defender.Y, attacker.X, attacker.Y)
    recorder.ProxyActions.MeleeAttack(attacker, defender)
    recorder.add(BattleStepView{Kind: "melee", Unit: unitIdRef(attacker), Target: unitIdRef(defender)})
}

func (recorder *battleRecorder) RangeAttack(attacker *combat.ArmyUnit, defender combat.RangeTarget) {
    recorder.flush()
    attacker.Facing = combat.FaceTowards(attacker.X, attacker.Y, defender.GetX(), defender.GetY())
    recorder.ProxyActions.RangeAttack(attacker, defender)
    step := BattleStepView{Kind: "ranged", Unit: unitIdRef(attacker)}
    if target, ok := defender.(*combat.ArmyUnit); ok {
        step.Target = unitIdRef(target)
    }
    recorder.add(step)
}
