# Handoff 023: unit ids, two-step spell trade; AI diplomacy doesn't exist

Date: 2026-11-06
Previous: [022](022-asset-export.md). Start-here doc: [020](020-start-here.md).

## State

- PRs #25–#28 are merged. This round: branch `protocol-gaps`, PR against `master`.
  **Don't merge without explicit approval.**
- Next: `frontend/web` (the Three.js map), see 022.

## What changed

1. **Unit ids** (`core/game/unitids.go`): `GameModel.UnitId(unit)` gives an overland unit a number
   the first time it's asked (ids never reused in a game, not saved); `FindUnitById(player, id)`.
   Dead units are pruned at the end of each turn. The player view lists `id` per unit.
   `disband-unit {unit}`, `set-unit-active {unit, active}` and unit spell targets (`target.unit`)
   take the id; the `stack` field is gone from the two unit commands and a spell target's
   `location` is optional when `unit` is given. The Ebitengine unit buttons use ids too.
   Heroes in `rename-hero` and item slots still use their 0–5 slot (slots don't shift).
2. **Two-step spell trade**: `trade-spells` now stores the wizard's offer
   (`GameModel.PendingTrades`) and emits `trade-offer {player, wizard, give, receive}`;
   `answer-trade {accept}` exchanges (then `learned-spell`) or declines. An offer is dropped at
   the player's next turn start. Same flow as the diplomacy screen ("I will trade you X": Yes /
   Forget It).
3. Docs: frontend-protocol (unit ids convention, commands, events, spell targets).

## Finding: no AI-initiated diplomacy

The open item "diplomacy initiated by other wizards has no events/answers" turned out to have
nothing behind it: besides the first meeting (already the `diplomacy` event) the AI never
proposes treaties, never declares war (`WarWithPlayer` is never called) and always accepts what
the player proposes. Making AI wizards do diplomacy is a game feature to design (when they
propose, what they want, how relations drive it), not a protocol gap.

## Still open (from 020)

- `GameModel.PlanarTravel` reads the viewed plane (`model.Plane`).
- Offers declined in the Ebitengine screens don't clear `PendingOffers` (harmless).
- AI attacks on the human are auto-resolved in `GameHost` (021).
- AI diplomacy (above).
