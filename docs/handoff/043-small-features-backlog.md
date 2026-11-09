# Handoff 043: small features to finish before the 3D style

Date: 2026-11-09
Previous: [042](042-ai-spells.md). Start-here doc: [034](034-start-here.md).

The web frontend covers steps 1-6 of 034. Before 034 step 7 (the 3D/retro look), finish these
smaller features. Work through them in the order below, one PR per group, and tick them here (or
move them to a round handoff) as they land. PR #48 (AI spells, `SpellCanWork`) may still be open:
merge it first.

## 1. Map orders the page doesn't send yet (page work; the commands exist)

Without the first one the web game can't expand. All take the selected stack (`stack`: its
location); see `docs/frontend-protocol.md`.

- [ ] **Settle / meld / purify**: `build-action`. A "Build" button (key B, as the desktop) in the
      stack panel when the stack can do it: settlers found an outpost (a `new-outpost` event
      follows; the page already handles it), a magic spirit or guardian spirit melds a node,
      priests purify corruption. The view should say which action applies (add it to the
      `StackView` if it isn't there).
- [ ] **Build road**: `build-road` with `to`. Engineers: a "Road" button, then click the
      destination on the map (like spell targets); show the path while picking.
- [ ] **Planar travel**: `planar-travel`. A "Plane" action on the stack when it stands on a tower
      or Astral Gate, or has Plane Shift.
- [ ] **Patrol**: `patrol`. A button; the stack stops being offered by Next unit.
- [ ] **Rename hero**: `rename-hero` (index 0-5). A ✎ next to the hero's name in the items screen
      (`frontend/web/vault.js`), like the city rename.

Fold in the core bug **`GameModel.PlanarTravel` reads the viewed plane (`model.Plane`)** instead
of the stack's (027 step 7): fix and test it with the planar travel button.

## 2. Save and load (most useful for real games)

A reload loses the game today. The core already serializes a game: `SerializeModel` /
`MakeModelFromSerialized` (`core/game/serialize.go`, players in `core/player/serialize.go`).

- [ ] Core / host: `civ.save()` returning the game as JSON, `civ.load(json)` rebuilding the host
      (the human, the command session, pending offers / trades / items: check what the host keeps
      outside the model). Unit ids aren't saved (027 step 7): reassign them on load, they're only
      stable within a session anyway.
- [ ] Page: save to `localStorage` (try/catch; a few slots, or one autosave per turn) and a file
      download / upload; a "Continue" button on the start screen when a save exists.
- [ ] Test: save, load, the views match (turn, cities, stacks, research, mana); a smoke step.

## 3. Lists

- [ ] **City list**: every city with size, what it builds and turns left, food / gold / production;
      click to open its city screen (desktop `citylistview`).
- [ ] **Army list**: every stack and hero with where it is and its moves; click to select and
      centre the map on it (desktop `armyview`).

## 4. Map overview and tile info

- [ ] **Minimap** in a corner (or a full world map, desktop `cartographer`): explored terrain,
      cities, the camera rectangle; click to move the camera. Both planes.
- [ ] **Tile info** (desktop surveyor): hovering a tile shows terrain, bonus, and for a possible
      city its maximum population and food / production / gold bonuses, or why a city can't go
      there. Needs a core query (the surveyor's numbers are in the Ebitengine code, move them to
      core).
- [ ] **Wizard info** (desktop mirror): your books, retorts, fame, spell skill; known rivals' the
      same as far as you know them. Could live in the Wizards screen.

## 5. Small core fixes (fold into the PR that touches the same code)

- [ ] `TakeEvents` returns every player's events (the AI's too): filter to the human in the host
      (027 step 7).
- [ ] New items can wait in the vault's "new" list for ever; the original makes you place or
      destroy them before going on (decide: force it at end of turn, or keep it lenient).
- [ ] Export wizard portraits (`wizards.lbx`) and item slot pictures (`itemisc.lbx` 19-24) and use
      them on the start, diplomacy, magic and items screens.
- [ ] Offers declined in the Ebitengine screens don't clear `PendingOffers` (harmless; 027).

## Later (bigger; not part of this list)

- The diplomacy decisions in `docs/core-review.md` (tributes and the visible mood, AI answering
  treaties), and AI wizards starting diplomacy themselves.
- A custom wizard builder on the start screen (pick books and retorts; the core takes a
  `setup.WizardCustom`, not yet from JSON).
- Spell of Mastery (refused by `cast-spell` for now: its victory screens).
- Disenchant Area / Call Chaos / summons in `SpellCanWork` (always offered now).
- Then 034 step 7: the 3D/retro look, Ebitengine xdotool smoke tests, the upstream side programs.

## Checks every round

As 034: `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` (CI under
xvfb-run), the `ebiten|/lbx` guard, the wasm build, the gamedata host tests, the exporter tests
with `MOM_DATA`, and the smoke test against serve-web (port 8099 if Open WebUI holds 8080).
Re-export the assets when a round adds sprites.
