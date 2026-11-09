package game

import (
    "errors"
    "fmt"

    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// Enchantments in the views (with who cast them and their upkeep), cancelling the player's own
// (cancel-enchantment), and the tax rates set-tax-rate takes.

type EnchantmentView struct {
    Name string `json:"name"`
    // mana per turn
    Upkeep int `json:"upkeep"`
    // the caster's banner, and whether it's the player (who may cancel it)
    Owner string `json:"owner,omitempty"`
    Yours bool `json:"yours,omitempty"`
}

// a tax rate set-tax-rate accepts: gold per citizen and the share of citizens it makes unrestful
type TaxRateView struct {
    Numerator int `json:"numerator"`
    Denominator int `json:"denominator"`
    // "1 1/2"
    Label string `json:"label"`
    Unrest float64 `json:"unrest"`
}

func makeTaxRates(player *playerlib.Player) []TaxRateView {
    rates := []TaxRateView{}
    rate := fraction.Zero()
    for {
        rates = append(rates, TaxRateView{Numerator: rate.Numerator, Denominator: max(1, rate.Denominator), Label: rate.NormalString(), Unrest: citylib.TaxUnrest(rate)})
        next, ok := player.NextTaxRate(rate)
        if !ok {
            return rates
        }
        rate = next
    }
}

func makeCityEnchantmentViews(city *citylib.City, player *playerlib.Player) []EnchantmentView {
    views := []EnchantmentView{}
    for _, enchantment := range city.Enchantments.Values() {
        views = append(views, EnchantmentView{
            Name: enchantment.Enchantment.Name(),
            Upkeep: enchantment.Enchantment.UpkeepMana(),
            Owner: enchantment.Owner.String(),
            Yours: enchantment.Owner == player.GetBanner(),
        })
    }
    return views
}

// cancel one of the player's enchantments: a global one (no city), or one the player cast on a
// city (theirs, or a curse on someone else's)
type CancelEnchantmentCommand struct {
    Enchantment string `json:"enchantment"`
    City *Location `json:"city,omitempty"`
}

func (command *CancelEnchantmentCommand) CommandType() string { return "cancel-enchantment" }

func (command *CancelEnchantmentCommand) Execute(context *CommandContext) error {
    model := context.Model
    player := context.Player

    if command.City == nil {
        enchantment := data.GetEnchantmentByName(command.Enchantment)
        if enchantment == data.EnchantmentNone || !player.GlobalEnchantments.Contains(enchantment) {
            return fmt.Errorf("you have no %q", command.Enchantment)
        }
        player.RemoveEnchantment(enchantment)
        for _, other := range model.Players {
            other.UpdateUnrest()
        }
        model.RefreshUI()
        return nil
    }

    city, _ := model.FindCity(command.City.X, command.City.Y, command.City.Plane)
    if city == nil {
        return errors.New("no city there")
    }
    for _, enchantment := range city.Enchantments.Values() {
        if enchantment.Enchantment.Name() != command.Enchantment {
            continue
        }
        if enchantment.Owner != player.GetBanner() {
            return fmt.Errorf("%v on %v isn't yours", command.Enchantment, city.Name)
        }
        city.RemoveEnchantments(enchantment.Enchantment)
        city.UpdateUnrest()
        model.RefreshUI()
        return nil
    }
    return fmt.Errorf("%v has no %q", city.Name, command.Enchantment)
}
