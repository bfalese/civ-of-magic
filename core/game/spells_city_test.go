package game

import (
    "testing"

    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/lib/set"
)

func TestApplyConsecrationRemovesOnlyCurses(test *testing.T) {
    city := &citylib.City{Enchantments: set.MakeSet[citylib.Enchantment]()}
    city.AddEnchantment(data.CityEnchantmentFamine, data.BannerGreen)
    city.AddEnchantment(data.CityEnchantmentChaosRift, data.BannerGreen)
    city.AddEnchantment(data.CityEnchantmentWallOfFire, data.BannerRed)

    model := &GameModel{}
    model.ApplyConsecration(city)

    if city.HasEnchantment(data.CityEnchantmentFamine) || city.HasEnchantment(data.CityEnchantmentChaosRift) {
        test.Errorf("curses should be removed")
    }
    if !city.HasEnchantment(data.CityEnchantmentWallOfFire) {
        test.Errorf("non-curse enchantments must stay")
    }
}
