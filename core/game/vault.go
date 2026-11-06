package game

import (
    "errors"
    "fmt"
    "slices"

    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// The vault and hero equipment, as model operations. The Ebitengine vault screen keeps an item
// "in hand" while the player drags it around; command frontends name the slots instead.
//
// Items arriving at the vault (merchant, Create Artifact, dead heroes' gear, gifts) are kept in
// PendingItems until placed with MoveItem or destroyed with DestroyItem.

// a place an item can be: a vault slot, a hero's equipment slot, or a newly arrived item
type ItemSlot struct {
    // index 0-3 into the vault, if set
    Vault *int `json:"vault,omitempty"`
    // index 0-5 into the player's heroes, with Slot the equipment slot, if set
    Hero *int `json:"hero,omitempty"`
    Slot int `json:"slot,omitempty"`
    // index into the player's newly arrived items, if set
    New *int `json:"new,omitempty"`
}

// send a vault event and keep the item as newly arrived for the player
func (model *GameModel) SendVaultItem(event *GameEventVault) {
    model.recordVaultItem(event)
    select {
        case model.Events <- event:
        default:
    }
}

func (model *GameModel) recordVaultItem(event *GameEventVault) {
    if event.CreatedArtifact == nil || event.Player == nil {
        return
    }
    if model.PendingItems == nil {
        model.PendingItems = make(map[*playerlib.Player][]*artifact.Artifact)
    }
    model.PendingItems[event.Player] = append(model.PendingItems[event.Player], event.CreatedArtifact)
}

// where an item in a slot is on the map: heroes carry theirs, the vault is at the fortress
func (model *GameModel) slotLocation(player *playerlib.Player, slot ItemSlot) data.PlanePoint {
    if slot.Hero != nil {
        return player.Heroes[*slot.Hero].GetPlanePoint()
    }
    if fortress := player.FindFortressCity(); fortress != nil {
        return fortress.GetPlanePoint()
    }
    return data.PlanePoint{}
}

// a pointer to the item in a slot, checked for the player
func (model *GameModel) itemSlot(player *playerlib.Player, slot ItemSlot) (**artifact.Artifact, error) {
    set := 0
    if slot.Vault != nil { set += 1 }
    if slot.Hero != nil { set += 1 }
    if slot.New != nil { set += 1 }
    if set != 1 {
        return nil, errors.New("give exactly one of vault, hero or new")
    }

    switch {
        case slot.Vault != nil:
            if *slot.Vault < 0 || *slot.Vault >= len(player.VaultEquipment) {
                return nil, fmt.Errorf("no vault slot %v", *slot.Vault)
            }
            return &player.VaultEquipment[*slot.Vault], nil
        case slot.Hero != nil:
            if *slot.Hero < 0 || *slot.Hero >= len(player.Heroes) || player.Heroes[*slot.Hero] == nil {
                return nil, fmt.Errorf("no hero %v", *slot.Hero)
            }
            hero := player.Heroes[*slot.Hero]
            if slot.Slot < 0 || slot.Slot >= len(hero.GetArtifactSlots()) {
                return nil, fmt.Errorf("%v has no slot %v", hero.GetName(), slot.Slot)
            }
            return &hero.Equipment[slot.Slot], nil
        default:
            pending := model.PendingItems[player]
            if *slot.New < 0 || *slot.New >= len(pending) {
                return nil, fmt.Errorf("no new item %v", *slot.New)
            }
            return &pending[*slot.New], nil
    }
}

// whether an item fits a slot (hero slots take certain item types)
func (model *GameModel) itemFits(player *playerlib.Player, slot ItemSlot, item *artifact.Artifact) bool {
    if item == nil || slot.Hero == nil {
        return true
    }
    return player.Heroes[*slot.Hero].GetArtifactSlots()[slot.Slot].CompatibleWith(item.Type)
}

// drop emptied entries from the newly arrived items
func (model *GameModel) compactPendingItems(player *playerlib.Player) {
    model.PendingItems[player] = slices.DeleteFunc(model.PendingItems[player], func(item *artifact.Artifact) bool {
        return item == nil
    })
}

// swap the items in two slots. An item that arrives at a hero somewhere else than where it was
// costs VaultTeleportCost mana (as in the vault screen). A newly arrived item can only be swapped
// into the vault or a hero.
func (model *GameModel) MoveItem(player *playerlib.Player, from ItemSlot, to ItemSlot) error {
    fromItem, err := model.itemSlot(player, from)
    if err != nil {
        return err
    }
    toItem, err := model.itemSlot(player, to)
    if err != nil {
        return err
    }
    if fromItem == toItem {
        return nil
    }
    if *fromItem == nil {
        return errors.New("no item there")
    }
    if to.New != nil {
        return errors.New("items can't be moved back to the new items")
    }
    if from.New != nil && *toItem != nil {
        return errors.New("place a new item in an empty slot")
    }

    if !model.itemFits(player, to, *fromItem) || !model.itemFits(player, from, *toItem) {
        return errors.New("the item doesn't fit that slot")
    }

    fromLocation := model.slotLocation(player, from)
    toLocation := model.slotLocation(player, to)
    cost := 0
    if to.Hero != nil && fromLocation != toLocation {
        cost += playerlib.VaultTeleportCost
    }
    if from.Hero != nil && *toItem != nil && fromLocation != toLocation {
        cost += playerlib.VaultTeleportCost
    }
    if cost > player.Mana {
        return errors.New("Not enough mana to teleport item")
    }

    player.Mana -= cost
    *fromItem, *toItem = *toItem, *fromItem
    if from.New != nil {
        model.compactPendingItems(player)
    }
    return nil
}

// destroy the item in a slot for mana (see Player.ArtifactManaValue); returns the mana gained
func (model *GameModel) DestroyItem(player *playerlib.Player, slot ItemSlot) (int, error) {
    item, err := model.itemSlot(player, slot)
    if err != nil {
        return 0, err
    }
    if *item == nil {
        return 0, errors.New("no item there")
    }

    gained := player.ArtifactManaValue(*item)
    player.Mana += gained
    *item = nil
    if slot.New != nil {
        model.compactPendingItems(player)
    }
    return gained, nil
}

// forget a newly arrived item (the Ebitengine vault screen placed it)
func (model *GameModel) ClearPendingItem(player *playerlib.Player, item *artifact.Artifact) {
    pending := model.PendingItems[player]
    if index := slices.Index(pending, item); index >= 0 {
        model.PendingItems[player] = slices.Delete(pending, index, index + 1)
    }
}
