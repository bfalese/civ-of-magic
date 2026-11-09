# Handoff 039: the start screen

Date: 2026-11-09
Previous: [038](038-diplomacy.md). Start-here doc: [034](034-start-here.md) (this is the first
half of its step 5).

## State

- #44 (diplomacy, plus `docs/core-review.md`) is merged. This round: branch `start-screen`, PR
  against `master`.
- **`docs/core-review.md`** is new: core behaviours to decide on (so far two diplomacy items:
  tributes not moving the shown mood right away, and the AI always accepting treaties). Add to it
  when a frontend round finds core behaviour that looks wrong.

## What changed

1. **Settings `wizard`, `race`, `banner`** (`HostSettings.WizardChoice/Race/Banner`,
   `core/game/startoptions.go`): the human plays a premade wizard by name, with a race and banner
   (random when empty). Myrror races only for a wizard with the Myrran retort, who then starts on
   Myrror; unknown names are refused.
2. **`civ.startOptions()`** (wasm) / `MakeStartOptions`: the wizards with their books and retort,
   races, banners, difficulties, magic strengths, land sizes, opponent range.
3. **Start screen** (`frontend/web/start.js`): shown when the URL has none of `opponents`,
   `difficulty`, `wizard`, `scenario`; pick a wizard card, name, race, banner swatch, opponents,
   difficulty, magic, land size. The last choices are remembered in localStorage (wrapped in
   try/catch). URL parameters still start at once (tests and bookmarks), now also `wizard`,
   `race`, `banner`, `magic`.
4. A global `[hidden] { display: none !important }`: `#top` (display flex) used to show through
   its `hidden` attribute while loading.
5. `TestHostStartChoice`; the smoke test starts with the start screen (Freya, Klackons, purple).

## Notes

- No wizard portraits (not exported yet: `wizards.lbx`); the cards show books and retort.
- Custom wizards (pick books and retorts) aren't offered; the core takes a `setup.WizardCustom`
  but not from JSON.

## Next

- Rest of 034 step 5: soft fog edges (`mapback.lbx` 0-13, exported and drawn at the edge of the
  explored area).
- Step 6: battle spell animations; `castable` that knows whether a spell has a valid target.
- Maybe: wizard portraits in the export, for the start, diplomacy and magic screens.
