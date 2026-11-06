# Handoff 011: city economy, offers, planar travel and Awareness move to core

Date: 2026-11-06
Previous: [010](010-watch-mode-and-game-go-split-2.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13 and #14 are merged into `master`.
- This round: branch `game-go-split-3`, PR #15 against `master`. Waiting for the user's smoke
  test. **Don't merge without explicit approval.**

## What changed

| Where | What moved | UI keeps |
|---|---|---|
| `core/city/economy.go` | `CanSellBuilding`, `SellValue`, `SellBuilding`, `BuyCost`, `SetFarmers`, `WorkersToFarmers`, `WasBuildingReplaced`, and the city screen's resource breakdowns (`FoodProducers`, `WorkProducers`, `BuildingMaintenanceResources`, `GoldProducers`, `PowerProducers`, `ResearchProducers`) with `ResourceUsage` | drawing, rubble, confirm dialogs (`cityview.ResourceUsage` is now an alias) |
| `core/player` | `SellBuilding`, `BuyProduction` (gold changes hands), `ArtifactManaValue`, `VaultTeleportCost` | vault screen |
| `core/game/offers.go` | `HireHero`, `DeclineHero`, `HireMercenaries`, `BuyMerchantItem` | offer screens, hero name input |
| `core/game/planes.go` | `PlanarTravel` (was `doPlanarTraval`), `ExploreFogForAwareness` | plane button |
| `core/game/spells.go` | `ApplyGlobalEnchantment` now handles Awareness and Nature Awareness | |

Line counts: `city-screen.go` 2766 → 2559, `game.go` 6035 → 5950.

Small differences, none visible:
- Hiring a hero: the name prompt now comes after the stack is resolved and selected (it came
  between). The name doesn't affect either.
- Buying production recomputes the price when confirmed instead of reusing the value from when
  the screen opened (same state, same result).
- A worker → farmer click and a farmer click now both keep `Workers = Citizens - Rebels - Farmers`.

New tests: selling, buy cost, citizen assignment (core/city), hiring mercenaries and heroes,
artifact mana value, Nature Awareness (core/game).

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass.
- CI guard (`go list -deps ./core/... | grep -E 'ebiten|/lbx'`): empty.
- `-watch` headless, 6 min: 134 turns, battles, 19 spells researched, no errors.

## Smoke test (by hand)

City screen: sell a building (gold, rubble, once per turn), buy production (price, gold), click
farmers/workers. Hire a hero (name prompt) and mercenaries, buy from the merchant, destroy an item
in the vault for mana, move an item to a hero elsewhere (20 mana). Planar travel through a tower
and with Plane Shift; cast Awareness / Nature Awareness. Plus 010's list if not done yet.

## What's left in the UI

`game.go` and `cast.go` are now nearly all screens. What remains model-ish is small:
`castGlobalEnchantment`, `doSummonHero`, `doIncarnation`, `doSummonUnit` (they mostly emit events
that start animations), `ActiveConjunctionName` and `StartingCityEvent` (kept on purpose, see 008).

## Next

1. **Headless AI-vs-AI game loop** in core (`DoNextTurn` + `DoAiTurn` + `Battle.Run`). It needs
   building infos, spells, hero names and city names without LBX, so it overlaps with the "drop
   LBX" goal: export those tables to YAML/JSON once (from the user's data, never committed if
   copyrighted; or hand-written replacements) and load them in core.
2. **Command/event API on `core/game`** (001): the model now has most actions as methods, so a
   command layer (`MoveStack`, `BuyProduction`, `SellBuilding`, `CastSpell`, ...) can wrap them.
3. Then the wasm bridge for the Three.js frontend.
