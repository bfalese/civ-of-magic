# Handoff 025: the city screen in the web frontend

Date: 2026-11-07
Previous: [024](024-web-frontend.md). Start-here doc: [020](020-start-here.md).

## State

- Branch `city-screen`, PR against `master`; the user asked to merge it when done.
- PR #31 (CI: only Linux x86-64 on push; macOS, Windows and arm64 by hand) is open.
- The user installed Playwright (Python, with its Chromium) and xdotool: browser tests and
  driving the Ebitengine game under Xvfb are now possible.
- Next, as the user chose: tactical battles in the web frontend.

## What changed

1. **City view** (`core/game/view.go`): `subsistence-farmers`, `can-build` (`{name, unit?, cost}`:
   buildings with prerequisites met plus Trade Goods/Housing in building order, then units by
   name; the Ebitengine build screen's lists) and `sell-values`.
2. **`set-city-production`** now refuses buildings whose prerequisites aren't built
   (`ComputePossibleBuildings`), like the build screen; before it only checked that the building
   is allowed in the city.
3. **City screen** (`frontend/web/main.js`): clicking your city opens it (unless a stack is
   selected: then the click moves the stack there). Food/gold/production/power/research,
   farmers −/+ (down to the subsistence minimum), what's being produced with progress and turns
   left, Buy (cost, disabled without the gold), the production choices with costs, buildings
   with Sell buttons (one sale per turn), the garrison with a Select button. Esc, Close or a
   click outside closes it; it follows refreshes.
4. **`frontend/web/smoke_test.py`**: Playwright test against a running `serve-web`: city screen,
   change production, farmers, sell, select the garrison, move it (tries neighbouring tiles; maps
   are random), end turn, pick research; fails on page errors. Passed on several random maps.
5. Docs: frontend-protocol (city view fields), CLAUDE.md (smoke test).

## Not yet

- Rename city (the command exists), city enchantments, the catchment-area map.
- Buying wasn't exercised in the smoke test (no gold on turn 1); core tests cover it.
