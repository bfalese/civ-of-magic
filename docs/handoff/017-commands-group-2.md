# Handoff 017: the remaining player actions as commands (magic, stacks, vault, names, diplomacy, items)

Date: 2026-11-06
Previous: [016](016-offers-unit-orders-ui-commands.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#20 are merged into `master`.
- This round: branch `commands-group-2`, PR #21 against `master`. **Don't merge without explicit approval.**
- Wasm bridge stays last (user's request). Hand smoke tests: [`docs/smoke-test.md`](../smoke-test.md).

## What changed (the "group 2" list from the command inventory)

| Area | Core rules | Commands | Ebitengine UI |
|---|---|---|---|
| Magic screen | `Player.AlchemyRate`, `Alchemy`, `SetPowerDistribution` | `alchemy`, `set-power-distribution` | alchemy uses `Player.Alchemy`; sliders still set values while dragging |
| Stacks | `GameModel.OrderStackMove`: only active units go, inactive ones stay as their own stack (ships keep passengers), Earth Gate instant moves, merge back if no path | `set-unit-active`; `move-stack` now uses `OrderStackMove` | the map click/keys order and the unit toggle go through it (toggle via `Game.execute`) |
| Vault | `core/game/vault.go`: `SendVaultItem` (+ `PendingItems`), `MoveItem` (slot compatibility, 20 mana per item teleported to a hero elsewhere), `DestroyItem`, `ClearPendingItem` | `move-item`, `destroy-item` with item slots `{vault}`, `{hero, slot}`, `{new}` | vault screen unchanged (its item "in hand" has no command equivalent); it clears the pending item it places |
| Names | | `rename-city`, `rename-hero` | name prompts unchanged |
| Diplomacy | `core/player/diplomacy.go`: `ProposeTreaty`, `TributeGold(Amount)`, `TributeSpell`, `WillTradeSpells`, `SpellOfferedBy`, `TradeSpells` | `propose-treaty`, `offer-tribute`, `trade-spells` (one step: no "see their offer, then decline") | the diplomacy screen calls the core rules |
| Items | `core/artifact/design.go`: `Design`, `BuildDesign` (designer screen's rules: type, picture range, ≤4 powers, one per stat, ability groups and spellbooks, Enchant Item cost threshold, staff/wand charges 1-4), `EnchantItemCostThreshold` | `cast-spell` with `artifact` for Create Artifact / Enchant Item | designer screen unchanged; uses the core threshold |

`GameModel.ArtifactPowers` is the power table hook (set from LBX by the UI, from gamedata by the
headless game). All vault events in core go through `SendVaultItem` (combat deaths, earthquake,
merchant, Create Artifact, Gift). Protocol doc updated.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass;
  CI guard empty; `TestHeadlessGame` with data passes.
- New tests: power distribution, alchemy, active units + split move, renames, vault items, treaties
  and tributes, item design (`core/artifact`) and Create Artifact through the command.
- `-watch` 3 min (84 years), no errors.
- **Not testable here** (no input automation tool): moving your own stacks with the mouse/keys now
  goes through `OrderStackMove`. Smoke test it first (see smoke-test.md §3).

## Next

- Command inventory groups 1 and 2 are done except UI wiring that has no one-to-one command (vault
  hand, sliders, name prompts). Remaining: **tactical combat** for JSON clients (auto-resolve now;
  later the `core/combat` remote protocol) and **wasm bridge**.
- Small follow-ups: `PlanarTravel` still reads the viewed plane; offers declined from the
  Ebitengine screens don't clear `PendingOffers` (harmless, overwritten by the next offer).
