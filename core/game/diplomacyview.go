package game

import (
    "cmp"
    "slices"

    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// The diplomacy screen's data: the wizards the player knows, how they feel about the player and
// what can be offered to them (propose-treaty, offer-tribute, trade-spells).

type DiplomacyView struct {
    Wizard PlayerRef `json:"wizard"`
    // none, pact, alliance or war
    Treaty string `json:"treaty"`
    // how they feel about you: -100 (hate) to 100 (harmony), and its name
    Relation int `json:"relation"`
    Mood string `json:"mood"`
    // turns of peace left after a peace treaty (they won't attack meanwhile)
    PeaceTurns int `json:"peace-turns,omitempty"`
    Banished bool `json:"banished,omitempty"`
    // what a tribute of gold would give them now (0: too little gold)
    TributeGold int `json:"tribute-gold"`
    // your spells they don't know: what offer-tribute's spell and trade-spells take
    Spells []string `json:"spells"`
    // they know spells you don't (a trade can get you one)
    CanTeach bool `json:"can-teach"`
    // a trade they offered that waits for answer-trade
    TradeOffer *TradeOfferView `json:"trade-offer,omitempty"`
    // their global enchantments
    GlobalEnchantments []string `json:"global-enchantments"`
    Fame int `json:"fame"`
    Cities int `json:"cities"`
}

type TradeOfferView struct {
    Give string `json:"give"`
    Receive string `json:"receive"`
}

func (model *GameModel) makeDiplomacyViews(player *playerlib.Player) []DiplomacyView {
    views := []DiplomacyView{}
    for _, other := range model.Players {
        if other == player || !player.IsAwareOf(other) || other.Defeated {
            continue
        }
        view := DiplomacyView{
            Wizard: *MakePlayerRef(other),
            Treaty: "none",
            Banished: other.Banished,
            TributeGold: player.TributeGoldAmount(),
            Spells: []string{},
            GlobalEnchantments: []string{},
            Fame: other.Fame,
            Cities: len(other.Cities),
        }
        if relation, ok := player.GetDiplomaticRelation(other); ok {
            view.Treaty = relation.Treaty.String()
        }
        if relation, ok := other.GetDiplomaticRelation(player); ok {
            view.Relation = relation.VisibleRelation
            view.Mood = relation.Description()
            view.PeaceTurns = relation.PeaceCounter
        }
        for _, spell := range player.KnownSpells.Spells {
            if !other.KnownSpells.Contains(spell) {
                view.Spells = append(view.Spells, spell.Name)
            }
        }
        slices.SortFunc(view.Spells, cmp.Compare)
        for _, spell := range other.KnownSpells.Spells {
            if !player.KnownSpells.Contains(spell) {
                view.CanTeach = true
                break
            }
        }
        if offer, ok := model.PendingTrades[player]; ok && offer.Wizard == other {
            view.TradeOffer = &TradeOfferView{Give: offer.Give.Name, Receive: offer.Receive.Name}
        }
        for _, enchantment := range other.GlobalEnchantments.Values() {
            view.GlobalEnchantments = append(view.GlobalEnchantments, enchantment.String())
        }
        slices.SortFunc(view.GlobalEnchantments, cmp.Compare)
        views = append(views, view)
    }
    return views
}
