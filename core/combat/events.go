package combat

import (
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

type CombatState int

const (
    CombatStateRunning CombatState = iota
    CombatStateAttackerWin
    CombatStateDefenderWin
    CombatStateAttackerFlee
    CombatStateDefenderFlee
    CombatStateNoCombat
)

func (state CombatState) IsWinner(team Team) bool {
    switch state {
        case CombatStateAttackerWin: return team == TeamAttacker
        case CombatStateDefenderWin: return team == TeamDefender
        case CombatStateAttackerFlee: return team == TeamDefender
        case CombatStateDefenderFlee: return team == TeamAttacker
    }

    return false
}

func (state CombatState) String() string {
    switch state {
        case CombatStateRunning: return "Running"
        case CombatStateAttackerWin: return "AttackerWin"
        case CombatStateDefenderWin: return "DefenderWin"
        case CombatStateAttackerFlee: return "AttackerFlee"
        case CombatStateDefenderFlee: return "DefenderFlee"
        case CombatStateNoCombat: return "NoCombat"
    }

    return ""
}

type CombatEvent interface {
}

type CombatEventSelectTile struct {
    SelectTile func(int, int)
    CanTarget func(int, int) bool
    Spell spellbook.Spell
    Selecter Team
}

type CombatEventNextUnit struct {
}

type CombatEventGlobalSpell struct {
    Caster ArmyPlayer
    Magic data.MagicType
    Name string
}

type CombatPlaySound struct {
    Sound int
}

type CombatEventSelectUnit struct {
    SelectTarget func(*ArmyUnit)
    CanTarget func(*ArmyUnit) bool
    Spell spellbook.Spell
    Selecter Team
    SelectTeam Team
}

type CombatSelectTargets struct {
    Title string
    Targets []*ArmyUnit
    Select func (*ArmyUnit)
    Army *Army
}

type CombatEventSummonUnit struct {
    Unit *ArmyUnit
}

type CombatEventMessage struct {
    Message string
}

type CombatCreateWallOfFire struct {
}

type CombatCreateWallOfDarkness struct {
}

type CombatDoSingleAuto struct {
}

// FIXME: kind of ugly to need a specific event like this for one projectile type
type CombatEventCreateLightningBolt struct {
    Target *ArmyUnit
    Strength int
}

type CombatUpdates struct {
    SingleAuto bool
}

// which units a targeted spell may pick
type Targeting int
const (
    TargetFriend Targeting = iota
    TargetEnemy
    TargetEither
)

// Apply the model-side effect of a combat event, for code without a battle screen (Run, battle
// sessions driven by commands). The Ebitengine battle screen does the same in its event loop,
// plus the animations. Returns false for events the caller has to answer: a human army choosing
// the target of a spell (CombatEventSelectUnit, CombatEventSelectTile, CombatSelectTargets).
// Projectiles created here still need resolving (e.g. ProxyActions.DoProjectiles).
func (model *CombatModel) ApplyEvent(spellSystem SpellSystem, event CombatEvent) bool {
    switch event.(type) {
        case *CombatEventNextUnit:
            model.NextUnit()
        case *CombatEventCreateLightningBolt:
            bolt := event.(*CombatEventCreateLightningBolt)
            model.AddProjectile(spellSystem.CreateLightningBoltProjectile(bolt.Target, bolt.Strength))
        case *CombatCreateWallOfFire:
            CreateWallOfFire(model.Tiles, TownCenterX, TownCenterY, 4, 0)
        case *CombatCreateWallOfDarkness:
            CreateWallOfDarkness(model.Tiles, TownCenterX, TownCenterY, 4, 0)
        case *CombatSelectTargets:
            choose := event.(*CombatSelectTargets)
            if !choose.Army.IsAI() {
                return false
            }
            // an AI (or auto) army picks one of the targets at random, as the battle screen does
            if len(choose.Targets) > 0 {
                choose.Select(choose.Targets[rand.N(len(choose.Targets))])
            }
        case *CombatEventSelectUnit, *CombatEventSelectTile:
            return false
    }
    return true
}
