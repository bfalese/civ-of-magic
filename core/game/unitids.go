package game

import (
    "fmt"

    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// Overland units get a number for command frontends: the views list it as a unit's "id" and
// commands refer to units by it, so a command still means the same unit after the stack changed
// order or split. A unit gets its id the first time it's asked for; ids are never reused in a
// game, but they aren't saved with it.
type unitIdRegistry struct {
    next int
    ids map[units.StackUnit]int
}

// the unit's id, assigned now if it has none yet
func (model *GameModel) UnitId(unit units.StackUnit) int {
    registry := &model.unitIds
    if registry.ids == nil {
        registry.ids = make(map[units.StackUnit]int)
    }
    if id, ok := registry.ids[unit]; ok {
        return id
    }
    registry.next += 1
    registry.ids[unit] = registry.next
    return registry.next
}

// one of the player's overland units by id, with its stack
func (model *GameModel) FindUnitById(player *playerlib.Player, id int) (units.StackUnit, *playerlib.UnitStack, error) {
    for _, stack := range player.Stacks {
        for _, unit := range stack.Units() {
            if known, ok := model.unitIds.ids[unit]; ok && known == id {
                return unit, stack, nil
            }
        }
    }
    return nil, nil, fmt.Errorf("you have no unit %v", id)
}

// forget units that no longer exist (dead, disbanded), so the registry doesn't grow forever
func (model *GameModel) pruneUnitIds() {
    if len(model.unitIds.ids) == 0 {
        return
    }
    alive := make(map[units.StackUnit]bool)
    for _, player := range model.Players {
        for unit := range player.Units() {
            alive[unit] = true
        }
    }
    for unit := range model.unitIds.ids {
        if !alive[unit] {
            delete(model.unitIds.ids, unit)
        }
    }
}
