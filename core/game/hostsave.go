package game

import (
    "encoding/json"
    "errors"
    "fmt"

    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// A saved GameHost game: the model as the desktop saves it (SerializeModel), plus what the host
// keeps outside the players: which player is the human, a finished spell still waiting for its
// target, and items that arrived at the vault but aren't placed yet. Open offers and trades are
// not kept (they last until the next turn anyway), nor unit ids (reassigned on load; they are only
// stable within a session).
type HostSave struct {
    Version int `json:"version"`
    Game SerializedGame `json:"game"`
    Human data.BannerType `json:"human"`
    // the spell waiting for a target (choose-spell-target), by name
    PendingSpell string `json:"pending-spell,omitempty"`
    // the human's new items (the vault's "new" list)
    PendingItems []artifact.SerializedArtifact `json:"pending-items,omitempty"`
}

const HostSaveVersion = 1

// the game as JSON (HostSave). Only between commands: not while a battle, an encounter or a
// captured city waits for an answer.
func (host *GameHost) Save(name string) ([]byte, error) {
    if waiting := host.Session.WaitingFor(); waiting != WaitingNothing {
        return nil, fmt.Errorf("can't save while waiting for %v", waiting)
    }
    model := host.Model

    save := HostSave{
        Version: HostSaveVersion,
        Game: SerializeModel(model, name),
        Human: host.Human.GetBanner(),
    }
    if spell, ok := model.PendingSpells[host.Human]; ok {
        save.PendingSpell = spell.Name
    }
    for _, item := range model.PendingItems[host.Human] {
        save.PendingItems = append(save.PendingItems, artifact.SerializeArtifact(item))
    }
    return json.Marshal(save)
}

// a host for a saved game (GameHost.Save), with the data tables it was played with. The human's
// turn goes on where it was saved; the frontend asks for the views again. A finished spell that
// still needs a target is announced again (choose-spell-target), as is the research question when
// nothing is being researched.
func LoadGameHost(gameData *gamedata.GameData, encoded []byte) (*GameHost, error) {
    var save HostSave
    if err := json.Unmarshal(encoded, &save); err != nil {
        return nil, fmt.Errorf("save: %v", err)
    }
    if save.Version != HostSaveVersion {
        return nil, fmt.Errorf("save version %v, expected %v", save.Version, HostSaveVersion)
    }
    if len(save.Game.Players) == 0 {
        return nil, errors.New("the save has no players")
    }

    events := make(chan GameEvent, 1000)
    model := MakeModelFromSerialized(&save.Game, events, gameData.HeroNames, gameData.Spells, gameData.ArtifactCatalog(), gameData.Buildings, gamedata.TerrainData())
    ai := wrapHeadlessModel(model, gameData)

    var human *playerlib.Player
    for _, player := range model.Players {
        if player.GetBanner() == save.Human {
            human = player
        }
    }
    if human == nil {
        return nil, fmt.Errorf("no player with the human's banner %v", save.Human)
    }
    if model.CurrentPlayer < 0 || model.CurrentPlayer >= len(model.Players) || model.Players[model.CurrentPlayer] != human {
        return nil, errors.New("the save isn't at the human's turn")
    }

    host := makeHost(ai, human)

    for _, item := range save.PendingItems {
        model.recordVaultItem(&GameEventVault{CreatedArtifact: artifact.ReconstructArtifact(&item, model.AllSpells), Player: human})
    }
    if save.PendingSpell != "" {
        spell := model.AllSpells.FindByName(save.PendingSpell)
        if !spell.Valid() {
            return nil, fmt.Errorf("unknown pending spell %q", save.PendingSpell)
        }
        // pending again and announced, as when it finished
        model.FinishCast(human, spell)
    }
    if !human.ResearchingSpell.Valid() && len(human.ResearchCandidateSpells.Spells) > 0 {
        select {
            case model.Events <- &GameEventResearchSpell{Player: human}:
            default:
        }
    }
    host.drainEvents()
    return host, nil
}
